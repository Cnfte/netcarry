package engine

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

// ANSI 颜色常量
const (
	ColorPurple = "\033[95m"
	ColorCyan   = "\033[96m"
	ColorYellow = "\033[93m"
	ColorRed    = "\033[91m"
	ColorGreen  = "\033[92m"
	ColorBold   = "\033[1m"
	ColorEnd    = "\033[0m"
)

// Stats 收集原子无锁统计数据
type Stats struct {
	TotalBytes    atomic.Uint64
	TotalRequests atomic.Uint64
	Errors        atomic.Uint64
	ActiveWorkers atomic.Int64
	StartTime     time.Time
}

func NewStats() *Stats {
	return &Stats{
		StartTime: time.Now(),
	}
}

func (s *Stats) AddBytes(n uint64) {
	s.TotalBytes.Add(n)
}

func (s *Stats) AddRequest() {
	s.TotalRequests.Add(1)
}

func (s *Stats) AddError() {
	s.Errors.Add(1)
}

func (s *Stats) WorkerStart() {
	s.ActiveWorkers.Add(1)
}

func (s *Stats) WorkerDone() {
	s.ActiveWorkers.Add(-1)
}

// StartMonitor 启动动态终端监控输出
func (s *Stats) StartMonitor(ctx context.Context, cfg *Config) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var lastBytes uint64
	var lastReqs uint64
	var lastTime = time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			elapsed := now.Sub(s.StartTime).Seconds()
			if elapsed <= 0 {
				continue
			}

			currBytes := s.TotalBytes.Load()
			currReqs := s.TotalRequests.Load()
			currErrors := s.Errors.Load()
			active := s.ActiveWorkers.Load()

			interval := now.Sub(lastTime).Seconds()
			if interval <= 0 {
				interval = 0.5
			}

			// 计算瞬时速率
			instantBytes := currBytes - lastBytes
			instantReqs := currReqs - lastReqs
			speedMBs := (float64(instantBytes) / 1048576.0) / interval
			speedMbps := (float64(instantBytes) * 8.0 / 1000000.0) / interval
			qps := float64(instantReqs) / interval

			lastBytes = currBytes
			lastReqs = currReqs
			lastTime = now

			totalMB := float64(currBytes) / 1048576.0
			totalGB := totalMB / 1024.0

			var dataStr string
			if totalGB >= 1.0 {
				dataStr = fmt.Sprintf("%.2f GB", totalGB)
			} else {
				dataStr = fmt.Sprintf("%.2f MB", totalMB)
			}

			var remainStr string
			if cfg.Duration > 0 {
				rem := cfg.Duration.Seconds() - elapsed
				if rem < 0 {
					rem = 0
				}
				remainStr = fmt.Sprintf(" | 倒计时: %ds", int(rem))
			}

			// 动态刷新单行输出
			fmt.Fprintf(os.Stdout,
				"\r%s[RUN]%s %s%s%s | 发送: %s%s%s | 速度: %s%.2f MB/s%s (%.1f Mbps) | 请求: %d (%.0f/s) | 活跃: %d | 错误: %d%s\033[K",
				ColorYellow, ColorEnd,
				ColorBold, cfg.Mode, ColorEnd,
				ColorCyan, dataStr, ColorEnd,
				ColorGreen, speedMBs, ColorEnd, speedMbps,
				currReqs, qps,
				active,
				currErrors,
				remainStr,
			)
		}
	}
}

// PrintSummary 打印压测结束汇总表格
func (s *Stats) PrintSummary(cfg *Config) {
	elapsed := time.Since(s.StartTime).Seconds()
	if elapsed <= 0 {
		elapsed = 0.001
	}

	totalBytes := s.TotalBytes.Load()
	totalReqs := s.TotalRequests.Load()
	totalErrors := s.Errors.Load()

	totalMB := float64(totalBytes) / 1048576.0
	avgMBs := totalMB / elapsed
	avgMbps := (float64(totalBytes) * 8.0 / 1000000.0) / elapsed
	avgQPS := float64(totalReqs) / elapsed

	var errRate float64
	if totalReqs+totalErrors > 0 {
		errRate = (float64(totalErrors) / float64(totalReqs+totalErrors)) * 100.0
	}

	fmt.Println()
	fmt.Printf("\n%s%s====================== 压测结果汇总 ======================%s\n", ColorCyan, ColorBold, ColorEnd)
	fmt.Printf(" 目标地址     : %s%s (%s:%d)%s\n", ColorPurple, cfg.TargetHost, cfg.ResolvedIP, cfg.Port, ColorEnd)
	fmt.Printf(" 运行模式     : %s%s%s (弱设备优化: %v)\n", ColorPurple, cfg.Mode, ColorEnd, cfg.WeakDevice)
	fmt.Printf(" 运行耗时     : %.2f 秒\n", elapsed)
	if totalMB >= 1024.0 {
		fmt.Printf(" 发送总流量   : %s%.2f GB%s (%.2f MB)\n", ColorGreen, totalMB/1024.0, ColorEnd, totalMB)
	} else {
		fmt.Printf(" 发送总流量   : %s%.2f MB%s\n", ColorGreen, totalMB, ColorEnd)
	}
	fmt.Printf(" 平均吞吐率   : %s%.2f MB/s%s (%.2f Mbps)\n", ColorGreen, avgMBs, ColorEnd, avgMbps)
	fmt.Printf(" 请求/包总数  : %d\n", totalReqs)
	fmt.Printf(" 平均每秒速率 : %s%.1f QPS/PPS%s\n", ColorGreen, avgQPS, ColorEnd)
	fmt.Printf(" 异常/错误数  : %d (错误率: %.2f%%)\n", totalErrors, errRate)
	fmt.Printf("%s==========================================================%s\n\n", ColorCyan, ColorEnd)
}
