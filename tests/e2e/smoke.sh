#!/usr/bin/env bash
#
# MCChain E2E 集成冒烟测试（tests/e2e/smoke.sh）
#
# 用真实二进制把 devnet 从 0 跑到出块，覆盖：
#   [1] 链能初始化（init N 个验证人 + 真实 gentx 校验）
#   [2] 链能启动并出块（status height 单调增长）
#   [3] 账号余额可查（CLI bank balances == 初始资金）
#   [4] 质押验证人可查（staking validators >= N）
#   [5] REST 1317 可用（/cosmos/bank/v1beta1/balances 返回 200 且余额一致）
#   [6] gRPC 9090 可用（mcchaind query --grpc-addr 走 gRPC 查账号）
#   [7] 转账冒烟（fund 目标地址 -> tx bank send -> 目标余额增加）
#
# 环境变量（与 scripts/devnet.sh 同族）：
#   MCCHAIND       mcchaind 二进制路径（默认自动构建到 build/mcchaind）
#   VALIDATORS     验证人数（默认 1，冒烟加速）
#   BLOCK_TIME     出块间隔（默认 2s，冒烟加速；主网为 4s）
#   E2E_HOME       devnet 数据根目录（默认 <仓库根>/.e2e-devnet，独立于开发网）
#   SKIP_CLEAN     非空则跑完后保留数据目录（默认清理）
#
# 用法：bash tests/e2e/smoke.sh
# 退出码：0=全部断言通过；1=任一断言失败（脚本立即 fail-fast）。
#
set -euo pipefail

# 与 scripts/devnet.sh 一致：禁用 git-bash/MSYS 的路径自动转换，
# 否则形如 E:\... 的参数会被转回 /e/...，原生 mcchaind 静默失败。
export MSYS2_ARG_CONV_EXCL='*'
export MSYS_NO_PATHCONV=1

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DEVNET_SH="$REPO_ROOT/scripts/devnet.sh"
E2E_HOME="${E2E_HOME:-$REPO_ROOT/.e2e-devnet}"
VALIDATORS="${VALIDATORS:-1}"
BLOCK_TIME="${BLOCK_TIME:-2s}"
SKIP_CLEAN="${SKIP_CLEAN:-}"

log()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
ok()   { printf '\033[1;32m[PASS]\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31m[FAIL]\033[0m %s\n' "$*" >&2; exit 1; }

# 依赖 python3（devnet.sh 修补 genesis/config 需要；CI 镜像与 git-bash 均有）
command -v python3 >/dev/null 2>&1 || die "需要 python3"

[[ -f "$DEVNET_SH" ]] || die "缺少 scripts/devnet.sh"

# devnet.sh 在 core.autocrlf=true 的 Windows 工作区会被检出为 CRLF，
# bash 直接执行会因 `set -euo pipefail\r` 报 "set: pipefail"。
# 这里检测到 CRLF 时生成 LF 拷贝再执行；CI（LF checkout）无需拷贝。
# 拷贝放到仓库根的 .e2e-tmp（独立于 E2E_HOME，避免被 e2e 数据清理连带删除）。
DEVNET_SH_RUN="$DEVNET_SH"
if grep -q $'\r' "$DEVNET_SH"; then
  DEVNET_SH_RUN="$REPO_ROOT/.e2e-tmp/devnet.sh"
  mkdir -p "$(dirname "$DEVNET_SH_RUN")"
  tr -d '\r' < "$DEVNET_SH" > "$DEVNET_SH_RUN"
  chmod +x "$DEVNET_SH_RUN" 2>/dev/null || true
  log "devnet.sh 为 CRLF，已生成 LF 拷贝：$DEVNET_SH_RUN"
fi

# ---------------------------------------------------------------------------
# 0) 构建 mcchaind（未显式指定时）
# ---------------------------------------------------------------------------
if [[ -z "${MCCHAIND:-}" ]]; then
  # 与 scripts/devnet.sh resolve_bin 对齐：Windows(git-bash) 用 .exe，Linux 无后缀
  MCCHAIND="$REPO_ROOT/build/mcchaind"
  if [[ "$(uname -s)" == MINGW* || "$(uname -s)" == MSYS* ]]; then
    MCCHAIND="$REPO_ROOT/build/mcchaind.exe"
  fi
  if [[ ! -f "$MCCHAIND" ]]; then
    log "构建 mcchaind -> $MCCHAIND"
    mkdir -p "$REPO_ROOT/build"
    ( cd "$REPO_ROOT" && go build -o "$MCCHAIND" ./cmd/mcchaind )
  fi
