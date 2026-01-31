#!/bin/bash
set -e

MODE=$1
XPANEL_VERSION=$2

echo ">>> [Container] Starting Domain Cert Verification"

# 0. Install dependencies
echo ">>> [Container] Installing dependencies..."
apt-get update >/dev/null 2>&1
apt-get install -y openssl sqlite3 socat >/dev/null 2>&1

# 1. Install X-Panel
mkdir -p /root/mock_server
if [ "${MODE}" == "local" ]; then
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

# 2. Setup Mock ACME
mkdir -p /root/.acme.sh
cp /root/assets/mock_acme.sh /root/.acme.sh/acme.sh
chmod +x /root/.acme.sh/acme.sh

# 3. Simulate Domain Cert Application
echo ">>> [Container] Simulating Domain Cert Application..."
TEST_DOMAIN="example.com"
CERT_DIR="/root/cert/${TEST_DOMAIN}"
mkdir -p "${CERT_DIR}"

# Mock x-ui.sh execution environment
cp /usr/bin/x-ui /root/x-ui-test.sh

# Disable main execution block in x-ui.sh
sed -i 's/if \[\[ $# -gt 0 \]\]; then/if false; then/' /root/x-ui-test.sh
sed -i 's/^else$/elif false; then/' /root/x-ui-test.sh

# Mock confirm function safely
sed -i 's/^confirm() {/confirm_orig() {/' /root/x-ui-test.sh
# Append new confirm definition
# We do this later via cat to avoid sed escaping hell, but let's do it cleanly here if possible.
# Actually, the cat block below appends content, so we can define confirm there or here via echo.
echo "confirm() { return 0; }" >>/root/x-ui-test.sh

# Mock read
sed -i 's/^read -rp.*/# read skipped/' /root/x-ui-test.sh

cat <<EOF >>/root/x-ui-test.sh

# Test Driver
LOGI "Starting Test Driver (Domain)"

certPath="${CERT_DIR}"
domain="${TEST_DOMAIN}"

echo ">>> [Container] Manually invoking Mock ACME (simulating x-ui.sh logic)..."
~/.acme.sh/acme.sh --issue -d "${TEST_DOMAIN}" --standalone --server letsencrypt --force

echo ">>> [Container] Manually invoking Mock ACME Install..."
~/.acme.sh/acme.sh --installcert -d "${TEST_DOMAIN}" \\
    --key-file "\${certPath}/privkey.pem" \\
    --fullchain-file "\${certPath}/fullchain.pem" \\
    --reloadcmd "systemctl restart x-ui"

EOF

bash /root/x-ui-test.sh

# 4. Verification
echo ">>> [Container] Verifying Certificate Installation..."
CERT_PATH="${CERT_DIR}/fullchain.pem"
KEY_PATH="${CERT_DIR}/privkey.pem"

if [ ! -f "$CERT_PATH" ]; then
  echo "❌ Certificate file missing: $CERT_PATH"
  exit 1
fi

# 5. Configure Panel
echo ">>> [Container] Configuring Panel..."
/usr/local/x-ui/x-ui cert -webCert "$CERT_PATH" -webCertKey "$KEY_PATH"
systemctl restart x-ui
sleep 2

# 6. Check Database
DB_PATH="/etc/x-ui/x-ui.db"
CURRENT_CERT=$(sqlite3 "$DB_PATH" "SELECT value FROM settings WHERE key='webCertFile';")
echo "DB Cert: $CURRENT_CERT"

if [ "$CURRENT_CERT" != "$CERT_PATH" ]; then
  echo "❌ DB not updated correctly."
  exit 1
fi

echo "✅ Domain Cert Test Passed!"

# Cleanup
if [ "${MODE}" == "local" ]; then
  kill "${SERVER_PID}" || true
fi
