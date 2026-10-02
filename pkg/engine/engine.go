package engine

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const Banner = `
%s%s
   _  ____________________   ___  _____  __
  / |/ / __/_  __/ ___/ _ | / _ \/ _ \ \/ /
 /    / _/  / / / /__/ __ |/ , _/ , _/\  / 
/_/|_/___/ /_/  \___/_/ |_/_/|_/_/|_| /_/  
                                           
%s >> NEON STRESS ULTIMATE V7.0 - GOLANG ASYNC HYBRID <<%s
`

// Engine 是压测的主执行引擎
type Engine struct {
	cfg   *Config
	stats *Stats
}

func NewEngine(cfg *Config) *Engine {
	return &Engine{
		cfg:   cfg,
		stats: NewStats(),
	}
}

// Run 启动引擎并阻塞直至完成或被信号中断
func (e *Engine) Run() error {
	if err := e.cfg.NormalizeAndValidate(); err != nil {
		return err
	}

	// 打印炫酷 Banner
	fmt.Printf(Banner, ColorPurple, ColorBold, ColorCyan, ColorEnd)
	fmt.Printf("%s目标地址:%s %s%s%s (%s)\n", ColorBold, ColorEnd, ColorPurple, e.cfg.TargetHost, ColorEnd, e.cfg.ResolvedIP)
	fmt.Printf("%s目标端口:%s %s%d%s | %s并发协程:%s %s%d%s | %s运行模式:%s %s%s%s\n",
		ColorBold, ColorEnd, ColorPurple, e.cfg.Port, ColorEnd,
		ColorBold, ColorEnd, ColorPurple, e.cfg.Concurrency, ColorEnd,
		ColorBold, ColorEnd, ColorPurple, e.cfg.Mode, ColorEnd,
	)
	if e.cfg.WeakDevice {
		fmt.Printf("%s设备特性:%s %s低功耗/弱设备优化模式已激活 (低内存/抗丢包/节流防锁死)%s\n",
			ColorBold, ColorEnd, ColorYellow, ColorEnd)
	}
	fmt.Printf("%s----------------------------------------------------------%s\n", ColorCyan, ColorEnd)

	// 创建带取消的上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 持续时间定时器
	if e.cfg.Duration > 0 {
		time.AfterFunc(e.cfg.Duration, func() {
			cancel()
		})
	}

	// 监听中断信号 (Ctrl+C / SIGINT / SIGTERM)
	sigChan := make(chan os.Signal, 2)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Fprintf(os.Stderr, "\n%s[!] 收到用户中断信号，正在平滑退出并统计数据...%s\n", ColorRed, ColorEnd)
		cancel()
	}()

	// 启动终端实时指标监控
	go e.stats.StartMonitor(ctx, e.cfg)

	// 根据模式运行对应的并发引擎
	switch e.cfg.Mode {
	case "UDP":
		RunUDPFlooder(ctx, e.cfg, e.stats)
	case "TCP":
		RunTCPFlooder(ctx, e.cfg, e.stats)
	case "CC", "BRUSH":
		RunHTTPFlooder(ctx, e.cfg, e.stats)
	}

	// 压测结束，打印统计汇总
	e.stats.PrintSummary(e.cfg)
	return nil
}
