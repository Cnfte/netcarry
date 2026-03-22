#!/bin/bash
# wpbench 安装脚本 (二进制版)
# 用法: sudo bash install.sh [--password 密钥] [--port 端口]

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

# ─── 配置：修改为你自己的下载基础 URL ───
# 脚本会自动拼接架构后缀，例如:
#   https://your-oss.example.com/wpbench/wpbench_linux_amd64
#   https://your-oss.example.com/wpbench/wpbench_linux_arm64
# 提前编译并上传到对象存储:
#   GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o wpbench_linux_amd64  wpbench.go
#   GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o wpbench_linux_arm64  wpbench.go
#   GOOS=linux GOARCH=arm   go build -ldflags="-s -w" -o wpbench_linux_armv6l wpbench.go
BASE_URL="https://your-oss.example.com/wpbench"

# ─── 默认参数 ───
WB_PASSWORD="wpbench2024"
WB_PORT="36499"
INSTALL_DIR="/opt/wpbench"
BIN_PATH="/usr/local/bin/wpbench"
SERVICE_NAME="wpbench"

# ─── 参数解析 ───
while [[ $# -gt 0 ]]; do
  case $1 in
    --password) WB_PASSWORD="$2"; shift 2 ;;
    --port)     WB_PORT="$2";     shift 2 ;;
    --dir)      INSTALL_DIR="$2"; shift 2 ;;
    --url)      BASE_URL="$2";    shift 2 ;;
    --help)
      echo "用法: sudo bash install.sh [选项]"
      echo "  --password <密钥>   API 鉴权密钥 (默认: wpbench2024)"
      echo "  --port     <端口>   监听端口     (默认: 36499)"
      echo "  --url      <地址>   二进制下载基础 URL"
      exit 0 ;;
    *) echo -e "${RED}未知参数: $1${NC}"; exit 1 ;;
  esac
done

# ─── 权限检查 ───
if [ "$EUID" -ne 0 ]; then
  echo -e "${RED}请使用 sudo 运行此脚本。${NC}"
  exit 1
fi

echo -e "${CYAN}"
echo "╔══════════════════════════════════════╗"
echo "║        wpbench 安装程序 v2.0         ║"
echo "╚══════════════════════════════════════╝"
echo -e "${NC}"

# ─── Step 1: 检测架构 ───
echo -e "${CYAN}[1/4] 检测系统架构...${NC}"
ARCH=$(uname -m)
case $ARCH in
  x86_64)  BIN_SUFFIX="linux_amd64"  ;;
  aarch64) BIN_SUFFIX="linux_arm64"  ;;
  armv7l)  BIN_SUFFIX="linux_armv6l" ;;
  *)
    echo -e "${RED}不支持的架构: $ARCH${NC}"
    exit 1 ;;
esac
echo -e "  ${GREEN}✓ 架构: $ARCH → $BIN_SUFFIX${NC}"

# ─── Step 2: 检查下载工具 ───
echo -e "${CYAN}[2/4] 检查下载工具...${NC}"
DOWNLOADER=""
if command -v wget &>/dev/null; then
  DOWNLOADER="wget"
elif command -v curl &>/dev/null; then
  DOWNLOADER="curl"
else
  echo -e "  ${YELLOW}未找到 wget/curl，尝试安装 wget...${NC}"
  if command -v apt-get &>/dev/null; then
    apt-get update -qq && apt-get install -y wget
  elif command -v yum &>/dev/null; then
    yum install -y wget
  elif command -v dnf &>/dev/null; then
    dnf install -y wget
  else
    echo -e "${RED}无法安装下载工具，请手动安装 wget 或 curl。${NC}"
    exit 1
  fi
  DOWNLOADER="wget"
fi
echo -e "  ${GREEN}✓ 使用 $DOWNLOADER${NC}"

# ─── Step 3: 下载二进制 ───
echo -e "${CYAN}[3/4] 下载 wpbench_${BIN_SUFFIX}...${NC}"
mkdir -p "$INSTALL_DIR"

DOWNLOAD_URL="${BASE_URL}/wpbench_${BIN_SUFFIX}"
DEST="$INSTALL_DIR/wpbench_agent"

echo -e "  URL: ${YELLOW}$DOWNLOAD_URL${NC}"

if [ "$DOWNLOADER" = "wget" ]; then
  wget -q --show-progress "$DOWNLOAD_URL" -O "$DEST"
else
  curl -L --progress-bar "$DOWNLOAD_URL" -o "$DEST"
fi

