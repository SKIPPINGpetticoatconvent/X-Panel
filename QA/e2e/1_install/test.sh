#!/bin/bash
# E2E test for installation script

echo "Testing install.sh..."
PROJECT_ROOT=$(dirname "$(dirname "$(dirname "$(dirname "$0")")")")
INSTALL_SCRIPT="$PROJECT_ROOT/install.sh"
echo "Looking for install.sh at: $INSTALL_SCRIPT"

if [ ! -f "$INSTALL_SCRIPT" ]; then
  echo "install.sh not found!"
  exit 1
fi

# Basic syntax check to avoid actually installing and messing up the environment during test
bash -n "$INSTALL_SCRIPT"
if [ $? -eq 0 ]; then
  echo "install.sh syntax check passed."
else
  echo "install.sh syntax check failed."
  exit 1
fi
