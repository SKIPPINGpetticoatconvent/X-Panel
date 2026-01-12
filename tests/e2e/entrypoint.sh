#!/bin/sh
set -e

echo "🟢 [Test] Starting initialization..."

# 1. 清理旧数据库
rm -f /etc/x-ui/x-ui.db /app/x-ui.db

# 确保数据库目录存在
mkdir -p /etc/x-ui

# 调试：打印帮助信息确认参数存在
echo "🟢 [Test] Checking setting command help..."
/app/x-ui setting -help || true

# 2. 初始化 (生成数据库)
# 使用 -key=value 格式以确保正确解析
echo "🟢 [Test] Initializing settings..."
/app/x-ui setting -webBasePath="/" -username=admin -password=admin -port=13688

# 3. 启动
echo "🟢 [Test] Starting x-ui..."
exec /app/x-ui