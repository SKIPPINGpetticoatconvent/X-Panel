#!/bin/bash

# Find all shell scripts and run shellcheck
# Excluding node_modules, .git, and .idea directories

echo "Running Shellcheck..."

# Check if shellcheck is installed
if ! command -v shellcheck &>/dev/null; then
  echo "Error: shellcheck is not installed."
  echo "Please install it via your package manager (e.g., apt install shellcheck)."
  exit 1
fi

# Find files and run shellcheck
# We look for .sh files and files with sh/bash shebangs if they don't have extension (optional, but sticking to .sh for now as per project structure)
# Also explicitly including the root scripts x-ui.sh and install.sh just in case find logic misses them due to depth or whatever, though find should catch them.

failed=0

# Use find to locate .sh files, excluding unwanted directories
# We'll also check specific known root scripts if they don't show up in the find command for some reason, but find is robust.
while IFS= read -r file; do
  echo "Checking $file..."
  if ! shellcheck --severity=error "$file"; then
    echo "FAILED: $file"
    failed=1
  fi
done < <(find . -type d \( -name node_modules -o -name .git -o -name .agent -o -name .github \) -prune -o -type f -name "*.sh" -print)

# Explicitly check root scripts if they haven't been picked up (find . includes ./install.sh)
# Logic above covers ./install.sh and ./x-ui.sh

if [ $failed -ne 0 ]; then
  echo "Shellcheck found issues."
  exit 1
else
  echo "Shellcheck passed!"
  exit 0
fi
