package engine

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

// RunTCPFlooder 启动 TCP 高速流压测工作池
func RunTCPFlooder(ctx context.Context, cfg *Config, stats *Stats) {
	addr := fmt.Sprintf("%s:%d", cfg.ResolvedIP, cfg.Port)

	// 载荷大小适配
	payloadSize := cfg.PayloadSize
	if payloadSize <= 0 {
		payloadSize = 65536 // 默认 64KB
	}
	if cfg.WeakDevice && payloadSize > 16384 {
		payloadSize = 16384 // 弱设备限制单次写缓冲区为 16KB，节省内存
	}

	payload := make([]byte, payloadSize)
	FillRandomBuffer(payload)

	var wg sync.WaitGroup
	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			stats.WorkerStart()
			defer stats.WorkerDone()

			d := &net.Dialer{
				Timeout: cfg.Timeout,
			}

			for {
				select {
				case <-ctx.Done():
					return
				default:
					conn, err := d.DialContext(ctx, "tcp", addr)
					if err != nil {
						stats.AddError()
						// 连接失败时稍作等待，防止弱设备 CPU 空转
						time.Sleep(50 * time.Millisecond)
						continue
					}

					// 启用 TCP NoDelay
					if tcpConn, ok := conn.(*net.TCPConn); ok {
						_ = tcpConn.SetNoDelay(true)
						_ = tcpConn.SetKeepAlive(true)
					}

					// 持续流式发送数据直至连接异常或取消
					active := true
					for active {
						select {
						case <-ctx.Done():
							_ = conn.Close()
							return
						default:
							_ = conn.SetWriteDeadline(time.Now().Add(cfg.Timeout))
							n, err := conn.Write(payload)
							if err != nil {
								stats.AddError()
								active = false
								break
							}

							stats.AddBytes(uint64(n))
							stats.AddRequest()

							if cfg.RateLimit > 0 {
								time.Sleep(time.Duration(1000000/cfg.RateLimit) * time.Microsecond)
							}
						}
					}
					_ = conn.Close()
				}
			}
		}(i)
	}

	wg.Wait()
}
