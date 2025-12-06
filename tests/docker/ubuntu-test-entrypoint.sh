#!/bin/bash
# Ubuntu测试环境入口脚本

set -e

echo "=== Ubuntu测试环境启动 ==="
echo "操作系统类型: Ubuntu 22.04"
echo "Go版本: $(go version)"
echo "当前用户: $(whoami)"
echo "工作目录: $(pwd)"

# 设置测试环境变量
export TEST_OS_TYPE=ubuntu
export SKIP_REAL_INSTALL_TESTS=${SKIP_REAL_INSTALL_TESTS:-true}

# 检查必要工具
echo "=== 检查测试环境 ==="
command -v go >/dev/null 2>&1 || { echo "Go未安装!"; exit 1; }
command -v git >/dev/null 2>&1 || { echo "Git未安装!"; exit 1; }

# 显示系统信息
echo "=== 系统信息 ==="
uname -a
cat /etc/os-release

# 显示网络信息
echo "=== 网络配置 ==="
ip addr show || ifconfig
ss -tlnp || netstat -tlnp

# 检查SSH服务配置（用于测试SSH端口检测）
echo "=== SSH配置 ==="
if [ -f /etc/ssh/sshd_config ]; then
    echo "SSH端口配置:"
    grep -E "^Port|^#Port" /etc/ssh/sshd_config || echo "使用默认端口22"
else
    echo "SSH配置文件不存在"
fi

# 显示包管理器信息
echo "=== 包管理器信息 ==="
command -v apt-get >/dev/null 2>&1 && echo "APT包管理器可用"
command -v dpkg >/dev/null 2>&1 && echo "DPKG包管理器可用"

# 运行测试
echo "=== 开始运行防火墙测试 ==="
echo "测试环境变量:"
echo "  TEST_OS_TYPE=$TEST_OS_TYPE"
echo "  SKIP_REAL_INSTALL_TESTS=$SKIP_REAL_INSTALL_TESTS"

# 进入项目根目录
cd /app

# 如果指定了要运行的特定测试
if [ -n "$1" ]; then
    echo "运行指定测试: $1"
    go test -v -run "$1" ./tests/
else
    echo "运行所有防火墙测试"
    go test -v ./tests/firewall/test_firewall_*.go
fi

echo "=== Ubuntu测试环境完成 ==="