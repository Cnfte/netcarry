package engine

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Config 包含了压测引擎的所有运行参数
type Config struct {
	Target       string        // 目标原始输入 (IP, 域名, 或带协议的 URL)
	TargetHost   string        // 解析后的主机名/域名 (用于 HTTP Host 和 TLS SNI)
	ResolvedIP   string        // DNS 解析后的目标 IP 地址
	Port         int           // 目标端口
	Concurrency  int           // 并发 Worker 数 (Goroutines)
	Mode         string        // 模式: UDP, TCP, CC, BRUSH
	Duration     time.Duration // 压测持续时间 (0 为无限)
	PayloadSize  int           // 单包载荷字节数
	KeepAlive    int           // CC/BRUSH 模式下单连接复用请求数
	RateLimit    int           // 速率限制 (QPS/PPS, 0 为无限制)
	Timeout      time.Duration // 网络连接/读写超时时间
	WeakDevice   bool          // 弱设备模式 (低内存占用、CPU 节流保护、MTU 友好)
	UseSSL       bool          // 是否启用 TLS/SSL
	Path         string        // 自定义 HTTP 路径 (默认为空，由模式动态随机生成)
	Method       string        // HTTP 请求方法 (GET, POST 等)
}

// NormalizeAndValidate 规范化配置并执行参数检验与 DNS 预解析
func (c *Config) NormalizeAndValidate() error {
	c.Mode = strings.ToUpper(strings.TrimSpace(c.Mode))
	if c.Mode == "" {
		c.Mode = "UDP"
	}
	switch c.Mode {
	case "UDP", "TCP", "CC", "BRUSH":
	default:
		return fmt.Errorf("不支持的模式: %s (仅支持 UDP, TCP, CC, BRUSH)", c.Mode)
	}

	// 处理 Target 可能带有 http:// 或 https:// 前缀的情况
	rawTarget := strings.TrimSpace(c.Target)
	if strings.HasPrefix(rawTarget, "http://") || strings.HasPrefix(rawTarget, "https://") {
		u, err := url.Parse(rawTarget)
		if err != nil {
			return fmt.Errorf("无效的目标 URL: %w", err)
		}
		c.TargetHost = u.Hostname()
		if u.Scheme == "https" {
			c.UseSSL = true
		}
		if u.Port() != "" {
			p, err := strconv.Atoi(u.Port())
			if err == nil {
				c.Port = p
			}
		} else {
			if c.UseSSL {
				c.Port = 443
			} else {
				c.Port = 80
			}
		}
		if u.Path != "" && u.Path != "/" {
			c.Path = u.Path
		}
	} else {
		// 检查是否包含端口 target:port
		if strings.Contains(rawTarget, ":") {
			host, portStr, err := net.SplitHostPort(rawTarget)
			if err == nil {
				c.TargetHost = host
				p, err := strconv.Atoi(portStr)
				if err == nil {
					c.Port = p
				}
			} else {
				c.TargetHost = rawTarget
			}
		} else {
			c.TargetHost = rawTarget
		}
	}

	if c.TargetHost == "" {
		return errors.New("目标地址不能为空")
	}

	// 端口校准
	if c.Port <= 0 || c.Port > 65535 {
		if c.Mode == "CC" || c.Mode == "BRUSH" {
			if c.UseSSL {
				c.Port = 443
			} else {
				c.Port = 80
			}
		} else {
			c.Port = 80
		}
	}
	if c.Port == 443 {
		c.UseSSL = true
	}

	// 超时校准
	if c.Timeout <= 0 {
		c.Timeout = 3 * time.Second
	}

	// KeepAlive 校准
	if c.KeepAlive <= 0 {
		c.KeepAlive = 30
	}

	// HTTP Method
	if c.Method == "" {
		c.Method = "GET"
	} else {
		c.Method = strings.ToUpper(c.Method)
	}

	// DNS 解析
	ips, err := net.LookupIP(c.TargetHost)
	if err != nil || len(ips) == 0 {
		return fmt.Errorf("DNS 解析失败无法访问主机 %s: %w", c.TargetHost, err)
	}
	// 优先挑选 IPv4 地址
	for _, ip := range ips {
		if ip.To4() != nil {
			c.ResolvedIP = ip.String()
			break
		}
	}
	if c.ResolvedIP == "" {
		c.ResolvedIP = ips[0].String()
	}

	// 弱设备自适应调优
	numCPU := runtime.NumCPU()
	if c.WeakDevice || numCPU <= 1 {
		// 弱设备模式：限制过大并发防止 FD 耗尽或内冲击
		if c.Concurrency > 64 {
			c.Concurrency = 64
		} else if c.Concurrency <= 0 {
			c.Concurrency = 32
		}
		// UDP 载荷限制在单 MTU 范围内 (1350 字节)，避免 IP 分片导致路由器/弱网卡 CPU 耗尽
		if c.Mode == "UDP" && (c.PayloadSize <= 0 || c.PayloadSize > 1400) {
			c.PayloadSize = 1350
		}
	} else {
		if c.Concurrency <= 0 {
			c.Concurrency = 128
		}
		if c.PayloadSize <= 0 {
			if c.Mode == "UDP" {
				c.PayloadSize = 1400 // 默认 MTU 友好尺寸
			} else if c.Mode == "TCP" {
				c.PayloadSize = 65536 // 默认 64KB 流块
			}
		}
	}

	return nil
}
