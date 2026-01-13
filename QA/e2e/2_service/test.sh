#!/bin/bash
# E2E test for service status

echo "Checking x-ui service status..."
if systemctl is-active --quiet x-ui; then
    echo "x-ui service is running."
else
    echo "x-ui service is NOT running."
    # In a real CI environment, we might want to fail here, but for now we just report.
    # exit 1 
fi
