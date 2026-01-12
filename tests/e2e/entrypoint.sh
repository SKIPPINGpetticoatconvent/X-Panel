#!/bin/sh
set -e

echo "🟢 [Test] Starting initialization..."

# 确保数据库目录存在
mkdir -p /etc/x-ui

# 1. 初始化 (生成数据库) - 明确指定 webBasePath 为 /
echo "🟢 [Test] Initializing settings..."
/app/x-ui setting -username admin -password admin -port 13688 -webBasePath /

# 2. 验证并强制修改数据库 (双重保险)
echo "🟢 [Test] Patching database..."
sqlite3 /etc/x-ui/x-ui.db "UPDATE settings SET value='/' WHERE key='webBasePath';"

# 3. 验证设置
echo "🟢 [Test] Verifying webBasePath..."
BASEPATH=$(sqlite3 /etc/x-ui/x-ui.db "SELECT value FROM settings WHERE key='webBasePath';")
echo "🟢 [Test] Current webBasePath: '$BASEPATH'"

# 4. 启动
echo "🟢 [Test] Starting x-ui..."
exec /app/x-ui