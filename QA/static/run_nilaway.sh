#!/bin/bash
# Wrapper script to run NilAway static analysis

set -e

# Install NilAway if not present
if ! command -v nilaway &> /dev/null; then
    echo "Installing NilAway..."
    go install go.uber.org/nilaway/cmd/nilaway@latest
fi

echo "Running NilAway analysis..."
# We run on all packages but exclude test files to focus on production code.
nilaway -test=false ./... || echo "NilAway found issues (Exit Code: $?)"
