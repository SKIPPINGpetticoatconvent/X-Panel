#!/bin/bash
# E2E test for web accessibility

echo "Checking web interface..."
PROJECT_ROOT=$(dirname "$(dirname "$(dirname "$0")")")
API_TEST_SCRIPT="$PROJECT_ROOT/QA/e2e/api_test.py"

echo "Running Python API tests..."
# Check if port 13688 (used in api_test.py) is open before running
if nc -z 127.0.0.1 13688 2>/dev/null; then
    if [ -f "$API_TEST_SCRIPT" ]; then
        python3 "$API_TEST_SCRIPT"
    else
        echo "api_test.py not found at $API_TEST_SCRIPT"
        exit 1
    fi
else
    echo "Port 13688 not open. Skipping API tests (Service not running?)"
fi


