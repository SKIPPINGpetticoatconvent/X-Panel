#!/bin/bash
set -e

MODE=$1
XPANEL_VERSION=$2

echo ">>> [Container] Starting E2E Verification in MODE: ${MODE}"

# Setup environment
export TERM=xterm
mkdir -p /root/mock_server

if [ "${MODE}" == "local" ]; then
    echo ">>> [Container] Setting up Local Mock Environment..."
    
    # 1. Prepare compiled build as a fake release
    # We expect `x-ui-linux-amd64.tar.gz` to be mapped/copied to /root/
    if [ ! -f "/root/x-ui-linux-amd64.tar.gz" ]; then
        echo "Error: /root/x-ui-linux-amd64.tar.gz not found!"
        exit 1
    fi
    
    # Create structure expected by install.sh download URL
    # URL pattern: .../releases/download/${version}/x-ui-linux-amd64.tar.gz
    # We will serve from /root/mock_server
    MOCK_DIR="/root/mock_server/releases/download/${XPANEL_VERSION}"
    mkdir -p "${MOCK_DIR}"
    cp /root/x-ui-linux-amd64.tar.gz "${MOCK_DIR}/x-ui-linux-amd64.tar.gz"

    # Start simple HTTP server in background
    cd /root/mock_server
    python3 -m http.server 8080 &
    SERVER_PID=$!
    echo ">>> [Container] Mock Server started at http://127.0.0.1:8080 (PID: ${SERVER_PID})"
    sleep 2 # Wait for server startup

    # 2. Modify install.sh to use local server
    # Target line: url="https://github.com/SKIPPINGpetticoatconvent/X-Panel/releases/download/${last_version}/x-ui-linux-$(arch).tar.gz"
    echo ">>> [Container] Patching install.sh..."
    sed -i "s|https://github.com/SKIPPINGpetticoatconvent/X-Panel/releases/download|http://127.0.0.1:8080/releases/download|g" /root/install.sh
    
    # Also patch the version check or ensure we pass the version to install function
else
    echo ">>> [Container] Using Online Mode (Connecting to GitHub)..."
fi

# 3. Run Install Script
echo ">>> [Container] Running install.sh..."
cd /root
chmod +x install.sh

# Mock user input for install.sh
# It asks: "想继续修改吗？... [y/n]?" -> n (default random)
# OR if we want to set specific values.
# Let's say 'n' to keep random/default, which is non-interactive friendly if piped?
# The script reads from stdin.
# Prompts:
# 1. IPv6? (Enter to skip)
# 2. Setup SSL? (n)
# 3. Modify settings? (n)
# Input: "\n" (IPv6) + "n\n" (SSL) + "n\n" (Settings)
printf "\nn\nn\n" | ./install.sh "${XPANEL_VERSION}"

# 4. Verification
echo ">>> [Container] Verifying Installation..."

# Check Service Status
if systemctl is-active --quiet x-ui; then
    echo "✅ Systemd service 'x-ui' is active."
else
    echo "❌ Systemd service 'x-ui' failed to start!"
    echo ">>> [Container] Dumping systemd logs (equivalent to x-ui log):"
    journalctl -u x-ui --no-pager -n 50
    systemctl status x-ui --no-pager
    exit 1
fi

# Check Listening Port
# Default port is random if we chose 'n' for settings.
# Need to find the port.
DB_PATH="/etc/x-ui/x-ui.db"
if [ ! -f "$DB_PATH" ]; then
    echo "❌ Database file not found at $DB_PATH"
    exit 1
fi

# We can query the port using x-ui setting command
# Correct usage: x-ui setting -show true
PORT=$(/usr/local/x-ui/x-ui setting -show true | grep -oP 'port.*: \K\d+')
echo ">>> [Container] Detect X-Panel port: $PORT"

if [ -z "$PORT" ]; then
    echo "❌ Failed to detect port."
    exit 1
fi

# Curl the port
HTTP_CODE=$(curl -o /dev/null -s -w "%{http_code}\n" http://127.0.0.1:${PORT})
if [[ "$HTTP_CODE" =~ 200|404|302 ]]; then
    # 302/200/404 are acceptable responses indicating server is up (depends on path)
    # Usually root redirect to login
    echo "✅ Port $PORT is reachable (HTTP $HTTP_CODE)."
else
    echo "❌ Port $PORT check failed with HTTP $HTTP_CODE"
    exit 1
fi

# 5. Cleanup Mock Server
if [ "${MODE}" == "local" ]; then
    kill ${SERVER_PID} || true
fi

echo ">>> [Container] E2E Verification PASSED!"
