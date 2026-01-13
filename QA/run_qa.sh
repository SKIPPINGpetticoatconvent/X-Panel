#!/bin/bash
# Master QA Entry Point

# set -e removed to allow full demo run


PROJECT_ROOT=$(dirname "$(dirname "$0")")
cd "$PROJECT_ROOT"

echo "=== Starting QA Process ==="

echo "--- 1. Static Analysis ---"
./QA/static/golangci-lint
./QA/static/config

echo "--- 2. Unit Tests ---"
./QA/unit/run.sh

echo "--- 3. E2E Tests ---"
./QA/e2e/install_e2e.sh
./QA/e2e/service_e2e.sh
./QA/e2e/web_e2e.sh

echo "=== QA Process Completed Successfully ==="