fi

# ---------------------------------------------------------------------------
# 1) 准备干净的 e2e 数据目录
# ---------------------------------------------------------------------------
log "清理旧 e2e 数据：$E2E_HOME"
if [[ -d "$E2E_HOME" ]]; then
  bash "$DEVNET_SH_RUN" stop >/dev/null 2>&1 || true
  rm -rf "$E2E_HOME"
fi

# ---------------------------------------------------------------------------
# 2) init / start
# ---------------------------------------------------------------------------
log "devnet init：VALIDATORS=$VALIDATORS BLOCK_TIME=$BLOCK_TIME"
DEVNET_HOME="$E2E_HOME" DEVNET_CHAINID="mcchain-dev-1" \
  VALIDATORS="$VALIDATORS" BLOCK_TIME="$BLOCK_TIME" MCCHAIND="$MCCHAIND" \
  bash "$DEVNET_SH_RUN" init "$VALIDATORS" >/dev/null

log "devnet start"
DEVNET_HOME="$E2E_HOME" BLOCK_TIME="$BLOCK_TIME" MCCHAIND="$MCCHAIND" \
  bash "$DEVNET_SH_RUN" start >/dev/null

# ---------------------------------------------------------------------------
# 3) 等待出块（height 单调增长到 > 0）
# ---------------------------------------------------------------------------
log "等待出块（RPC 26657）"
CHAIN_ID="mcchain-dev-1"
RPC="http://127.0.0.1:26657"
wait_height() { # 轮询直到 height >= 目标
  local target="$1" h=0 tries=0
  while (( tries < 60 )); do
    h=$(curl -s "$RPC/status" | python3 -c 'import sys,json;print(json.load(sys.stdin)["result"]["sync_info"]["latest_block_height"])' 2>/dev/null || echo 0)
    if (( h >= target )); then echo "$h"; return 0; fi
    sleep 1; tries=$((tries + 1))
  done
  echo "$h"; return 1
}
H1=$(wait_height 2) || die "60s 内未出块（height=$H1）；检查 .e2e-devnet/logs/*.log"
ok "链已出块，height=$H1"

# ---------------------------------------------------------------------------
# 4) 断言 1：status.chain_id == mcchain-dev-1
# ---------------------------------------------------------------------------
GOT_ID=$(curl -s "$RPC/status" | python3 -c 'import sys,json;print(json.load(sys.stdin)["result"]["node_info"]["network"])')
[[ "$GOT_ID" == "$CHAIN_ID" ]] || die "chain_id=$GOT_ID != $CHAIN_ID"
ok "chain_id=$CHAIN_ID"

