package engine

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"sync"
	"time"
)

// RunUDPFlooder 启动 UDP 高速压测工作池
func RunUDPFlooder(ctx context.Context, cfg *Config, stats *Stats) {
	raddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", cfg.ResolvedIP, cfg.Port))
	if err != nil {
		stats.AddError()
		return
	}

	// 准备载荷数据
	payloadSize := cfg.PayloadSize
	if payloadSize <= 0 {
		payloadSize = 1350 // MTU 友好尺寸，防止 IP 分片
	}
	if payloadSize > 65507 {
		payloadSize = 65507
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

			// 每个 Worker 独立建立已连接的 UDP 套接字，极大降低内核每次发包查找路由的开销
			conn, err := net.DialUDP("udp", nil, raddr)
			if err != nil {
				stats.AddError()
				return
			}
			defer conn.Close()

			// 弱设备或常规设备的自适应发包
			batchSize := 16
			if cfg.WeakDevice {
				batchSize = 4
			}

			localCount := uint64(0)
			localBytes := uint64(0)

			for {
				select {
				case <-ctx.Done():
					if localCount > 0 {
						stats.TotalRequests.Add(localCount)
						stats.AddBytes(localBytes)
					}
					return
				default:
					n, err := conn.Write(payload)
					if err != nil {
						stats.AddError()
						if cfg.WeakDevice {
							// 弱设备在发包缓冲区满时微睡眠避让，防止内核锁死
							time.Sleep(100 * time.Microsecond)
						} else {
							runtime.Gosched()
						}
						continue
					}

					localBytes += uint64(n)
					localCount++

					// 批量合并更新原子计数器，降低多核心总线竞争
					if localCount >= uint64(batchSize) {
						stats.TotalRequests.Add(localCount)
						stats.AddBytes(localBytes)
						localCount = 0
						localBytes = 0
					}

					// 速率限制检测
					if cfg.RateLimit > 0 {
						// 简单的自适应休眠
						time.Sleep(time.Duration(1000000/cfg.RateLimit) * time.Microsecond)
					}
				}
			}
		}(i)
	}

	wg.Wait()
}
