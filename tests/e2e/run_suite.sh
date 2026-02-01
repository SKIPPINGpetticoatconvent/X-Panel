#!/bin/bash
set -e

# 基础配置
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$PROJECT_ROOT"

echo "Running E2E Checks..."
chmod +x tests/e2e/runner.sh

# 1. 核心安装测试
echo ">> [Core] Installation Test"
./tests/e2e/runner.sh --mode local --test install

# 2. 自动发现循环
# 排除特定测试（空格分隔）
DEFAULT_EXCLUDE="in_container ip_cert domain_cert ssl_fallback"
exclude_list="${EXCLUDE_LIST:-$DEFAULT_EXCLUDE}"

for f in tests/e2e/verify_*.sh; do
  name=$(basename "$f" .sh | sed 's/^verify_//')

  # 检查是否排除
  if [[ " $exclude_list " == *" $name "* ]]; then
    echo ">> [Skip] $name (Excluded)"
    continue
  fi

  echo ">> [Auto] Running Test: $name"
  ./tests/e2e/runner.sh --mode local --test "$name"
done