# ---------------------------------------------------------------------------
# 5) 断言 2：CLI bank balances == 初始资金（扣质押后）
# ---------------------------------------------------------------------------
NODE0_HOME="$E2E_HOME/node0"
ADDR0=$(cat "$NODE0_HOME/.address")
FUND="100000000000000umc"   # devnet.sh 默认 FUND_AMOUNT
STAKE="1000000000000umc"    # devnet.sh 默认 STAKE_AMOUNT（质押后余额 = FUND - STAKE）
EXPECT_BAL="99000000000000"
BAL0=$("$MCCHAIND" q bank balances "$ADDR0" --node "$RPC" --chain-id "$CHAIN_ID" -o json \
        | python3 -c 'import sys,json
d=json.load(sys.stdin)["balances"]
print(next((c["amount"] for c in d if c["denom"]=="umc"), "0"))')
[[ "$BAL0" == "$EXPECT_BAL" ]] || die "node0 余额=$BAL0 != $EXPECT_BAL（FUND $FUND - STAKE $STAKE）"
ok "CLI 余额查询：node0=$BAL0 umc（FUND-STAKE）"

# ---------------------------------------------------------------------------
# 6) 断言 3：staking validators >= N
# ---------------------------------------------------------------------------
VNUM=$("$MCCHAIND" q staking validators --node "$RPC" --chain-id "$CHAIN_ID" -o json \
        | python3 -c 'import sys,json;print(len(json.load(sys.stdin)["validators"]))')
(( VNUM >= VALIDATORS )) || die "验证人数=$VNUM < $VALIDATORS"
ok "staking validators=$VNUM"

# ---------------------------------------------------------------------------
# 7) 断言 4：REST 1317 可用且余额与 CLI 一致
# ---------------------------------------------------------------------------
REST="http://127.0.0.1:1317"
# REST API server 与 RPC 启动时序不同，先轮询到返回合法 balances 再断言
REST_BAL=""
tries=0
while (( tries < 30 )); do
  REST_BAL=$(curl -s "$REST/cosmos/bank/v1beta1/balances/$ADDR0" \
          | python3 -c 'import sys,json
try:
    d=json.load(sys.stdin)["balances"]
    print(next((c["amount"] for c in d if c["denom"]=="umc"), ""))
except Exception:
    print("")' 2>/dev/null || echo "")
  [[ -n "$REST_BAL" ]] && break
  sleep 1; tries=$((tries + 1))
done
[[ "$REST_BAL" == "$BAL0" ]] || die "REST 余额=$REST_BAL != CLI $BAL0"
ok "REST 1317：/cosmos/bank/v1beta1/balances 返回 $REST_BAL umc（与 CLI 一致）"

# ---------------------------------------------------------------------------
# 8) 断言 5：gRPC 9090 可用（--grpc-addr 查询账号）
# ---------------------------------------------------------------------------
GRPC_BAL=""
tries=0
while (( tries < 30 )); do
  GRPC_BAL=$("$MCCHAIND" q bank balances "$ADDR0" --grpc-addr 127.0.0.1:9090 --grpc-insecure \
          --chain-id "$CHAIN_ID" -o json \
          | python3 -c 'import sys,json
try:
    d=json.load(sys.stdin)["balances"]
    print(next((c["amount"] for c in d if c["denom"]=="umc"), ""))
except Exception:
    print("")' 2>/dev/null || echo "")
  [[ -n "$GRPC_BAL" ]] && break
  sleep 1; tries=$((tries + 1))
done
[[ "$GRPC_BAL" == "$BAL0" ]] || die "gRPC 余额=$GRPC_BAL != CLI $BAL0"
ok "gRPC 9090：查询返回 $GRPC_BAL umc（与 CLI 一致）"

# ---------------------------------------------------------------------------
# 9) 断言 6：转账冒烟（fund 目标地址 -> tx -> 余额增加）
# ---------------------------------------------------------------------------
log "转账冒烟：fund 新地址并 tx bank send"
SEND_ADDR=$("$MCCHAIND" keys add e2e-sink --keyring-backend test --home "$NODE0_HOME" \
        --output json | python3 -c 'import sys,json;print(json.load(sys.stdin)["address"])')
"$MCCHAIND" tx bank send "$ADDR0" "$SEND_ADDR" 1000000umc \
        --from validator0 --keyring-backend test --home "$NODE0_HOME" \
        --chain-id "$CHAIN_ID" --node "$RPC" --gas auto --gas-adjustment 1.5 \
        --yes >/dev/null

H2=$(wait_height "$(( H1 + 2 ))") || die "转账后未继续出块（height=$H2）"
SINK_BAL=$("$MCCHAIND" q bank balances "$SEND_ADDR" --node "$RPC" --chain-id "$CHAIN_ID" -o json \
        | python3 -c 'import sys,json
d=json.load(sys.stdin)["balances"]
print(next((c["amount"] for c in d if c["denom"]=="umc"), "0"))')
[[ "$SINK_BAL" == "1000000" ]] || die "收款地址余额=$SINK_BAL != 1000000"
ok "转账冒烟：$ADDR0 -> $SEND_ADDR 1000000umc，收款余额=$SINK_BAL"

# ---------------------------------------------------------------------------
# 10) 收尾
# ---------------------------------------------------------------------------
log "devnet stop"
DEVNET_HOME="$E2E_HOME" MCCHAIND="$MCCHAIND" bash "$DEVNET_SH_RUN" stop >/dev/null 2>&1 || true

if [[ -z "$SKIP_CLEAN" ]]; then
  rm -rf "$E2E_HOME"
  log "已清理 e2e 数据目录"
else
  log "SKIP_CLEAN 已设，保留 $E2E_HOME 供排查"
fi

log "E2E 冒烟测试全部通过"
