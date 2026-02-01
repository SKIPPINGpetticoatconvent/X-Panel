#!/bin/bash
set -e

# 基础配置
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$PROJECT_ROOT"

echo "Running E2E Checks..."
chmod +x tests/e2e/runner.sh

# 1. 核心安装测试 (构建并保留构件)
echo ">> [Core] Installation Test"
./tests/e2e/runner.sh --mode local --test install --keep-artifact

# 2. 自动发现循环
# 排除特定测试（空格分隔）
DEFAULT_EXCLUDE="in_container ip_cert domain_cert ssl_fallback"
exclude_list="${EXCLUDE_LIST:-$DEFAULT_EXCLUDE}"

pids=""

for f in tests/e2e/verify_*.sh; do
  name=$(basename "$f" .sh | sed 's/^verify_//')

  # 检查是否排除
  if [[ " $exclude_list " == *" $name "* ]]; then
    echo ">> [Skip] $name (Excluded)"
    continue
  fi

  echo ">> [Auto] Running Test: $name (Parallel)"
  # 使用唯一容器名称，跳过构建，并行运行，且保留构件
  ./tests/e2e/runner.sh --mode local --test "$name" --skip-build --container-name "xpanel-e2e-$name" --keep-artifact &
  pids="$pids $!"
done

# 等待所有并行任务完成
if [ -n "$pids" ]; then
  echo ">> Waiting for parallel tests..."
  wait $pids
fi

# 最终清理（移除共享构件）
rm -f "$PROJECT_ROOT/x-ui-linux-amd64.tar.gz"
