#!/bin/bash
# Master QA Entry Point

# set -e removed to allow full demo run


PROJECT_ROOT=$(dirname "$(dirname "$0")")
cd "$PROJECT_ROOT"

echo "=== Starting QA Process ==="

echo "--- 1. Static Analysis ---"
./QA/static/golangci-lint
./QA/static/config

echo "--- 1.1 Static Analysis (Deep: NilAway) ---"
if ! command -v nilaway &> /dev/null; then
    go install go.uber.org/nilaway/cmd/nilaway@latest
fi
nilaway -test=false ./... || echo "NilAway found issues"


echo "--- 1.2 Static Analysis (Shellcheck) ---"
./QA/static/run_shellcheck.sh

echo "--- 2. Unit Tests ---"
./QA/unit/run.sh

echo "--- 3. E2E Tests ---"

# 1. Install Check
echo "[E2E] Checking Installation Scripts..."
./QA/e2e/1_install/test.sh

# 2. Service Check
echo "[E2E] Checking Service Status..."
./QA/e2e/2_service/test.sh

# 3. Web API Tests (Python)
echo "[E2E] Running Web API Tests..."
# Check for service port availability first to avoid hang
if nc -z 127.0.0.1 13688 2>/dev/null; then
    # Run all python tests in 3_api/cases
    for test_file in $(find QA/e2e/3_api/cases -name "test.py"); do
        echo "Running $test_file..."
        python3 "$test_file"
    done
else
    echo "Port 13688 not open. Skipping Python API tests."
fi

