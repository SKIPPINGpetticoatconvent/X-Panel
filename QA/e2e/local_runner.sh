#!/bin/bash
# QA/e2e/local_runner.sh
# Runs x-ui locally for E2E testing without root/systemd

set -e

PROJECT_ROOT=$(dirname "$(dirname "$(dirname "$0")")")
cd "$PROJECT_ROOT"

# Configuration for local run
export XUI_DB_FOLDER="$PROJECT_ROOT/test_env/db"
export XUI_LOG_FOLDER="$PROJECT_ROOT/test_env/log"
export XUI_BIN_FOLDER="$PROJECT_ROOT/test_env/bin"
export XUI_SNI_FOLDER="$PROJECT_ROOT/test_env/sni"

# Cleanup function
cleanup() {
    echo "Stopping background x-ui process..."
    if [ -n "$XUI_PID" ]; then
        kill "$XUI_PID" || true
        wait "$XUI_PID" || true
    fi
    echo "Cleanup complete."
}
trap cleanup EXIT

echo "=== Setting up Local Test Environment ==="
mkdir -p "$XUI_DB_FOLDER"
mkdir -p "$XUI_LOG_FOLDER"
mkdir -p "$XUI_BIN_FOLDER"
mkdir -p "$XUI_SNI_FOLDER"

echo "=== Building x-ui ==="
go build -o x-ui main.go

echo "=== Starting x-ui (Environment: Local) ==="
# Start in background
./x-ui > "$XUI_LOG_FOLDER/stdout.log" 2>&1 &
XUI_PID=$!
echo "x-ui started with PID $XUI_PID"

echo "=== Waiting for service to start ==="
# Wait for port 13688 (default)
MAX_RETRIES=30
for ((i=1; i<=MAX_RETRIES; i++)); do
    if nc -z 127.0.0.1 13688; then
        echo "Port 13688 is open!"
        break
    fi
    echo "Waiting for port 13688... ($i/$MAX_RETRIES)"
    sleep 1
    
    # Check if process died
    if ! kill -0 "$XUI_PID" 2>/dev/null; then
        echo "x-ui process died unexpectedly!"
        cat "$XUI_LOG_FOLDER/stdout.log"
        exit 1
    fi
done

if ! nc -z 127.0.0.1 13688; then
    echo "Timeout waiting for port 13688"
    exit 1
fi

echo "=== Running E2E Tests ==="
# Run service check
./QA/e2e/2_service/test.sh

# Run API tests if present (skipping for now if empty folder, but structure is there)
if [ -d "QA/e2e/3_api/cases" ]; then
    for test_file in $(find QA/e2e/3_api/cases -name "test.py"); do
        echo "Running $test_file..."
        python3 "$test_file"
    done
fi

echo "=== ALL TESTS PASSED ==="
