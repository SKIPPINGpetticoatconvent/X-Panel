#!/bin/bash
# E2E test for service status

echo "Checking x-ui service status..."

# Check via systemctl first
if command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet x-ui; then
  echo "x-ui service is running (systemd)."
  exit 0
fi

# Fallback: Check for process or port (Local Runner Mode)
echo "Systemd service not active. Checking for local process..."
if pgrep -f "x-ui" >/dev/null; then
  echo "x-ui process is running."
  exit 0
fi

# Fallback 2: Check port
if nc -z 127.0.0.1 13688; then
  echo "x-ui is listening on port 13688."
  exit 0
fi

echo "x-ui service is NOT running."
exit 1
