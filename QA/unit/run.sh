#!/bin/bash
# Unit test runner

echo "Running unit tests..."
go test -v ./util/... ./database/...