# 校验文件非空
if [ ! -s "$DEST" ]; then
  echo -e "${RED}下载失败或文件为空，请检查 URL: $DOWNLOAD_URL${NC}"
  exit 1
fi

chmod +x "$DEST"

# 创建全局命令
cat > "$BIN_PATH" << EOF
#!/bin/bash
$INSTALL_DIR/wpbench_agent "\$@"
EOF
chmod +x "$BIN_PATH"
echo -e "  ${GREEN}✓ 二进制已部署 → $DEST${NC}"

# ─── Step 4: 写配置 + 注册服务 ───
echo -e "${CYAN}[4/4] 配置服务...${NC}"

cat > "$INSTALL_DIR/wpbench.conf" << EOF
# wpbench 配置文件
# 修改后执行: systemctl restart wpbench
PASSWORD=$WB_PASSWORD
PORT=$WB_PORT
EOF
chmod 600 "$INSTALL_DIR/wpbench.conf"

if command -v systemctl &>/dev/null; then
  cat > "/etc/systemd/system/${SERVICE_NAME}.service" << EOF
[Unit]
Description=wpbench Agent - WordPress 压测后端
After=network.target

[Service]
Type=simple
User=nobody
Group=nogroup
WorkingDirectory=$INSTALL_DIR
EnvironmentFile=$INSTALL_DIR/wpbench.conf
ExecStart=$INSTALL_DIR/wpbench_agent -password \${PASSWORD} -port \${PORT}
Restart=on-failure
RestartSec=5s
StartLimitIntervalSec=60s
StartLimitBurst=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable "$SERVICE_NAME"
  systemctl restart "$SERVICE_NAME"

  sleep 1
  if systemctl is-active --quiet "$SERVICE_NAME"; then
    echo -e "  ${GREEN}✓ 服务已启动，开机自启已设置${NC}"
  else
    echo -e "  ${YELLOW}! 服务启动异常，请检查:${NC}"
    echo -e "  ${YELLOW}  journalctl -u $SERVICE_NAME -n 30${NC}"
  fi

else
  # 无 systemd 兜底（OpenVZ、容器等环境）
  echo -e "  ${YELLOW}! 未检测到 systemd，使用 nohup 后台运行${NC}"
  pkill -f "wpbench_agent" 2>/dev/null || true
  nohup "$DEST" -password "$WB_PASSWORD" -port "$WB_PORT" \
    >> "$INSTALL_DIR/wpbench.log" 2>&1 &
  echo $! > "$INSTALL_DIR/wpbench.pid"
  echo -e "  ${GREEN}✓ 后台启动，PID: $(cat $INSTALL_DIR/wpbench.pid)${NC}"
  echo -e "  ${YELLOW}  开机自启请添加到 crontab:${NC}"
  echo -e "  ${YELLOW}  @reboot $DEST -password $WB_PASSWORD -port $WB_PORT >> $INSTALL_DIR/wpbench.log 2>&1 &${NC}"
fi

# ─── 完成 ───
echo ""
echo -e "${GREEN}"
echo "╔══════════════════════════════════════════════════╗"
echo "║           wpbench 安装完成！                     ║"
echo "╚══════════════════════════════════════════════════╝"
echo -e "${NC}"
echo -e "  安装目录:  ${CYAN}$INSTALL_DIR${NC}"
echo -e "  监听端口:  ${CYAN}$WB_PORT${NC}"
echo -e "  API 密钥:  ${CYAN}$WB_PASSWORD${NC}"
echo ""
echo -e "  常用命令:"
echo -e "    ${YELLOW}systemctl status  $SERVICE_NAME${NC}   查看状态"
echo -e "    ${YELLOW}systemctl restart $SERVICE_NAME${NC}   重启"
echo -e "    ${YELLOW}systemctl stop    $SERVICE_NAME${NC}   停止"
echo -e "    ${YELLOW}journalctl -u $SERVICE_NAME -f${NC}    实时日志"
echo ""
echo -e "  前端面板填写:"
echo -e "    Agent: ${CYAN}$(hostname -I | awk '{print $1}')${NC}  端口: ${CYAN}$WB_PORT${NC}  密钥: ${CYAN}$WB_PASSWORD${NC}"
echo ""
echo -e "  卸载:"
echo -e "    ${YELLOW}systemctl disable --now $SERVICE_NAME${NC}"
echo -e "    ${YELLOW}rm -rf $INSTALL_DIR $BIN_PATH /etc/systemd/system/${SERVICE_NAME}.service${NC}"