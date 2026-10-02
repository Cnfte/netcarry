package engine

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"time"
)

// RunHTTPFlooder 启动 CC 或 BRUSH 模式的高性能应用层压测
func RunHTTPFlooder(ctx context.Context, cfg *Config, stats *Stats) {
	addr := fmt.Sprintf("%s:%d", cfg.ResolvedIP, cfg.Port)
	isBrush := (cfg.Mode == "BRUSH")

	// TLS 配置优化 (开启 SNI、跳过自签名证书验证、启用会话缓存降低握手 CPU 消耗)
	var tlsConfig *tls.Config
	if cfg.UseSSL {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         cfg.TargetHost,
			ClientSessionCache: tls.NewLRUClientSessionCache(2048),
			MinVersion:         tls.VersionTLS12,
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			stats.WorkerStart()
			defer stats.WorkerDone()

			fr := NewFastRand()
			readBufPtr := AcquireBuffer4k()
			defer ReleaseBuffer4k(readBufPtr)
			readBuf := *readBufPtr

			dialer := &net.Dialer{
				Timeout: cfg.Timeout,
			}

			for {
				select {
				case <-ctx.Done():
					return
				default:
					rawConn, err := dialer.DialContext(ctx, "tcp", addr)
					if err != nil {
						stats.AddError()
						time.Sleep(50 * time.Millisecond)
						continue
					}

					var conn net.Conn = rawConn
					if cfg.UseSSL {
						tlsConn := tls.Client(rawConn, tlsConfig)
						_ = rawConn.SetDeadline(time.Now().Add(cfg.Timeout))
						if err := tlsConn.HandshakeContext(ctx); err != nil {
							stats.AddError()
							_ = rawConn.Close()
							time.Sleep(30 * time.Millisecond)
							continue
						}
						conn = tlsConn
					}

					// 单个连接内的 Keep-Alive 复用批次
					keepAliveCount := cfg.KeepAlive
					for k := 0; k < keepAliveCount; k++ {
						select {
						case <-ctx.Done():
							_ = conn.Close()
							return
						default:
							reqData := fr.FormatHTTPRequest(cfg.Method, cfg.TargetHost, cfg.Path, isBrush)
							_ = conn.SetWriteDeadline(time.Now().Add(cfg.Timeout))
							n, err := conn.Write(reqData)
							if err != nil {
								stats.AddError()
								goto ConnBroken
							}

							stats.AddBytes(uint64(n))
							stats.AddRequest()

							// 读取部分响应数据以排空 TCP 接收窗口，避免服务端缓冲区堵塞
							_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
							_, _ = conn.Read(readBuf)

							if cfg.RateLimit > 0 {
								time.Sleep(time.Duration(1000000/cfg.RateLimit) * time.Microsecond)
							}
						}
					}

				ConnBroken:
					_ = conn.Close()
				}
			}
		}(i)
	}

	wg.Wait()
}
