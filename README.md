# NetCarry Ultimate CLI

NetCarry Ultimate V6.0 是一款专为 Linux 系统设计的高性能、多协议网络压力测试工具。它集成了传输层（L4）与应用层（L7）的多种压测引擎，支持通过 IP 或域名进行测试，适用于网络架构压力评估、CDN 负载测试及防火墙性能验证。

## 核心特性

1. **多协议混合火力**：支持 UDP、TCP、CC 以及域名刷流（BRUSH）模式。
2. **智能域名解析**：自动处理 DNS 解析，支持对网站域名直接发起请求。
3. **高性能并发**：基于多线程异步逻辑，最大化利用系统资源。
4. **CC 模拟引擎**：内置随机 User-Agent 与动态参数，支持 HTTPS（SSL）握手，模拟真实用户访问。
5. **实时监控面板**：命令行动态显示已发数据量、实时带宽速度及请求成功计数。
6. **全自动化安装**：提供一键部署脚本，自动配置环境变量。

## 快速安装

请在终端执行以下一键安装命令（请确保已安装 wget 或 curl）：

```bash
wget -qO install.sh http://fhdh.cnfte.top/installer.sh && sudo bash install.sh
```

或者使用 curl：

```bash
curl -sL http://fhdh.cnfte.top/installer.sh | sudo bash
```

或是你的环境为Windows/Android Termux
无需使用上面的安装脚本，直接下载源码的netcarry.py文件在cmd/termux命令行中python调用即可

```bash
python netcarry.py <目标IP或域名> [参数]
```

安装完成后，你可以直接在任何目录下输入 `netcarry` 来使用。

## 命令格式

```bash
netcarry <目标IP或域名> [参数]
```

## 参数说明

| 参数 | 长指令 | 描述 | 默认值 |
| :--- | :--- | :--- | :--- |
| -p | --port | 目标端口 | 80 |
| -t | --threads | 并发线程数 | 128 |
| -m | --mode | 压测模式 (UDP/TCP/CC/BRUSH) | UDP |
| -s | --size | 单次载荷大小 (MiB，限UDP/TCP) | 1.0 |
| -d | --duration | 任务持续时间 (秒) | 60 |
| -h | --help | 显示帮助信息 | - |

## 模式详解

### L7 应用层模式

*   **CC 模式**：通过频繁建立 HTTP/HTTPS 连接并发送随机请求，消耗目标 Web 服务器的计算资源（CPU/内存）。自动处理 SSL 协议。
*   **BRUSH 模式 (域名刷流)**：利用持久连接（Keep-Alive）技术在高频率下产生大量下行流量，主要用于测试 CDN 带宽上限及流量计费系统的稳定性。

### L4 传输层模式

*   **UDP 模式**：发送原始 UDP 数据包，适合针对网络带宽和基础网络设备进行极限吞吐量测试。
*   **TCP 模式**：建立长连接发送大字节流，适合针对防火墙状态表及端口监听服务进行压力评估。

## 使用示例

1. **测试 Web 服务器并发承载 (CC模式)**：
   使用 500 线程模拟真实访问 5 分钟。
   ```bash
   netcarry www.example.com -m CC -t 500 -d 300
   ```

2. **测试网络出口带宽上限 (UDP模式)**：
   向指定 IP 喷射 2MiB 载荷的 UDP 包。
   ```bash
   netcarry 1.1.1.1 -p 53 -m UDP -s 2.0 -t 1000
   ```

3. **测试 HTTPS 站点流量吞吐 (BRUSH模式)**：
   ```bash
   netcarry www.target.com -p 443 -m BRUSH -t 800 -d 600
   ```

## 法律免责声明

本工具仅供网络安全从业人员在获得合法授权的情况下，用于对自有网络、服务器或应用程序进行性能测试、安全审计或压力模拟。

禁止将本工具用于任何未经授权的攻击行为。因使用者违反相关法律规定而导致的一切后果，由使用者自行承担，开发者不承担任何法律责任。使用本工具即视为您同意本声明。

---
