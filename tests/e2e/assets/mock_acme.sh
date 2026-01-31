#!/bin/bash
# Mock acme.sh for E2E testing
# Simulates certificate issuance and installation

set -e

# Parse arguments
DOMAIN=""
INSTALL_CERT=false
ISSUE=false

while [[ $# -gt 0 ]]; do
  case $1 in
  --issue)
    ISSUE=true
    shift
    ;;
  --installcert | --install-cert)
    INSTALL_CERT=true
    shift
    ;;
  -d)
    DOMAIN="$2"
    shift
    shift
    ;;
  --key-file)
    KEY_FILE="$2"
    shift
    shift
    ;;
  --fullchain-file)
    CERT_FILE="$2"
    shift
    shift
    ;;
  *)
    shift
    ;;
  esac
done

if [ "$ISSUE" = true ]; then
  echo "[Mock ACME] Issuing certificate for $DOMAIN"

  # Create directory structure similar to acme.sh
  # ~/.acme.sh/<domain>/
  BASE_DIR="$HOME/.acme.sh/$DOMAIN"
  mkdir -p "$BASE_DIR"

  # Generate self-signed cert
  openssl req -x509 -newkey rsa:2048 -keyout "$BASE_DIR/$DOMAIN.key" -out "$BASE_DIR/$DOMAIN.cer" -days 365 -nodes -subj "/CN=$DOMAIN" 2>/dev/null

  # Copy to fullchain
  cp "$BASE_DIR/$DOMAIN.cer" "$BASE_DIR/fullchain.cer"

  echo "[Mock ACME] Certificate issued successfully."
  exit 0
fi

if [ "$INSTALL_CERT" = true ]; then
  echo "[Mock ACME] Installing certificate for $DOMAIN"

  if [ -z "$KEY_FILE" ] || [ -z "$CERT_FILE" ]; then
    echo "Error: Key file or Cert file not specified"
    exit 1
  fi

  BASE_DIR="$HOME/.acme.sh/$DOMAIN"

  # Verify source exists
  if [ ! -f "$BASE_DIR/$DOMAIN.key" ]; then
    echo "Error: Source key not found at $BASE_DIR/$DOMAIN.key"
    exit 1
  fi

  # Ensure target dir exists
  mkdir -p "$(dirname "$KEY_FILE")"
  mkdir -p "$(dirname "$CERT_FILE")"

  cp "$BASE_DIR/$DOMAIN.key" "$KEY_FILE"
  cp "$BASE_DIR/fullchain.cer" "$CERT_FILE"

  echo "[Mock ACME] Certificate installed to:"
  echo "  Key: $KEY_FILE"
  echo "  Cert: $CERT_FILE"
  exit 0
fi

# Default success for other commands (upgrade, etc)
echo "[Mock ACME] Command mocked successfully"
exit 0
