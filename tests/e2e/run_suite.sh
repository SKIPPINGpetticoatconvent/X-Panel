#!/bin/bash
set -e

# Base Config
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$PROJECT_ROOT"

echo "Running E2E Checks..."
chmod +x tests/e2e/runner.sh

# 1. Core Installation Test
echo ">> [Core] Installation Test"
./tests/e2e/runner.sh --mode local --test install

# 2. Auto-discovery Loop
# Exclude specific tests (space separated)
# Exclude specific tests (space separated)
DEFAULT_EXCLUDE="in_container ip_cert domain_cert ssl_fallback"
exclude_list="${EXCLUDE_LIST:-$DEFAULT_EXCLUDE}"

for f in tests/e2e/verify_*.sh; do
  name=$(basename "$f" .sh | sed 's/^verify_//')

  # Check exclusion
  if [[ " $exclude_list " == *" $name "* ]]; then
    echo ">> [Skip] $name (Excluded)"
    continue
  fi

  echo ">> [Auto] Running Test: $name"
  ./tests/e2e/runner.sh --mode local --test "$name"
done
