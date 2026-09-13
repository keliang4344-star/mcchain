#!/usr/bin/env bash
#
# MC 公链 · 本地开发网（devnet）一键脚本
#
# 为什么需要它：生态从 0 到 1 的前提是「外部开发者能在几分钟内跑起一条链」。
# 在此之前唯一能起链的路径是生产向的 deploy/init.sh —— 它面向真实主网、
# 要求配置真实核心账户、且只起单节点，无法用于开发迭代。
#
# 用法：
#   ./scripts/devnet.sh init [N]        初始化 N 个验证人（默认读 VALIDATORS，3）
#   ./scripts/devnet.sh start           启动全部节点
#   ./scripts/devnet.sh stop            停止全部节点
#   ./scripts/devnet.sh status          查看高度、端点、账户
#   ./scripts/devnet.sh fund A 1000000  给地址发 1 MC（金额单位 umc；仅 devnet）
#   ./scripts/devnet.sh clean           停止并删除全部 devnet 数据
#
# 环境变量：
#   DEVNET_HOME     数据根目录（默认 <仓库根>/.devnet）
#   DEVNET_CHAINID  链 ID（默认 mcchain-dev-1）
#   MCCHAIND        mcchaind 二进制路径（默认自动查找/构建）
#   VALIDATORS      验证人数量（默认 3）
#   BLOCK_TIME      出块间隔（默认 4s，与主网一致；调小可加速开发迭代）
#   FUND_AMOUNT     每个验证人账户的初始余额（默认 100000000000000umc = 1 亿 MC）
#
# 平台说明：Linux / macOS 与 Windows(git-bash) 均可直接运行。
# Windows 下脚本会自动把 /e/... 转成 E:/... 再交给原生 mcchaind.exe —— 见 win()。
#
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEVNET_HOME="${DEVNET_HOME:-$REPO_ROOT/.devnet}"
CHAIN_ID="${DEVNET_CHAINID:-mcchain-dev-1}"
DENOM="${DEVNET_DENOM:-umc}"
VALIDATORS="${VALIDATORS:-3}"
BLOCK_TIME="${BLOCK_TIME:-4s}"
# FUND_AMOUNT 必须 >= STAKE_AMOUNT，否则 gentx 会因余额不足失败。
#
# ⚠️ devnet-only：这部分钱是 add-genesis-account 凭空记的账，**不在五池之内**，
#    因此 devnet 的 bank 总供应 = 1e15（硬顶）+ FUND_AMOUNT×N，会超过硬顶
#    （app.assertGenesisSupplyCap 在非生产链上只告警不拦截）。
#    主网创世**绝不能**照抄：所有创世账户余额必须来自五池分配，
#    否则「总量 10 亿 MC 恒定」当场失效且不可逆。
FUND_AMOUNT="${FUND_AMOUNT:-100000000000000${DENOM}}"   # 100,000,000 MC
# 每个验证人自质押 1,000,000 MC —— devnet 不校验 30000 MC 的生产门槛，
# 目的是让本地任意规模的验证人集合都能立刻出块。
STAKE_AMOUNT="${STAKE_AMOUNT:-1000000000000${DENOM}}"   # 1,000,000 MC
# gentx 的 min_self_delegation：链上 ante 装饰器强制下限 30,000 MC
# （app/ante.go MinSelfDelegationLowerBound）。devnet 虽不校验生产门槛，
# 但该装饰器在 InitChain 投递 gentx 时同样执行，不显式传就会起不来。
# 注意：SDK 的 gentx 该旗标只接受纯整数（base unit），不接受 "30000000000umc" 这种
# coin 串（会报 minimum self delegation must be a positive integer）。
MIN_SELF_DELEGATION="${MIN_SELF_DELEGATION:-30000000000}"  # 30,000 MC（单位 umc）

# 端口基址，第 i 个节点使用 base + i*10
PORT_STEP=10
BASE_RPC=26657
BASE_P2P=26656
BASE_GRPC=9090
BASE_API=1317

