#!/bin/sh

# 设置容器运行标识
export XPANEL_RUN_IN_CONTAINER=true

# Start fail2ban
[ $XUI_ENABLE_FAIL2BAN == "true" ] && fail2ban-client -x start

# Run x-ui
exec /app/x-ui
