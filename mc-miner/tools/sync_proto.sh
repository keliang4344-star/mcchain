#!/usr/bin/env bash
# 把链仓库 proto/ 下的移动端相关 proto 同步为 mc-miner/contracts/ 的只读副本。
#
# 用法：
#   tools/sync_proto.sh          # 同步
#   tools/sync_proto.sh --check  # 只校验（CI / pre-commit），不一致时退出码 1
#
# 依赖：bash、cmp。Windows 下用 Git Bash / WSL 运行。
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MINER_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CHAIN_ROOT="$(cd "$MINER_ROOT/.." && pwd)"
SRC="$CHAIN_ROOT/proto"
DST="$MINER_ROOT/contracts"

FILES=(
  "mcchain/depin/tx.proto"
  "mcchain/phonenode/tx.proto"
)

check_only=0
if [[ "${1:-}" == "--check" ]]; then
  check_only=1
elif [[ -n "${1:-}" ]]; then
  echo "用法: $0 [--check]" >&2
  exit 2
fi

status=0
for f in "${FILES[@]}"; do
  if [[ ! -f "$SRC/$f" ]]; then
    echo "错误: 链仓库缺少 $SRC/$f" >&2
    exit 2
  fi
  if [[ $check_only -eq 1 ]]; then
    if ! cmp -s "$SRC/$f" "$DST/$f"; then
      echo "不同步: contracts/$f（请运行 tools/sync_proto.sh）" >&2
      status=1
    fi
  else
    mkdir -p "$(dirname "$DST/$f")"
    cp "$SRC/$f" "$DST/$f"
    echo "已同步: contracts/$f"
  fi
done

if [[ $check_only -eq 1 && $status -eq 0 ]]; then
  echo "proto 副本与链仓库一致"
fi
exit $status