log()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[!]\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31m[FATAL]\033[0m %s\n' "$*" >&2; exit 1; }

node_home()  { echo "$DEVNET_HOME/node$1"; }
port_of()    { echo $(( $1 + $2 * PORT_STEP )); }
rpc_of()     { port_of "$BASE_RPC" "$1"; }
p2p_of()     { port_of "$BASE_P2P" "$1"; }
grpc_of()    { port_of "$BASE_GRPC" "$1"; }
api_of()     { port_of "$BASE_API" "$1"; }

need_python() {
  command -v python3 >/dev/null 2>&1 || die "需要 python3（用于生成与修补 genesis / config）"
}

# ---------------------------------------------------------------------------
# 原生路径转换（Windows / git-bash 专用，Linux 上是恒等函数）
#
# 为什么必须有它：mcchaind / python3 在 Windows 上是**原生 PE 程序**，而
# git-bash 传参时并不总会把 MSYS 路径（/e/foo/bar）转成 Windows 路径。
# 一旦没转，原生程序会把 "/e/foo/bar" 当成「相对当前盘的路径」，
# 结果是 init 明明退出码 0、却什么都没写下来 —— 极难排查的静默失败。
# 所以凡是交给原生程序的**文件系统路径**，统一先过 win()。
# ---------------------------------------------------------------------------
win() {
  if command -v cygpath >/dev/null 2>&1; then
    cygpath -w "$1"
  else
    printf '%s' "$1"   # Linux/macOS：原样返回
  fi
}

resolve_bin() {
  if [[ -n "${MCCHAIND:-}" ]]; then
    [[ -x "$MCCHAIND" ]] || die "MCCHAIND=$MCCHAIND 不存在或不可执行"
    return
  fi
  local candidate="$REPO_ROOT/build/mcchaind"
  if [[ ! -x "$candidate" ]]; then
    log "未找到 $candidate，开始构建 mcchaind"
    mkdir -p "$REPO_ROOT/build"
    ( cd "$REPO_ROOT" && go build -o build/mcchaind ./cmd/mcchaind )
  fi
  MCCHAIND="$candidate"
}

# MC：代替直接调用 "$MCCHAIND"。唯一职责是把 --home 的值转成原生路径，
# 其余参数原样透传（URL 等不能被转换，所以只认 --home）。
MC() {
  local -a argv=()
  while (( $# )); do
    if [[ "$1" == "--home" && $# -ge 2 ]]; then
      argv+=("--home" "$(win "$2")")
      shift 2
    else
      argv+=("$1")
      shift
    fi
  done
  "$MCCHAIND" "${argv[@]}"
}

# ---------------------------------------------------------------------------
# genesis 修补：devnet 与主网共用同一套 denom 口径，避免「devnet 能跑、
# 主网参数不同」造成的隐性差异。
# ---------------------------------------------------------------------------
patch_genesis() {
  local genesis="$1"
  python3 - "$(win "$genesis")" "$DENOM" "$BLOCK_TIME" <<'PY'
import json, sys

path, denom, block_time = sys.argv[1], sys.argv[2], sys.argv[3]
g = json.load(open(path, encoding="utf-8"))
as_ = g["app_state"]

# 1) denom 统一：staking / mint / crisis / gov 的默认 stake → umc
as_["staking"]["params"]["bond_denom"] = denom
as_["mint"]["params"]["mint_denom"] = denom
cf = (as_.get("crisis") or {}).get("constant_fee")
if cf:
    cf["denom"] = denom
for d in ((as_.get("gov") or {}).get("params") or {}).get("min_deposit", []) or []:
    if d.get("denom") == "stake":
        d["denom"] = denom

# 2) 零通胀：devnet 也必须与主网同口径，否则本地跑不出真实的发行行为
ZERO = "0.000000000000000000"
mp = as_["mint"]["params"]
for k in ("inflation_rate_change", "inflation_max", "inflation_min"):
    if k in mp:
        mp[k] = ZERO
if "minter" in as_["mint"]:
    as_["mint"]["minter"]["inflation"] = ZERO
    as_["mint"]["minter"]["annual_provisions"] = ZERO

# 3) 每区块 gas 上限显式化（与主网硬顶一致，杜绝默认 -1 的「不限」）
blk = g.setdefault("consensus_params", {}).setdefault("block", {})
blk["max_gas"] = "100000000"

# 4) 出块间隔
g.setdefault("consensus_params", {}).setdefault("evidence", {})
g["consensus_params"]["timeout_commit"] = block_time

# 5) slashing / staking 与主网同口径校准（scripts/make_genesis.py 的同一套约束）。
#    意义：devnet 若沿用 SDK 默认（窗口 100 块 = 400s、jail 600s），
#    就复现不了「窗口 86400s / jail 600s」下的停机判定行为，
#    集成期发现不了主网参数才会暴露的问题。
sp = as_.setdefault("slashing", {}).setdefault("params", {})
sp.update({
    "signed_blocks_window": "21600",          # 21600 块 × 4s = 24h
    "min_signed_per_window": "0.5",
    "downtime_jail_duration": "600s",
    "slash_fraction_downtime": "0.01",
    "slash_fraction_double_sign": "0.05",
})
tp = as_.setdefault("staking", {}).setdefault("params", {})
tp.update({
    "unbonding_time": "1814400s",             # 21 天
    "max_validators": 100,
    "max_entries": 7,
    "historical_entries": 10000,
})

# 6) 模块 params 兜底：部分模块空 params 会被序列化成 null，
#    链启动时 InitGenesis 解析失败。统一补成空对象。
#
# ⚠️ 只能补给「GenesisState 里确实有 params 字段」的模块。
#    tokenomics 的 GenesisState **没有** params 字段
#    （参数走 types.DefaultParams() 硬编码，不进创世），给它塞一个 "params": {}
#    会让 InitGenesis 的 MustUnmarshalJSON 直接 panic：
#        unknown field "params" in types.GenesisState
#    而 validate-genesis 用 encoding/json（宽松）会报 valid，
#    因此建链必须走与链上同口径的创世校验（见 tokenomics/module.go）。
for mod in ("phonenode", "edgeai", "depin", "referral", "dex", "mcchain"):
    if mod in as_ and as_[mod] is None:
        as_[mod] = {}
    if mod in as_ and isinstance(as_[mod], dict) and not as_[mod].get("params"):
        as_[mod]["params"] = as_[mod].get("params") or {}

json.dump(g, open(path, "w", encoding="utf-8"), indent=2, ensure_ascii=False)
print(f"    genesis patched: denom={denom} block_time={block_time} "
      f"slashing_window=21600 jail=600s")
PY
}

# ---------------------------------------------------------------------------
# config.toml / app.toml 修补：端口、持久对等、出块间隔、pruning、API 开关
# ---------------------------------------------------------------------------
patch_node_config() {
  local home="$1" idx="$2" peers="$3"
  python3 - "$(win "$home")" "$idx" "$peers" "$BLOCK_TIME" \
      "$(rpc_of "$idx")" "$(p2p_of "$idx")" "$(grpc_of "$idx")" "$(api_of "$idx")" <<'PY'
import re, sys

home, idx, peers, block_time, rpc, p2p, grpc, api = sys.argv[1:9]

def sub(text, pattern, repl, required=True, label=""):
    new, n = re.subn(pattern, repl, text, count=1, flags=re.M)
    if required and n == 0:
        raise SystemExit(f"[FATAL] 未能在配置中找到 {label or pattern}")
    return new

cfg_path = f"{home}/config/config.toml"
cfg = open(cfg_path, encoding="utf-8").read()
cfg = sub(cfg, r'^moniker\s*=.*$', f'moniker = "devnet-node{idx}"', label="moniker")
cfg = sub(cfg, r'^laddr\s*=\s*"tcp://127\.0\.0\.1:26657"$', f'laddr = "tcp://127.0.0.1:{rpc}"', label="rpc laddr")
cfg = sub(cfg, r'^laddr\s*=\s*"tcp://0\.0\.0\.0:26656"$', f'laddr = "tcp://0.0.0.0:{p2p}"', label="p2p laddr")
cfg = sub(cfg, r'^persistent_peers\s*=.*$', f'persistent_peers = "{peers}"', label="persistent_peers")
cfg = sub(cfg, r'^timeout_commit\s*=.*$', f'timeout_commit = "{block_time}"', label="timeout_commit")
cfg = sub(cfg, r'^addr_book_strict\s*=.*$', 'addr_book_strict = false', required=False, label="addr_book_strict")
cfg = sub(cfg, r'^prometheus\s*=.*$', 'prometheus = true', required=False, label="prometheus")
open(cfg_path, "w", encoding="utf-8").write(cfg)

app_path = f"{home}/config/app.toml"
app = open(app_path, encoding="utf-8").read()
app = sub(app, r'^minimum-gas-prices\s*=.*$', 'minimum-gas-prices = "0umc"', label="minimum-gas-prices")
# devnet 用 every 档：节点重启后无需从创世重放，迭代更快。
app = sub(app, r'^pruning\s*=.*$', 'pruning = "nothing"', label="pruning")
app = sub(app, r'^pruning-keep-recent\s*=.*$', 'pruning-keep-recent = "0"', required=False, label="keep-recent")
app = sub(app, r'^pruning-interval\s*=.*$', 'pruning-interval = "0"', required=False, label="interval")
app = sub(app, r'^enable\s*=\s*false$', 'enable = true', label="api enable")
app = sub(app, r'^address\s*=\s*"tcp://localhost:1317"$', f'address = "tcp://0.0.0.0:{api}"', label="api address")
app = sub(app, r'^address\s*=\s*"localhost:9090"$', f'address = "0.0.0.0:{grpc}"', label="grpc address")
app = sub(app, r'^enable\s*=\s*true$', 'enable = true', required=False, label="grpc enable")
open(app_path, "w", encoding="utf-8").write(app)
print(f"    node{idx} config: rpc={rpc} p2p={p2p} grpc={grpc} api={api}")
PY
}

install_pruning_note() {
  # 在 app.toml 的 pruning 旁留下出处注释，避免运维把它当成随手写的值。
  local app_path="$1/config/app.toml"
  python3 - "$(win "$app_path")" <<'PY'
import sys
p = sys.argv[1]
t = open(p, encoding="utf-8").read()
mark = 'pruning = "nothing"'
note = ('# devnet 档：保留全部历史状态，便于反复调试与链上状态检查（见 docs/NODE_CONFIG.md）\n'
        'pruning = "nothing"')
if 'devnet 档' not in t and mark in t:
    t = t.replace(mark, note, 1)
    open(p, "w", encoding="utf-8").write(t)
PY
}

# ---------------------------------------------------------------------------
# init
# ---------------------------------------------------------------------------
cmd_init() {
  local n="${1:-$VALIDATORS}"
  need_python
  resolve_bin

  case "$n" in ''|*[!0-9]*) die "验证人数量必须是数字，收到: $n";; esac
  (( n >= 1 && n <= 20 )) || die "验证人数量应在 1..20，收到: $n"

  [[ -d "$DEVNET_HOME" ]] && die "$DEVNET_HOME 已存在。先运行 ./scripts/devnet.sh clean，或换 DEVNET_HOME。"

  log "初始化 $n 个验证人（chain-id=$CHAIN_ID home=$DEVNET_HOME）"
  VALIDATORS="$n"
  mkdir -p "$DEVNET_HOME"

  # [1] 逐个 init，并生成 key
  local i
  for (( i = 0; i < n; i++ )); do
    local home; home="$(node_home "$i")"
    mkdir -p "$home"
    MC init "devnet-node$i" --chain-id "$CHAIN_ID" --home "$home" >/dev/null 2>&1
    # init 必须落盘才算成功：git-bash 下若路径没被正确转换，init 会静默退出 0
    # 却什么都不写，直到若干步之后才以 "cp: cannot stat ..." 的形式暴露。
    # 这里提前fail-fast，把错误定位在最贴近根因的地方。
    [[ -f "$home/config/genesis.json" ]] \
      || die "node$i 初始化失败：$home/config/genesis.json 未生成（检查 --home 路径）"
    # 固定 keyring-backend=test：devnet 的助记词本就公开，无需安全后端，
    # 且避免非交互环境下 prompt 卡住。
    MC keys add "validator$i" --keyring-backend test --home "$home" >/dev/null 2>&1
    log "  node$i 密钥已生成"
  done

  # [2] 用 node0 的 genesis 作为模板，加账户
  local g0; g0="$(node_home 0)/config/genesis.json"
  for (( i = 0; i < n; i++ )); do
    local home addr; home="$(node_home "$i")"
    addr="$(MC keys show "validator$i" -a --keyring-backend test --home "$home")"
    MC add-genesis-account "$addr" "$FUND_AMOUNT" --home "$(node_home 0)" >/dev/null
    printf '%s\n' "$addr" > "$home/.address"
    log "  node$i 账户 $addr 余额 $FUND_AMOUNT"
  done

  # [3] 修补 genesis（denom / 零通胀 / gas 上限 / slashing / params 兜底）
  patch_genesis "$g0"

  # [4] 各节点生成 gentx（先把已加好账户的 genesis 分发下去）
  local peers=""
  for (( i = 0; i < n; i++ )); do
    peers+="$(MC comet show-node-id --home "$(node_home "$i")")@127.0.0.1:$(p2p_of "$i"),"
  done
  peers="${peers%,}"

  for (( i = 0; i < n; i++ )); do
    local home; home="$(node_home "$i")"
    # node0 的 genesis 就是模板本身，不能再 cp 到自己头上（cp 会报 same file）
    [[ "$home/config/genesis.json" == "$g0" ]] || cp "$g0" "$home/config/genesis.json"
    # 必须显式给 --min-self-delegation。
    #
    # 链上的 MinSelfDelegationDecorator（app/ante.go，下限 30k MC）在 genutil 投递
    # gentx 时同样会执行（走 DeliverTx）。不传该参数时 SDK 默认写 "1"，于是：
    #     panic: ... min self delegation 1 < lower bound 30000000000 umc
    # 这在主网 gentx 收集阶段是同一个坑：任何一份 min_self_delegation 不达标的
    # gentx 都会让整条链在 InitChain 起不来，且没有任何前置检查会报出来
    # （validate-genesis 不校验 gentx 内容）。因此 devnet 与主网都必须显式传。
    MC gentx "validator$i" "$STAKE_AMOUNT" \
      --min-self-delegation "$MIN_SELF_DELEGATION" \
      --chain-id "$CHAIN_ID" --keyring-backend test --home "$home" >/dev/null 2>&1 \
      || die "node$i gentx 生成失败（常见原因：min-self-delegation 未达链上下限 $MIN_SELF_DELEGATION）"
  done

  # [5] 收集 gentx 并分发给所有节点
  log "收集 gentx 并分发最终 genesis"
  mkdir -p "$(node_home 0)/config/gentx"
  for (( i = 1; i < n; i++ )); do
    cp "$(node_home "$i")/config/gentx/"*.json "$(node_home 0)/config/gentx/" 2>/dev/null || true
  done
  MC collect-gentxs --home "$(node_home 0)" >/dev/null 2>&1
  MC validate-genesis --home "$(node_home 0)" >/dev/null
  [[ -d "$(node_home 0)/config/gentx" ]] && rm -rf "$(node_home 0)/config/gentx"

  for (( i = 1; i < n; i++ )); do
    cp "$(node_home 0)/config/genesis.json" "$(node_home "$i")/config/genesis.json"
  done

  # [6] 修补每个节点的 config
  for (( i = 0; i < n; i++ )); do
    patch_node_config "$(node_home "$i")" "$i" "$peers"
    install_pruning_note "$(node_home "$i")"
  done

  log "devnet 初始化完成：$n 个验证人"
  cmd_status
}

# ---------------------------------------------------------------------------
# start / stop
# ---------------------------------------------------------------------------
cmd_start() {
  [[ -d "$DEVNET_HOME" ]] || die "未找到 $DEVNET_HOME，先运行 ./scripts/devnet.sh init"
  resolve_bin
  local n="${VALIDATORS:-0}"
  # 从已有目录推断实际节点数，避免 VALIDATORS 与环境不一致时漏启节点。
  n=0
  while [[ -d "$(node_home "$n")" ]]; do n=$((n + 1)); done
  (( n > 0 )) || die "$DEVNET_HOME 下没有节点目录"

  mkdir -p "$DEVNET_HOME/logs"
  for (( i = 0; i < n; i++ )); do
    local home logf; home="$(node_home "$i")"; logf="$DEVNET_HOME/logs/node$i.log"
    if [[ -f "$home/.pid" ]] && kill -0 "$(cat "$home/.pid")" 2>/dev/null; then
      warn "node$i 已在运行（pid $(cat "$home/.pid")），跳过"
      continue
    fi
    nohup env MC_ORACLE_ALLOW_SOFT=1 \
        "$MCCHAIND" start --home "$(win "$home")" >"$logf" 2>&1 &
    echo $! > "$home/.pid"
    log "node$i 已启动 (pid $!) → $logf"
  done
  log "等待出块…"
  sleep 6
  cmd_status
}

cmd_stop() {
  [[ -d "$DEVNET_HOME" ]] || die "未找到 $DEVNET_HOME"
  local i=0
  while [[ -d "$(node_home "$i")" ]]; do
    local home; home="$(node_home "$i")"
    if [[ -f "$home/.pid" ]]; then
      local pid; pid="$(cat "$home/.pid")"
      if kill -0 "$pid" 2>/dev/null; then
        kill "$pid" 2>/dev/null || true
        log "node$i 已停止 (pid $pid)"
      fi
      rm -f "$home/.pid"
    fi
    i=$((i + 1))
  done
}

cmd_status() {
  [[ -d "$DEVNET_HOME" ]] || die "未找到 $DEVNET_HOME"
  resolve_bin
  printf '\n%-8s %-30s %-10s %s\n' "NODE" "RPC" "HEIGHT" "STATUS"
  local i=0
  while [[ -d "$(node_home "$i")" ]]; do
    local rpc height status
    rpc="http://127.0.0.1:$(rpc_of "$i")"
    height="$("$MCCHAIND" status --node "$rpc" 2>/dev/null \
      | python3 -c 'import json,sys; print(json.load(sys.stdin)["sync_info"]["latest_block_height"])' 2>/dev/null || echo "-")"
    if [[ "$height" == "-" ]]; then status="未响应"; else status="出块中"; fi
    printf '%-8s %-30s %-10s %s\n' "node$i" "$rpc" "$height" "$status"
    i=$((i + 1))
  done

  printf '\n%-8s %-16s %-16s %-12s\n' "NODE" "gRPC" "REST" "P2P"
  for (( k = 0; k < i; k++ )); do
    printf '%-8s %-16s %-16s %-12s\n' "node$k" "127.0.0.1:$(grpc_of "$k")" \
      "http://127.0.0.1:$(api_of "$k")" "127.0.0.1:$(p2p_of "$k")"
  done

  if [[ -f "$(node_home 0)/.address" ]]; then
    printf '\n%s\n' "验证人账户（可直接用于 faucet / 转账测试）："
    for (( k = 0; k < i; k++ )); do
      [[ -f "$(node_home "$k")/.address" ]] && printf '  node%s: %s\n' "$k" "$(cat "$(node_home "$k")/.address")"
    done
  fi
  printf '\n链 ID: %s   出块: %s\n' "$CHAIN_ID" "$BLOCK_TIME"
}

# ---------------------------------------------------------------------------
# fund：从 node0 转币（仅本地开发用；测试网请用 faucet 服务）
# ---------------------------------------------------------------------------
cmd_fund() {
  local to="${1:-}" amount="${2:-10000000000}"
  [[ -n "$to" ]] || die "用法: ./scripts/devnet.sh fund <地址> [金额umc]"
  resolve_bin
  MC tx bank send "validator0" "$to" "${amount}${DENOM}" \
    --keyring-backend test --home "$(node_home 0)" \
    --chain-id "$CHAIN_ID" --node "http://127.0.0.1:$(rpc_of 0)" \
    --gas auto --gas-adjustment 1.4 --fees 1000"${DENOM}" -y
}

cmd_clean() {
  [[ -d "$DEVNET_HOME" ]] || { log "无 $DEVNET_HOME，无需清理"; return; }
  cmd_stop || true
  # 仅删除本脚本自己创建的目录，路径由 DEVNET_HOME 明确指定，不做通配删除。
  rm -rf "$DEVNET_HOME"
  log "已删除 $DEVNET_HOME"
}

usage() {
  sed -n '2,26p' "${BASH_SOURCE[0]}" | sed 's/^#\{1,2\} \{0,1\}//'
  exit 1
}

main() {
  local cmd="${1:-}"; shift || true
  case "$cmd" in
    init)   cmd_init "$@";;
    start)  cmd_start "$@";;
    stop)   cmd_stop "$@";;
    status) cmd_status "$@";;
    fund)   cmd_fund "$@";;
    clean)  cmd_clean "$@";;
    *)      usage;;
  esac
}

main "$@"
