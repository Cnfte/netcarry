package main

// wpbench - WordPress 性能压测工具 (后端 Agent)
// 用法: go run wpbench.go -password yourkey
// 需要 Go 1.21+

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// ─────────────────────────────────────────
// 数据结构
// ─────────────────────────────────────────

// Task 由前端 POST /api/start 传入
type Task struct {
	BaseURL     string   `json:"base_url"`    // 如 https://example.com
	Paths       []string `json:"paths"`       // 要测试的路径列表
	Concurrency int      `json:"concurrency"` // 并发 goroutine 数
	Duration    int      `json:"duration"`    // 测试时长(秒)
	RampUp      int      `json:"ramp_up"`     // 爬坡时间(秒), 0=立即全并发
	PostLogin   bool     `json:"post_login"`  // 是否测试 wp-login.php POST
	LoginUser   string   `json:"login_user"`
	LoginPass   string   `json:"login_pass"`
}

// Sample 单次请求的原始数据
type Sample struct {
	LatencyMs int64 // 响应耗时(毫秒)
	Status    int   // HTTP 状态码
	Bytes     int64 // 响应体字节数
	IsError   bool  // 网络/超时错误
}

// Snapshot 每秒推送给前端的实时快照
type Snapshot struct {
	Running     bool    `json:"running"`
	Elapsed     int     `json:"elapsed"`     // 已运行秒数
	Duration    int     `json:"duration"`    // 总时长
	Concurrency int     `json:"concurrency"` // 实际并发数
	TotalReq    int64   `json:"total_req"`
	TotalBytes  int64   `json:"total_bytes"`
	QPS         float64 `json:"qps"`        // 当前秒 QPS
	ErrorRate   float64 `json:"error_rate"` // 当前秒错误率(0~1)
	P50         int64   `json:"p50"`
	P95         int64   `json:"p95"`
	P99         int64   `json:"p99"`
	Avg         float64 `json:"avg"`
	Status2xx   int64   `json:"status_2xx"`
	Status3xx   int64   `json:"status_3xx"`
	Status4xx   int64   `json:"status_4xx"`
	Status5xx   int64   `json:"status_5xx"`
}

// ─────────────────────────────────────────
// 全局状态
// ─────────────────────────────────────────

var (
	apiAuth    string
	mu         sync.Mutex
	cancelFunc context.CancelFunc

	// 原子计数器
	atomicTotalReq   int64
	atomicTotalBytes int64
	atomicStatus2xx  int64
	atomicStatus3xx  int64
	atomicStatus4xx  int64
	atomicStatus5xx  int64
	atomicConcur     int64

	// 当前快照(供 /api/status 读取)
	snapshotMu sync.RWMutex
	lastSnap   Snapshot

	// 每秒采样窗口(用于计算延迟分位数)
	windowMu  sync.Mutex
	windowBuf []Sample

	// CSV 历史记录
	csvRows [][]string
)

// ─────────────────────────────────────────
// main
// ─────────────────────────────────────────

