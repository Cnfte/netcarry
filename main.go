package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"netcarry/pkg/engine"
)

func main() {
	var (
		port        int
		concurrency int
		mode        string
		size        float64
		duration    int
		keepAlive   int
		rateLimit   int
		timeoutSec  int
		weakDevice  bool
		useSSL      bool
		customPath  string
		method      string
		showHelp    bool
	)

	flag.IntVar(&port, "p", 80, "目标端口 (默认: 80)")
	flag.IntVar(&concurrency, "t", 128, "并发协程数 / 并发连接数 (默认: 128)")
	flag.IntVar(&concurrency, "c", 128, "并发协程数 (同 -t)")
	flag.StringVar(&mode, "m", "UDP", "压测模式: UDP, TCP, CC, BRUSH (默认: UDP)")
	flag.Float64Var(&size, "s", 1.0, "单次数据载荷大小 MiB (针对 UDP/TCP, 默认: 1.0)")
	flag.IntVar(&duration, "d", 60, "压测持续时间 (秒, 0 为持续运行直至按 Ctrl+C)")
	flag.IntVar(&keepAlive, "k", 30, "CC/BRUSH 单连接复用请求次数 (Keep-Alive, 默认: 30)")
	flag.IntVar(&rateLimit, "rate", 0, "速率限制 (QPS/PPS 限制, 0 为无限制)")
	flag.IntVar(&timeoutSec, "timeout", 3, "网络超时时间 (秒, 默认: 3)")
	flag.BoolVar(&weakDevice, "weak", false, "弱设备/低功耗优化模式 (限制内存峰值与发包分片，防崩溃)")
	flag.BoolVar(&useSSL, "ssl", false, "强制启用 HTTPS/TLS 握手")
	flag.StringVar(&customPath, "path", "", "自定义请求路径 (如 /api/test)")
	flag.StringVar(&method, "method", "GET", "HTTP 方法 (针对 CC 模式)")
	flag.BoolVar(&showHelp, "help", false, "显示帮助信息")
	flag.BoolVar(&showHelp, "h", false, "显示帮助信息")

	flag.Usage = func() {
		fmt.Printf(engine.Banner, engine.ColorPurple, engine.ColorBold, engine.ColorCyan, engine.ColorEnd)
		fmt.Println("用法: netcarry [选项] <目标IP或域名>")
		fmt.Println()
		fmt.Println("选项参数:")
		fmt.Println("  -m <模式>         压测模式: UDP | TCP | CC | BRUSH (默认: UDP)")
		fmt.Println("  -p <端口>         目标端口 (默认: 80, HTTPS 自动选 443)")
		fmt.Println("  -t, -c <并发数>   并发协程数 (Goroutines, 默认: 128)")
		fmt.Println("  -d <秒数>         压测持续时间 (秒, 默认: 60, 0 为无限)")
		fmt.Println("  -s <MiB>          单包/单流载荷大小 MiB (默认: 1.0)")
		fmt.Println("  -k <请求数>       CC/BRUSH 单连接 Keep-Alive 请求复用数 (默认: 30)")
		fmt.Println("  --weak            开启弱设备/低功耗防护模式 (适合路由器/老旧VPS/树莓派)")
		fmt.Println("  --rate <速率>     限制最大 QPS/PPS 速率 (默认 0 为满载)")
		fmt.Println("  --ssl             强制开启 TLS/SSL 加密")
		fmt.Println("  --path <路径>     指定 HTTP 请求路径")
		fmt.Println("  --timeout <秒>    套接字超时时间 (默认: 3 秒)")
		fmt.Println("  -h, --help        显示此帮助信息")
		fmt.Println()
		fmt.Println("模式指南:")
		fmt.Println("  UDP    : 极速无连接 UDP 洪水发包，直击网络带宽上限")
		fmt.Println("  TCP    : TCP 数据流密集写入，考验连接建立与端口吞吐能力")
		fmt.Println("  CC     : 模拟高频真实浏览器 HTTP/HTTPS 请求，穿透代理并施压 Web 应用")
		fmt.Println("  BRUSH  : 域名刷流模式，内置 CDN 击穿头部与 Range 切片，消耗边缘带宽")
		fmt.Println()
		fmt.Println("使用示例:")
		fmt.Println("  netcarry 1.1.1.1 -m UDP -p 53 -t 256")
		fmt.Println("  netcarry example.com -m CC -p 443 --ssl -t 300 -d 120")
		fmt.Println("  netcarry cdn.example.com -m BRUSH -p 443 -d 60 --weak")
		fmt.Println()
	}

	flag.Parse()

	if showHelp || len(flag.Args()) < 1 {
		flag.Usage()
		os.Exit(0)
	}

	target := flag.Arg(0)

	cfg := &engine.Config{
		Target:      target,
		Port:        port,
		Concurrency: concurrency,
		Mode:        mode,
		Duration:    time.Duration(duration) * time.Second,
		PayloadSize: int(size * 1024 * 1024),
		KeepAlive:   keepAlive,
		RateLimit:   rateLimit,
		Timeout:     time.Duration(timeoutSec) * time.Second,
		WeakDevice:  weakDevice,
		UseSSL:      useSSL,
		Path:        customPath,
		Method:      method,
	}

	eng := engine.NewEngine(cfg)
	if err := eng.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s[!] 错误: %v%s\n", engine.ColorRed, err, engine.ColorEnd)
		os.Exit(1)
	}
}
