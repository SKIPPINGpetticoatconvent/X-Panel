#!/bin/sh
set -e

echo "🟢 [Test] Starting initialization..."

# 1. 初始化 x-ui (生成数据库文件)
# 注意：这里使用固定参数，确保环境一致
/app/x-ui setting -username admin -password admin -port 13688

# 2. 强行修改数据库 (核弹级修复 404 问题)
echo "🟢 [Test] Patching database to force root path..."
sqlite3 /etc/x-ui/x-ui.db "UPDATE settings SET value='/' WHERE key='webBasePath';"

# 3. 启动主程序
echo "🟢 [Test] Starting x-ui server..."
exec /app/x-ui