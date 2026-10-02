# NetCarry Ultimate CLI - 高性能多协议网络压力测试套件

NetCarry 是一款专为网络工程与安全基准测试设计的高性能、低开销多协议网络压力测试工具（**已完成 Go 原生重构 + 弱设备低功耗优化**）。

在最新版本中，系统全面升级为 **Golang 原生高并发异步架构**，同时保留了高度优化的 **Python 3 `asyncio` 异步引擎** 作为降级兼容方案。

```
       _  ____________________   ___  _____  __
      / |/ / __/_  __/ ___/ _ | / _ \/ _ \ \/ /
     /    / _/  / / / /__/ __ |/ , _/ , _/\  / 
    /_/|_/___/ /_/  \___/_/ |_/_/|_/_/|_| /_/  
                                               
     >> NEON STRESS ULTIMATE V7.0 - GOLANG ASYNC HYBRID <<
```

### 核心特性与架构升级

*   **Go 原生异步调度 (epoll / IOCP / kqueue)**：利用 Go 语言协程（Goroutine）轻量化特性（初始栈仅 ~2KB），单机轻松并发上万协程，彻底免除 Python GIL 全局锁瓶颈。
*   **零外部依赖静态二进制**：`CGO_ENABLED=0` 纯静态编译，无任何 glibc 或运行时依赖，极速部署于 Alpine、BusyBox、OpenWrt 等极简环境。
*   **弱设备专项防护 (`--weak`)**：
    *   **超低内存 footprint**：基于 `sync.Pool` 预分配缓冲池与载荷模板，杜绝 GC 内存抖动，弱机常驻内存 **< 10MB**。
    *   **MTU 友好防分片**：UDP 载荷限制在单 MTU（1350 字节）内，避免 IP 分片导致路由器/低配网卡 CPU 耗尽。
    *   **自适应防死锁退避**：当内核套接字缓冲区满或并发文件描述符达到系统上限时平滑微休眠避让，杜绝 CPU 盲跑和进程被 OOM Killer 杀掉。
*   **多架构全平台覆盖**：提供一键多架构交叉编译脚本，原生支持：
    *   `linux_amd64` (常规云服务器/VPS)
    *   `linux_arm64` (AWS Graviton, 树莓派 4/5, 苹果 M 系列 Linux)
    *   `linux_armv7` (老旧嵌入式设备, 树莓派 2/3)
    *   `linux_mipsle` (OpenWrt / 常见家用与工业路由器)
    *   `windows_amd64` (Windows 本地环境)
*   **全协议覆盖**：
    *   `UDP`：极速已连接套接字（Connected UDP），批量计数合流，直击网络带宽上限。
    *   `TCP`：高频 TCP 流持续喷射，测试连接数与服务吞吐。
    *   `CC`：应用层真实 HTTP/HTTPS 模拟，内置会话缓存（TLS Session Resumption）、Keep-Alive 请求管道复用与动态绕过缓存随机参数。
    *   `BRUSH`：域名刷流模式，内置 CDN 击穿头部与 Range 切片，消耗边缘带宽与验证账单保护机制。
*   **实时终端仪表盘**：动态刷新单行彩色控制台，实时输出 MB/s、Gbps、QPS/PPS、倒计时及成功/错误统计，中断时输出完整性能汇总报告。

---

### 性能基准对比 (1 vCPU / 512MB RAM 弱机环境)

| 指标 / 维度 | 旧版 Python 多线程 | 新版 Python asyncio | **新版 Go 原生引擎** |
| :--- | :--- | :--- | :--- |
| **单核 UDP 发包吞吐 (PPS)** | ~18,000 PPS | ~80,000 PPS | **350,000+ PPS (提升 19x)** |
| **CC 模式 QPS (Keep-Alive)** | ~450 QPS | ~1,800 QPS | **8,000+ QPS (提升 17x)** |
| **内存占用 (RAM Usage)** | 120MB - 350MB+ | 30MB - 60MB | **< 10MB (极致轻量)** |
| **弱机防崩能力 (防 OOM/锁死)** | 容易触发 OOM/僵死 | 良好 | **卓越 (内置退避与缓冲池)** |
| **环境依赖** | Python 3 + pip | Python 3 | **零依赖 (单个可执行文件)** |

---

### 快速安装与部署

