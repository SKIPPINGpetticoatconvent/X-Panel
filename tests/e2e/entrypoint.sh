#!/bin/sh
set -e

echo "🟢 [Test] Starting initialization..."

# 确保 bin 目录下的数据库文件有正确的目录
mkdir -p /etc/x-ui

# 1. 初始化 (生成数据库)
/app/x-ui setting -username admin -password admin -port 13688

# 2. 强行修改数据库 (修复 404)
echo "🟢 [Test] Patching database..."
sqlite3 /etc/x-ui/x-ui.db "UPDATE settings SET value='/' WHERE key='webBasePath';"

# 3. 启动
echo "🟢 [Test] Starting x-ui..."
exec /app/x-ui