func main() {
	pass := flag.String("password", "wpbench2024", "API 鉴权密钥")
	port := flag.Int("port", 36499, "监听端口")
	flag.Parse()
	apiAuth = *pass

	mux := http.NewServeMux()
	mux.HandleFunc("/api/start", cors(authMiddleware(startHandler)))
	mux.HandleFunc("/api/stop", cors(authMiddleware(stopHandler)))
	mux.HandleFunc("/api/status", cors(authMiddleware(statusHandler)))
	mux.HandleFunc("/api/download", cors(authMiddleware(downloadHandler)))
	mux.HandleFunc("/api/ping", cors(pingHandler))

	addr := fmt.Sprintf(":%d", *port)
	fmt.Printf("[wpbench] Agent 已启动, 监听 %s\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// ─────────────────────────────────────────
// HTTP 处理器
// ─────────────────────────────────────────

func pingHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

func startHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	var t Task
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	// 参数边界保护
	if t.Concurrency < 1 {
		t.Concurrency = 10
	}
	if t.Concurrency > 500 {
		t.Concurrency = 500
	}
	if t.Duration < 5 {
		t.Duration = 5
	}
	if t.Duration > 600 {
		t.Duration = 600
	}
	if len(t.Paths) == 0 {
		t.Paths = []string{"/"}
	}
	if t.BaseURL == "" {
		http.Error(w, "base_url required", 400)
		return
	}

	mu.Lock()
	if cancelFunc != nil {
		cancelFunc()
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(t.Duration)*time.Second)
	cancelFunc = cancel
	mu.Unlock()

	// 重置所有计数器
	atomic.StoreInt64(&atomicTotalReq, 0)
	atomic.StoreInt64(&atomicTotalBytes, 0)
	atomic.StoreInt64(&atomicStatus2xx, 0)
	atomic.StoreInt64(&atomicStatus3xx, 0)
	atomic.StoreInt64(&atomicStatus4xx, 0)
	atomic.StoreInt64(&atomicStatus5xx, 0)
	windowMu.Lock()
	windowBuf = windowBuf[:0]
	windowMu.Unlock()
	csvRows = [][]string{{"Timestamp", "QPS", "P50ms", "P95ms", "P99ms", "AvgMs", "ErrorRate", "2xx", "3xx", "4xx", "5xx"}}

	go runBench(ctx, t)

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"started"}`))
}

func stopHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	if cancelFunc != nil {
		cancelFunc()
	}
	mu.Unlock()
	w.Write([]byte(`{"status":"stopped"}`))
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	snapshotMu.RLock()
	snap := lastSnap
	snapshotMu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(snap)
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=wpbench_report.csv")
	cw := csv.NewWriter(bufio.NewWriter(w))
	for _, row := range csvRows {
		cw.Write(row)
	}
	cw.Flush()
}

// ─────────────────────────────────────────
// 压测引擎
// ─────────────────────────────────────────

func runBench(ctx context.Context, t Task) {
	transport := &http.Transport{
		MaxIdleConns:        t.Concurrency + 50,
		MaxIdleConnsPerHost: t.Concurrency + 50,
		DisableCompression:  false,
		ForceAttemptHTTP2:   true,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil // 跟随重定向但计入响应时间
		},
	}

	pathCount := int64(len(t.Paths))
	var reqCounter int64 // 用于轮询路径

	// 爬坡: 按 ramp_up 秒线性增加并发
	rampStep := t.Concurrency
	rampInterval := time.Duration(0)
	if t.RampUp > 0 && t.Concurrency > 1 {
		steps := t.Concurrency
		rampInterval = time.Duration(t.RampUp) * time.Second / time.Duration(steps)
		rampStep = 1
	}

	var wg sync.WaitGroup
	launched := 0

	launchWorker := func(workerCtx context.Context) {
		wg.Add(1)
		atomic.AddInt64(&atomicConcur, 1)
		go func() {
			defer wg.Done()
			defer atomic.AddInt64(&atomicConcur, -1)
			for {
				select {
				case <-workerCtx.Done():
					return
				default:
					// 轮询路径
					idx := atomic.AddInt64(&reqCounter, 1) % pathCount
					path := t.Paths[idx]

					var (
						start  = time.Now()
						status int
						bytes  int64
						isErr  bool
					)

					url := t.BaseURL + path
					var req *http.Request
					var err error

					if t.PostLogin && path == "/wp-login.php" {
						// 模拟登录 POST
						req, err = buildLoginRequest(url, t.LoginUser, t.LoginPass)
					} else {
						req, err = http.NewRequestWithContext(workerCtx, "GET", url, nil)
						if err == nil {
							req.Header.Set("User-Agent", "wpbench/2.0 (+https://github.com/wpbench)")
							req.Header.Set("Accept", "text/html,application/xhtml+xml")
						}
					}

					if err != nil {
						isErr = true
					} else {
						resp, doErr := client.Do(req)
						latency := time.Since(start).Milliseconds()
						if doErr != nil {
							isErr = true
						} else {
							status = resp.StatusCode
							// 读取并丢弃 body，确保连接可复用
							buf := make([]byte, 4096)
							for {
								n, e := resp.Body.Read(buf)
								bytes += int64(n)
								if e != nil {
									break
								}
							}
							resp.Body.Close()

							atomic.AddInt64(&atomicTotalReq, 1)
							atomic.AddInt64(&atomicTotalBytes, bytes)
							switch {
							case status >= 200 && status < 300:
								atomic.AddInt64(&atomicStatus2xx, 1)
							case status >= 300 && status < 400:
								atomic.AddInt64(&atomicStatus3xx, 1)
							case status >= 400 && status < 500:
								atomic.AddInt64(&atomicStatus4xx, 1)
							case status >= 500:
								atomic.AddInt64(&atomicStatus5xx, 1)
							}

							windowMu.Lock()
							windowBuf = append(windowBuf, Sample{
								LatencyMs: latency,
								Status:    status,
								Bytes:     bytes,
								IsError:   false,
							})
							windowMu.Unlock()
						}
					}

					if isErr {
						atomic.AddInt64(&atomicTotalReq, 1)
						windowMu.Lock()
						windowBuf = append(windowBuf, Sample{IsError: true})
						windowMu.Unlock()
					}
				}
			}
		}()
	}

	// 启动 goroutine（含爬坡逻辑）
	go func() {
		for launched < t.Concurrency {
			for i := 0; i < rampStep && launched < t.Concurrency; i++ {
				launchWorker(ctx)
				launched++
			}
			if rampInterval > 0 && launched < t.Concurrency {
				select {
				case <-ctx.Done():
					return
				case <-time.After(rampInterval):
				}
			}
		}
	}()

	// 统计 ticker: 每秒采样一次
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	startTime := time.Now()
	var prevReq int64

	for {
		select {
		case <-ctx.Done():
			// 最终快照
			buildSnapshot(t, startTime, prevReq, true)
			wg.Wait()
			return
		case <-ticker.C:
			prevReq = buildSnapshot(t, startTime, prevReq, false)
		}
	}
}

func buildSnapshot(t Task, startTime time.Time, prevReq int64, final bool) int64 {
	elapsed := int(time.Since(startTime).Seconds())

	totalReq := atomic.LoadInt64(&atomicTotalReq)
	totalBytes := atomic.LoadInt64(&atomicTotalBytes)
	s2xx := atomic.LoadInt64(&atomicStatus2xx)
	s3xx := atomic.LoadInt64(&atomicStatus3xx)
	s4xx := atomic.LoadInt64(&atomicStatus4xx)
	s5xx := atomic.LoadInt64(&atomicStatus5xx)
	concur := int(atomic.LoadInt64(&atomicConcur))

	// 拿走当前窗口
	windowMu.Lock()
	window := make([]Sample, len(windowBuf))
	copy(window, windowBuf)
	windowBuf = windowBuf[:0]
	windowMu.Unlock()

	deltaReq := totalReq - prevReq
	qps := float64(deltaReq)

	var p50, p95, p99 int64
	var avg float64
	var errCount int

	if len(window) > 0 {
		latencies := make([]int64, 0, len(window))
		var sumLat int64
		for _, s := range window {
			if s.IsError {
				errCount++
			} else {
				latencies = append(latencies, s.LatencyMs)
				sumLat += s.LatencyMs
			}
		}
		if len(latencies) > 0 {
			sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
			p50 = percentile(latencies, 50)
			p95 = percentile(latencies, 95)
			p99 = percentile(latencies, 99)
			avg = math.Round(float64(sumLat)/float64(len(latencies))*10) / 10
		}
	}

	errRate := 0.0
	if len(window) > 0 {
		errRate = float64(errCount) / float64(len(window))
	}

	snap := Snapshot{
		Running:     !final,
		Elapsed:     elapsed,
		Duration:    t.Duration,
		Concurrency: concur,
		TotalReq:    totalReq,
		TotalBytes:  totalBytes,
		QPS:         qps,
		ErrorRate:   math.Round(errRate*1000) / 1000,
		P50:         p50,
		P95:         p95,
		P99:         p99,
		Avg:         avg,
		Status2xx:   s2xx,
		Status3xx:   s3xx,
		Status4xx:   s4xx,
		Status5xx:   s5xx,
	}

	snapshotMu.Lock()
	lastSnap = snap
	snapshotMu.Unlock()

	// 追加 CSV
	csvRows = append(csvRows, []string{
		fmt.Sprintf("%d", time.Now().Unix()),
		fmt.Sprintf("%.1f", qps),
		fmt.Sprintf("%d", p50),
		fmt.Sprintf("%d", p95),
		fmt.Sprintf("%d", p99),
		fmt.Sprintf("%.1f", avg),
		fmt.Sprintf("%.3f", errRate),
		fmt.Sprintf("%d", s2xx),
		fmt.Sprintf("%d", s3xx),
		fmt.Sprintf("%d", s4xx),
		fmt.Sprintf("%d", s5xx),
	})

	return totalReq
}

// ─────────────────────────────────────────
// 工具函数
// ─────────────────────────────────────────

func percentile(sorted []int64, p int) int64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(float64(p)/100*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func buildLoginRequest(url, user, pass string) (*http.Request, error) {
	body := fmt.Sprintf("log=%s&pwd=%s&wp-submit=Log+In&redirect_to=%%2Fwp-admin%%2F&testcookie=1", user, pass)
	req, err := http.NewRequest("POST", url,
		&stringReader{s: body, i: 0})
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "wpbench/2.0")
	req.Header.Set("Cookie", "wordpress_test_cookie=WP+Cookie+check")
	return req, nil
}

// 轻量字符串 Reader，避免 strings 包依赖
type stringReader struct {
	s string
	i int
}

func (r *stringReader) Read(p []byte) (n int, err error) {
	if r.i >= len(r.s) {
		return 0, fmt.Errorf("EOF")
	}
	n = copy(p, r.s[r.i:])
	r.i += n
	return
}

func cors(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "X-Auth, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		h(w, r)
	}
}

func authMiddleware(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 同时接受 Header: X-Auth 和 query: ?X-Auth=xxx（供浏览器直接下载用）
		token := r.Header.Get("X-Auth")
		if token == "" {
			token = r.URL.Query().Get("X-Auth")
		}
		if token != apiAuth {
			http.Error(w, `{"error":"auth failed"}`, 403)
			return
		}
		h(w, r)
	}
}
