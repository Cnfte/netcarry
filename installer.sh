#!/bin/bash

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
YELLOW='\033[0;33m'
NC='\033[0m'

echo -e "${PURPLE}================================================${NC}"
echo -e "${PURPLE}   正在安装 NetCarry Ultimate 高性能压测套件...   ${NC}"
echo -e "${PURPLE}================================================${NC}"

# 1. 权限检查
if [ "$EUID" -ne 0 ]; then 
  echo -e "${RED}错误: 请使用 sudo 运行此安装脚本。${NC}"
  exit 1
fi

# 2. 识别系统架构
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64)
    GO_ARCH="amd64"
    ;;
  aarch64|arm64)
    GO_ARCH="arm64"
    ;;
  armv7l|armv6l|arm)
    GO_ARCH="armv7"
    ;;
  mips|mipsle)
    GO_ARCH="mipsle"
    ;;
  *)
    GO_ARCH="amd64"
    ;;
esac

echo -e "[1/4] 硬件平台检测: ${CYAN}${ARCH} (对应引擎: linux_${GO_ARCH})${NC}"

# 3. 部署路径
TARGET_BIN="/usr/local/bin/netcarry"
INSTALLED_MODE=""

# 优先级 1: 本地源码编译 (如果系统安装了 Go)
if command -v go &> /dev/null && [ -f "main.go" ]; then
    echo -e "[2/4] 检测到本地 Go 编译器，正在静态编译原生极致性能引擎..."
    CGO_ENABLED=0 go build -ldflags="-s -w" -o "$TARGET_BIN" main.go
    if [ $? -eq 0 ]; then
        INSTALLED_MODE="Go Native Compiled"
    fi
fi

# 优先级 2: 本地已有的预编译二进制 (bin/ 目录)
if [ -z "$INSTALLED_MODE" ] && [ -f "bin/netcarry_linux_${GO_ARCH}" ]; then
    echo -e "[2/4] 使用本地预编译二进制文件 bin/netcarry_linux_${GO_ARCH}..."
    cp "bin/netcarry_linux_${GO_ARCH}" "$TARGET_BIN"
    INSTALLED_MODE="Go Precompiled (Local)"
fi

# 优先级 3: 远端下载预编译静态二进制
if [ -z "$INSTALLED_MODE" ]; then
    echo -e "[2/4] 正在从镜像源下载静态 Go 引擎..."
    DOWNLOAD_URL="http://fhdh.cnfte.top/bin/netcarry_linux_${GO_ARCH}"
    if command -v curl &> /dev/null; then
        curl -sSfL "$DOWNLOAD_URL" -o "$TARGET_BIN" 2>/dev/null
    elif command -v wget &> /dev/null; then
        wget -q -O "$TARGET_BIN" "$DOWNLOAD_URL" 2>/dev/null
    fi

    if [ -s "$TARGET_BIN" ]; then
        INSTALLED_MODE="Go Static Binary (Downloaded)"
    fi
fi

# 优先级 4: 降级回退到异步 Python 引擎
if [ -z "$INSTALLED_MODE" ]; then
    echo -e "${YELLOW}[!] 未能获取 Go 原生二进制，降级安装 Python3 异步引擎...${NC}"
    if ! command -v python3 &> /dev/null; then
        if command -v apt-get &> /dev/null; then
            apt-get update && apt-get install -y python3
        elif command -v yum &> /dev/null; then
            yum install -y python3
        elif command -v apk &> /dev/null; then
            apk add python3
        fi
    fi

    PY_ENGINE_PATH="/usr/local/bin/netcarry_engine.py"
    if [ -f "netcarry.py" ]; then
        cp netcarry.py "$PY_ENGINE_PATH"
    else
        wget -q -O "$PY_ENGINE_PATH" http://fhdh.cnfte.top/netcarry.py || curl -sSfL http://fhdh.cnfte.top/netcarry.py -o "$PY_ENGINE_PATH"
    fi

    cat << 'EOF' > "$TARGET_BIN"
#!/bin/bash
ulimit -n 65535 2>/dev/null
exec python3 /usr/local/bin/netcarry_engine.py "$@"
EOF
    chmod +x "$PY_ENGINE_PATH"
    INSTALLED_MODE="Python3 Async Fallback"
fi

# 4. 系统优化与权限设置
echo -e "[3/4] 配置可执行权限与内核并发参数..."
chmod +x "$TARGET_BIN"
ulimit -n 65535 2>/dev/null || true

# 5. 完成提示
echo -e "[4/4] 验证安装..."
echo -e "${GREEN}"
echo "================================================"
echo "    NETCARRY ULTIMATE 安装成功！"
echo "    安装引擎模式: ${INSTALLED_MODE}"
echo "================================================"
echo -e "${NC}"
echo "常用指令推荐:"
echo "  netcarry --help                    查看完整参数手册"
echo "  netcarry 1.1.1.1 -m UDP -t 256     全速带宽打满测试"
echo "  netcarry 1.1.1.1 -m UDP --weak     弱设备/路由器低功耗防护模式"
echo "  netcarry example.com -m CC -p 443  CC 穿透真实模拟压测"
echo "  netcarry cdn.domain.com -m BRUSH   CDN 刷流与带宽计费穿透"
echo "------------------------------------------------"