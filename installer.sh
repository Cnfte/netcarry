#!/bin/bash

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
PURPLE='\033[0;35m'
NC='\033[0m'

echo -e "${PURPLE}正在安装 NetCarry Ultimate CLI...${NC}"

# 1. 权限检查
if [ "$EUID" -ne 0 ]; then 
  echo -e "${RED}错误: 请使用 sudo 运行此脚本。${NC}"
  exit 1
fi

# 2. 安装依赖
echo -e "[1/4] 检查环境..."
if ! command -v python3 &> /dev/null; then
    apt-get update && apt-get install -y python3
fi

if ! command -v wget &> /dev/null; then
    apt-get update && apt-get install -y wget
fi

# 3. 下载文件
wget -O netcarry.py http://fhdh.cnfte.top/netcarry.py

# 4. 部署代码
ENGINE_PATH="/usr/local/bin/netcarry_engine.py"
BIN_PATH="/usr/local/bin/netcarry"

echo -e "[2/4] 写入引擎文件到 $ENGINE_PATH..."
# 获取当前目录下的 netcarry.py 内容并写入
if [ -f "netcarry.py" ]; then
    cp netcarry.py $ENGINE_PATH
else
    echo -e "${RED}错误: 当前目录下未找到 netcarry.py，请先创建该文件。${NC}"
    exit 1
fi

# 5. 创建快捷方式
echo -e "[3/4] 创建快捷命令 'netcarry'..."
cat << 'EOF' > $BIN_PATH
#!/bin/bash
python3 /usr/local/bin/netcarry_engine.py "$@"
EOF

# 6. 设置权限
echo -e "[4/4] 设置可执行权限..."
chmod +x $ENGINE_PATH
chmod +x $BIN_PATH

echo -e "${GREEN}"
echo "================================================"
echo "    NETCARRY ULTIMATE 安装成功！"
echo "================================================"
echo -e "${NC}"
echo "使用方法:"
echo "  netcarry --help             查看完整帮助"
echo "  netcarry <目标> -m CC       开启CC模拟模式"
echo "  netcarry <目标> -m BRUSH    开启域名刷流模式"
echo "------------------------------------------------"