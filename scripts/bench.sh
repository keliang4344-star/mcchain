#!/usr/bin/env bash
#
# MC 公链 · 性能基线测量脚本（docs/PERF_BASELINES.md）
#
# 为什么需要它：性能数字的「可重复性」比「绝对值」更重要。
# 不同时刻、不同机器、不同 devnet 配置下，跑出来的数字要能直接比较，
# 就必须用同一套命令、同一个等待窗口、同一个测点。
#
# 用法：
#   ./scripts/bench.sh all           # 跑完 B1..B5
#   ./scripts/bench.sh b3            # 只跑 B3（转账吞吐）
#   ./scripts/bench.sh --help
#
# 依赖：mcchaind + curl + python3（JSON 解析）+ bc（可选，整数除法）
#
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MCCHAIND="${MCCHAIND:-$REPO_ROOT/build/mcchaind}"
REST="${REST:-http://127.0.0.1:1317}"
RPC="${RPC:-http://127.0.0.1:26657}"

if [[ ! -x "$MCCHAIND" ]]; then
  echo "[bench] 未找到 $MCCHAIND，请先 ./scripts/devnet.sh init && start" >&2
  exit 1
fi

log()  { printf '\033[1;34m[BENCH]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[BENCH]\033[0m %s\n' "$*" >&2; }

# 等待 devnet 起来
wait_ready() {
  for _ in $(seq 1 30); do
    h=$("$MCCHAIND" status --node "$RPC" 2>/dev/null \
      | python3 -c 'import json,sys;print(json.load(sys.stdin)["sync_info"]["latest_block_height"])' 2>/dev/null || echo 0)
    if [[ "$h" != "0" && "$h" != "False" ]]; then
      log "链已就绪 height=$h"
      return 0
    fi
    sleep 1
  done
  die "等待 devnet 超时"
}

die() { printf '\033[1;31m[BENCH]\033[0m %s\n' "$*" >&2; exit 1; }

# B1: REST 最新块延迟
b1() {
  log "B1: REST /blocks/latest × 100"
  python3 - <<PY
import json, statistics, time, urllib.request
samples = []
for _ in range(100):
    t0 = time.time()
    with urllib.request.urlopen("$REST/cosmos/base/tendermint/v1beta1/blocks/latest") as r:
        r.read()
    samples.append((time.time() - t0) * 1000)
samples.sort()
print(json.dumps({
  "p50_ms": samples[50],
  "p95_ms": samples[95],
  "p99_ms": samples[99],
  "max_ms": max(samples),
  "min_ms": min(samples),
}, indent=2))
PY
}

# B2: LCD 序列化吞吐
b2() {
  log "B2: q bank total × 1000"
  python3 - <<PY
import json, statistics, subprocess, time
times = []
for _ in range(1000):
    t0 = time.time()
    subprocess.run(
        ["$MCCHAIND", "q", "bank", "total", "--denom", "umc", "--node", "$RPC"],
        stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=False)
    times.append((time.time() - t0) * 1000)
times.sort()
print(json.dumps({
  "p50_ms": times[500],
  "p95_ms": times[950],
  "p99_ms": times[990],
  "throughput_tps": 1000 / (sum(times) / 1000 / 1000),
}, indent=2))
PY
}

# B3: 转账端到端
b3() {
  log "B3: tx bank send × 50（端到端含 commit）"
  if [[ ! -f "$REPO_ROOT/.devnet/node0/.address" ]]; then
    warn "缺少 node0/.address，请先 ./scripts/devnet.sh init"; return 1
  fi
  from_addr=$(cat "$REPO_ROOT/.devnet/node0/.address")
  to_addr=$(cat "$REPO_ROOT/.devnet/node1/.address" 2>/dev/null || echo "$from_addr")

  start_h=$("$MCCHAIND" status --node "$RPC" \
    | python3 -c 'import json,sys;print(json.load(sys.stdin)["sync_info"]["latest_block_height"])')

  for i in $(seq 1 50); do
    "$MCCHAIND" tx bank send validator0 "$to_addr" 1000umc \
      --chain-id mcchain-dev-1 --keyring-backend test \
      --home "$REPO_ROOT/.devnet/node0" --node "$RPC" \
      --gas auto --gas-adjustment 1.5 --fees 1000umc -y >/dev/null 2>&1
  done

  # 等高度前进 50 块 + 1
  while :; do
    cur_h=$("$MCCHAIND" status --node "$RPC" \
      | python3 -c 'import json,sys;print(json.load(sys.stdin)["sync_info"]["latest_block_height"])')
    [[ $((cur_h - start_h)) -ge 50 ]] && break
    sleep 1
  done

  log "  50 笔全部 commit 完成（$start_h → $cur_h）"
  log "  端到端吞吐 ≈ 50 tx / ($((cur_h - start_h)) × 4) s"
}

# B4: 历史块延迟
b4() {
  log "B4: 任意历史块拉取 × 100"
  python3 - <<PY
import json, subprocess, time
times = []
import random
heights = [random.randint(1, 1000) for _ in range(100)]
for h in heights:
    t0 = time.time()
    subprocess.run(
        ["$MCCHAIND", "q", "block", "--type=height", str(h), "--node", "$RPC"],
        stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=False)
    times.append((time.time() - t0) * 1000)
times.sort()
print(json.dumps({"p50_ms": times[50], "p95_ms": times[95], "p99_ms": times[99]}, indent=2))
PY
}

# B5: 并发转账
b5() {
  log "B5: 并发 200 笔转账（4 并发 worker）"
  if [[ ! -f "$REPO_ROOT/.devnet/node0/.address" ]]; then
    warn "缺少 node0/.address，请先 ./scripts/devnet.sh init"; return 1
  fi
  to_addr=$(cat "$REPO_ROOT/.devnet/node1/.address" 2>/dev/null)
  for w in $(seq 1 4); do
    (
      for i in $(seq 1 50); do
        "$MCCHAIND" tx bank send validator0 "$to_addr" 1000umc \
          --chain-id mcchain-dev-1 --keyring-backend test \
          --home "$REPO_ROOT/.devnet/node0" --node "$RPC" \
          --gas auto --gas-adjustment 1.5 --fees 1000umc -y >/dev/null 2>&1
      done
    ) &
  done
  wait
  log "  200 笔全部广播完毕"
}

main() {
  case "${1:-all}" in
    b1) b1 ;;
    b2) b2 ;;
    b3) b3 ;;
    b4) b4 ;;
    b5) b5 ;;
    all)
      wait_ready
      b1; b2; b3; b4; b5
      ;;
    --help|-h)
      sed -n '4,15p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
      ;;
    *) die "未知子命令: $1" ;;
  esac
}

main "$@"