#### 方法 1：一键自动化安装 (Linux / 路由器 / 云服务器)
安装脚本会自动检测 CPU 架构（x86_64 / arm64 / armv7 / mipsle），优先使用原生 Go 引擎，并自动配置 `ulimit` 内核句柄：
```bash
curl -sL http://fhdh.cnfte.top/installer.sh | sudo bash
```

#### 方法 2：源码编译 (推荐)
如果你已经安装了 Go (1.20+)：
```bash
git clone <repo_url> netcarry
cd netcarry
# 编译本地可执行文件
go build -ldflags="-s -w" -o netcarry main.go
sudo cp netcarry /usr/local/bin/
```

#### 方法 3：多架构一键交叉编译
```bash
# Linux / macOS
chmod +x build.sh && ./build.sh

# Windows
build.bat
```
编译产物将存放在 `bin/` 目录下：
*   `bin/netcarry_linux_amd64`
*   `bin/netcarry_linux_arm64`
*   `bin/netcarry_linux_armv7`
*   `bin/netcarry_linux_mipsle`
*   `bin/netcarry_windows_amd64.exe`

#### 方法 4：直接运行 Python 异步引擎 (Termux / Windows / 临时免安装)
```bash
python netcarry.py <目标> [参数]
```

---

### 命令参数与使用示例

```text
用法: netcarry [选项] <目标IP或域名>

选项参数:
  -m <模式>         压测模式: UDP | TCP | CC | BRUSH (默认: UDP)
  -p <端口>         目标端口 (默认: 80, HTTPS 自动选 443)
  -t, -c <并发数>   并发协程数 (Goroutines, 默认: 128)
  -d <秒数>         压测持续时间 (秒, 默认: 60, 0 为无限)
  -s <MiB>          单包/单流载荷大小 MiB (默认: 1.0)
  -k <请求数>       CC/BRUSH 单连接 Keep-Alive 请求复用数 (默认: 30)
  --weak            开启弱设备/低功耗防护模式 (适合路由器/老旧VPS/树莓派)
  --rate <速率>     限制最大 QPS/PPS 速率 (默认 0 为满载)
  --ssl             强制开启 TLS/SSL 加密
  --path <路径>     指定 HTTP 请求路径
  --timeout <秒>    套接字超时时间 (默认: 3 秒)
  -h, --help        显示帮助信息
```

#### 常见使用场景

1. **测试出口带宽极限 (UDP 洪水发包)**:
   ```bash
   netcarry 1.1.1.1 -m UDP -p 53 -t 256 -d 60
   ```

2. **弱设备 / OpenWrt 路由器低功耗防护测试**:
   ```bash
   netcarry 1.1.1.1 -m UDP -p 53 --weak -d 30
   ```

3. **CC 模拟真实高频访问 (带 SSL 握手与 Keep-Alive 管道复用)**:
   ```bash
   netcarry example.com -m CC -p 443 --ssl -t 200 -d 120
   ```

4. **CDN 计费上限与缓存穿透测试 (BRUSH 刷流模式)**:
   ```bash
   netcarry cdn.example.com -m BRUSH -p 443 -d 60
   ```

---

## 模式选型指南

| 场景 | 推荐模式 | 理由 |
| :--- | :--- | :--- |
| **测试网络出口带宽** | **UDP 模式** | Connected UDP 快速压测，直接打满带宽瓶颈 |
| **测试端口服务连接能力** | **TCP 模式** | 流式 TCP 高并发写入，测试端口服务承载与连接池 |
| **网站遭受 CC 攻击防御验证** | **CC 模式** | 模拟高频真实随机请求、随机 User-Agent 与 SSL 握手压力 |
| **低配路由器/弱 VPS 压测** | **任意模式 + `--weak`** | 内存 < 10MB，MTU 友好防分片，自适应避让防死锁 |
| **测试 CDN 流量计费/上限** | **BRUSH 模式** | 内置 CDN 击穿头部与 Range 切片，消耗边缘带宽 |

---

## 法律免责声明

**重要声明**：本仓库所有工具仅供网络安全从业人员及系统管理员，在**获得明确合法授权**的情况下，对自有或客户受权的服务器/应用程序进行性能测试、安全审计或压力模拟。

禁止将本工具用于任何未经授权的攻击行为。因使用者违反相关法律规定而导致的一切后果，由使用者自行承担，开发者不承担任何法律责任。使用本工具即视为您同意本声明。
