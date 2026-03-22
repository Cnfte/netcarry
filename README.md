# 网络性能与压力测试工具套件

本仓库集成了两款高性能网络压力测试工具，分别用于不同的测试场景：

1.  **[NetCarry](#netcarry-ultimate-cli)**：专注于多协议网络层（L4）与通用应用层（L7）的极限压力测试。
2.  **[WPBench](#wpbench)**：专注于 WordPress 站点应用层的性能基准分析与可视化测试。

---

<a id="netcarry-ultimate-cli"></a>
## 1. NetCarry Ultimate CLI
NetCarry 是一款专为 Linux 系统设计的高性能、多协议网络压力测试工具，适用于网络架构评估、CDN 负载测试及防火墙性能验证。

### 核心特性
*   **多协议混合**：支持 UDP、TCP、CC 以及域名刷流（BRUSH）。
*   **高性能并发**：基于多线程异步逻辑。
*   **CC 模拟**：内置随机 User-Agent 与 SSL/TLS 握手，模拟真实用户。
*   **全自动化**：支持一键部署脚本。

### 快速安装
```bash
# Linux
curl -sL http://fhdh.cnfte.top/installer.sh | sudo bash

# Windows/Android Termux (直接下载 netcarry.py)
python netcarry.py <目标IP或域名> [参数]
```

### 命令格式
```bash
netcarry <目标IP或域名> [参数]
```

---

<a id="wpbench"></a>
## 2. WPBench
WordPress 性能压测工具。采用 **Go 后端 Agent + PHP 单文件控制面板** 架构，用于精准测试 WordPress 站点在高并发下的响应表现。

### 核心特性
*   **可视化分析**：支持 QPS、P50/P95/P99 延迟、错误率及 HTTP 状态码分类统计。
*   **WP 专项**：支持 `wp-login.php` 登录测试及多路径自定义爬坡（Ramp-up）。
*   **分布式管理**：控制面板支持管理多个远程 Agent 节点。
*   **结果导向**：支持 CSV 数据导出，便于性能报告生成。

### 快速部署
1.  **Agent 安装 (压测机)**:
    ```bash
    sudo wget -O install.sh http://fhdh.cnfte.top/installgo.sh && bash install.sh --url http://fhdh.cnfte.top/wpbench --password {密钥}
    ```
2.  **面板部署 (管理机)**:
    将 `index.php` 放入任意 PHP Web 目录，通过浏览器访问即可配置节点并开始测试。

---

## 选型指南：我该用哪个？

| 场景 | 推荐工具 | 理由 |
| :--- | :--- | :--- |
| **测试网络出口带宽** | **NetCarry** | UDP/TCP 模式可直接打满带宽瓶颈 |
| **网站遭受 CC 攻击防御验证** | **NetCarry** | 模拟高频随机请求与 SSL 握手压力 |
| **WordPress 页面响应慢排查** | **WPBench** | 可监控 P95 延迟，专门优化 PHP/数据库性能 |
| **测试 WordPress 登录接口** | **WPBench** | 内置专门的 POST 接口压测逻辑 |
| **测试 CDN 流量计费/上限** | **NetCarry** | BRUSH 模式可产生持续的大流量 |

---

## 法律免责声明

**重要声明**：本仓库所有工具仅供网络安全从业人员及系统管理员，在**获得明确合法授权**的情况下，对自有或客户受权的服务器/应用程序进行性能测试、安全审计或压力模拟。

禁止将本工具用于任何未经授权的攻击行为。因使用者违反相关法律规定而导致的一切后果，由使用者自行承担，开发者不承担任何法律责任。使用本工具即视为您同意本声明。
