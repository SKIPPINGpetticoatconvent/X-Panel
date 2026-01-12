#!/bin/sh
set -e

echo "🟢 [Test] Starting initialization..."

# 1. 清理旧数据库，确保生成全新的
# 我们不猜测数据库到底在哪里，直接把可能的位置都删了
rm -f /etc/x-ui/x-ui.db /app/x-ui.db

# 确保数据库目录存在
mkdir -p /etc/x-ui

# 2. 初始化 (生成数据库)
# 当没有旧数据库干扰时，setting 命令会创建一个全新的数据库
# 并严格按照我们的参数（-webBasePath /）写入配置
echo "🟢 [Test] Initializing settings..."
/app/x-ui setting -username admin -password admin -port 13688 -webBasePath /

# 3. 启动
echo "🟢 [Test] Starting x-ui..."
exec /app/x-ui