#!/bin/bash
set -e

MODE=$1
XPANEL_VERSION=$2

echo ">>> [Container] Starting SSL Fallback Verification"

# 0. Install dependencies
echo ">>> [Container] Installing dependencies (openssl, sqlite3)..."
apt-get update >/dev/null 2>&1
apt-get install -y openssl sqlite3 >/dev/null 2>&1

# 1. Install X-Panel (Reuse code from verify_in_container.sh logic)
mkdir -p /root/mock_server
if [ "${MODE}" == "local" ]; then
  echo ">>> [Container] Setting up Local Mock Environment..."
  if [ ! -f "/root/x-ui-linux-amd64.tar.gz" ]; then
    echo "Error: /root/x-ui-linux-amd64.tar.gz not found!"
    exit 1
  fi
  MOCK_DIR="/root/mock_server/releases/download/${XPANEL_VERSION}"
  mkdir -p "${MOCK_DIR}"
  cp /root/x-ui-linux-amd64.tar.gz "${MOCK_DIR}/x-ui-linux-amd64.tar.gz"

  cd /root/mock_server
  python3 -m http.server 8080 &
  SERVER_PID=$!
  sleep 2

  sed -i "s|https://github.com/SKIPPINGpetticoatconvent/X-Panel/releases/download|http://127.0.0.1:8080/releases/download|g" /root/install.sh
fi

echo ">>> [Container] Installing X-Panel..."
cd /root
chmod +x install.sh
printf "\nn\nn\n" | ./install.sh "${XPANEL_VERSION}"

# 2. Setup Short-Lived Certificate
echo ">>> [Container] Generating Expiring Certificate (2 days valid)..."
mkdir -p /root/cert/test
CERT_PATH="/root/cert/test/fullchain.pem"
KEY_PATH="/root/cert/test/privkey.pem"

# Generate self-signed cert valid for 2 days
openssl req -x509 -newkey rsa:2048 -keyout "$KEY_PATH" -out "$CERT_PATH" -days 2 -nodes -subj "/CN=test.example.com" 2>/dev/null

if [ ! -f "$CERT_PATH" ]; then
  echo "❌ Failed to generate certificate"
  exit 1
fi

# 3. Stop Panel & Configure DB
echo ">>> [Container] Configuring X-Panel to use expiring cert..."
systemctl stop x-ui

# Use CLI to set certs (ensures keys exist)
/usr/local/x-ui/x-ui cert -webCert "$CERT_PATH" -webCertKey "$KEY_PATH"

# Verify update took effect
DB_PATH="/etc/x-ui/x-ui.db"
CURRENT_CERT=$(sqlite3 "$DB_PATH" "SELECT value FROM settings WHERE key='webCertFile';")
echo ">>> [Container] Cert configured as: '$CURRENT_CERT'"
if [ "$CURRENT_CERT" != "$CERT_PATH" ]; then
  echo "❌ Failed to set certificate via CLI"
  exit 1
fi

# 4. Start Panel
echo ">>> [Container] Starting X-Panel..."
systemctl start x-ui
sleep 5

if ! systemctl is-active --quiet x-ui; then
  echo "❌ Service failed to start"
  journalctl -u x-ui --no-pager
  exit 1
fi

# 5. Wait for Fallback (CertMonitor runs every minute at :05)
echo ">>> [Container] Waiting for CertMonitorJob (approx 70s)..."
# We wait enough time to cover a full minute boundary + buffer
sleep 70

# 6. Verify Fallback
echo ">>> [Container] Verifying Fallback Results..."

# Check DB for new cert path
NEW_CERT_PATH=$(sqlite3 "$DB_PATH" "SELECT value FROM settings WHERE key='webCertFile';")
echo "OLD: $CERT_PATH"
echo "NEW: $NEW_CERT_PATH"

if [ "$NEW_CERT_PATH" == "$CERT_PATH" ]; then
  echo "❌ DB still points to old certificate. Fallback failed!"
  echo ">>> [Container] Dumping CertMonitor Logs:"
  grep "CertMonitor" /var/log/syslog || journalctl -u x-ui --no-pager | grep "CertMonitor"
  exit 1
fi

# Check if new path is in the expected format (IP-based)
# Usually /root/cert/<IP>/fullchain.pem
if [[ $NEW_CERT_PATH != *"/root/cert/"* ]]; then
  echo "❌ New cert path looks suspicious: '$NEW_CERT_PATH'"
  echo ">>> [Container] Dumping ALL Logs:"
  journalctl -u x-ui --no-pager
  exit 1
fi

# Check file existence
if [ ! -f "$NEW_CERT_PATH" ]; then
  echo "❌ New certificate file does not exist at $NEW_CERT_PATH"
  exit 1
fi

# Check if old file was deleted
if [ -f "$CERT_PATH" ]; then
  echo "❌ Old certificate file was NOT deleted"
  exit 1
fi

echo "✅ Fallback Successful!"
echo "   - DB updated to: $NEW_CERT_PATH"
echo "   - Old cert deleted"
echo "   - New cert exists"

# Cleanup
if [ "${MODE}" == "local" ]; then
  kill ${SERVER_PID} || true
fi
