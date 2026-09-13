#!/usr/bin/env bash
#
# MC 公链 · 主网创世生成器（团队 3-of-5 多签作创世验证人）
# ============================================================================
# 本脚本是主网创世的唯一路径（已通过端到端验证）。
#
# 核心事实：
#   1. 主网创世**绝不使用 add-genesis-account 给任何账户留余额**。
#      全链 1e15 umc 由 tokenomics 模块在 InitGenesis 一次性铸造并拨入五池；
#      genesis.json 的 bank.balances 必须为空。任何凭空记账的创世余额都会
#      击穿「总量 10 亿 MC 恒定」，链上供应上限校验会在生产链上拒绝启动。
#   2. 验证人的自抵押资金来自五池中的**团队池**：tokenomics 在创世时把
#      1.2e14 拨给团队多签地址并建为 1yr cliff + 3yr 线性的 ContinuousVestingAccount。
#      InitGenesis 顺序为 … → tokenomics → genutil → …，所以 genutil 投递
#      gentx 时该账户已有余额。
#   3. vesting 锁定**不阻塞**抵押：SDK v0.47 的 bank.DelegateCoins 只校验原始余额，
#      锁定与已解锁部分由 trackDelegation 分别记账（x/bank/keeper/keeper.go:142）。
#   4. gentx 命令行的 ValidateAccountInGenesis 只读 genesis.json 的 bank.balances，
#      因此必须先**临时**给团队地址记账 → 生成 gentx → 再删掉。
#      这是本脚本第 4/8 步存在的原因，不是冗余步骤。
#   5. 签名用 --account-number 0 是**正确**的：SDK 在 ctx.BlockHeight()==0 时
#      强制用 AccountNumber=0 参与验签（x/auth/ante/sigverify.go），
#      因为创世期账户编号尚未定型。不要改成"实际账号编号"，那会验签失败。
#
# 用法：
#   ./scripts/gen_genesis_teamval.sh              # 全流程 + 自检
#   ./scripts/gen_genesis_teamval.sh --keep       # 保留中间产物（排查用）
#
# 环境变量：
#   GEN_HOME      输出目录（默认 <仓库根>/.mainnet-genesis）
#   CHAIN_ID      链 ID（默认 mcchain-mainnet-1）
#   SELF_DELEGATION  创世自抵押额（默认 120000000000000 umc = 全部团队池）
#   TEAM_KEYS     团队密钥文件（默认 <仓库根>/team_keys_gen.json，gitignore 内）
#   TEAM_ADDRESS  期望的团队多签地址。默认**从 init 产出的 genesis 里读**
#                 （tokenomics.allocations[name=team].address —— 链上权威值），
#                 因此不存在"文档里的地址过期"这一类问题。
#
# 主网必需环境变量（缺失即 fail-fast，因为链上密钥闸门会让 InitChain 直接失败）：
#   MC_REQUIRE_REAL_GENESIS_KEYS=1
#   MC_FOUNDATION_EARLY_DEV_PUBKEY / MC_FOUNDATION_OPS_PUBKEY / MC_FOUNDATION_VESTING_PUBKEY
#   （或把三个真实公钥写进 x/tokenomics/types/foundation_addrs_gen.go）
#
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GEN_HOME="${GEN_HOME:-$REPO_ROOT/.mainnet-genesis}"
CHAIN_ID="${CHAIN_ID:-mcchain-mainnet-1}"
SELF_DELEGATION="${SELF_DELEGATION:-120000000000000}"   # 1.2e14 umc = 团队池全额
MIN_SELF_DELEGATION="${MIN_SELF_DELEGATION:-30000000000}" # 3e10 umc，链上 ante 硬门槛
TEAM_KEYS="${TEAM_KEYS:-$REPO_ROOT/team_keys_gen.json}"
# TEAM_ADDRESS：留空则由步骤 1 生成的 genesis 反查（权威、自动跟随链上代码）。
TEAM_ADDRESS="${TEAM_ADDRESS:-}"
CFG="${CFG:-$REPO_ROOT/scripts/mainnet-genesis-config.json}"
KEEP=0
[[ "${1:-}" == "--keep" ]] && KEEP=1

log()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[!]\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31m[FATAL]\033[0m %s\n' "$*" >&2; exit 1; }

win() {
  if command -v cygpath >/dev/null 2>&1; then cygpath -w "$1"; else printf '%s' "$1"; fi
}

PY="$(command -v python3 || command -v python || true)"
[[ -n "$PY" ]] || die "需要 python3（用于构造/校验 genesis）"

# --- 定位二进制（Windows 上是 build/mcchaind.exe）-----------------------------
BIN=""
for cand in "$REPO_ROOT/build/mcchaind" "$REPO_ROOT/build/mcchaind.exe" "$REPO_ROOT/mcchaind.exe"; do
  [[ -x "$cand" ]] && { BIN="$cand"; break; }
done
[[ -n "$BIN" ]] || die "未找到 mcchaind，先执行：go build -o build/mcchaind ./cmd/mcchaind"

# MC：代替直接调用二进制，把 --home 的值转成原生路径（Windows 原生程序要求）。
MC() {
  local -a argv=()
  while (( $# )); do
    if [[ "$1" == "--home" && $# -ge 2 ]]; then
      argv+=("--home" "$(win "$2")"); shift 2
    else
      argv+=("$1"); shift
    fi
  done
  "$BIN" "${argv[@]}"
}

# ---------------------------------------------------------------------------
# 步骤 0：前置闸门（全部在动手之前检查，避免做到一半才发现链根本起不来）
# ---------------------------------------------------------------------------
log "[0] 前置检查"

[[ -f "$TEAM_KEYS" ]] || die "团队密钥文件不存在：$TEAM_KEYS
  主网创世需要 5 把团队多签私钥来重建 3-of-5 多签并签名 gentx。
  该文件包含助记词，必须在离线安全机器上生成并保管（已在 .gitignore 中）。"

if [[ "$CHAIN_ID" == *mainnet* || "${MC_REQUIRE_REAL_GENESIS_KEYS:-}" == "1" ]]; then
  missing=()
  [[ "${MC_REQUIRE_REAL_GENESIS_KEYS:-}" == "1" ]] || missing+=("MC_REQUIRE_REAL_GENESIS_KEYS=1")
  for v in MC_FOUNDATION_EARLY_DEV_PUBKEY MC_FOUNDATION_OPS_PUBKEY MC_FOUNDATION_VESTING_PUBKEY; do
    [[ -n "${!v:-}" ]] || missing+=("$v")
  done
  if (( ${#missing[@]} )); then
    die "主网创世密钥闸门未满足，链上 InitGenesis 会直接失败（链起不来）。缺失：
$(printf '      - %s\n' "${missing[@]}")
  说明：tokenomics.InitGenesis 在 mainnet chain-id（或 MC_REQUIRE_REAL_GENESIS_KEYS=1）下
  要求团队多签与三个基金会/早期开发拨付地址都是真实公钥；否则 30% 总量会落在
  源码可推导的占位私钥上。三个基金会公钥可用环境变量注入（推荐，源码不留密钥材料），
  也可写入 x/tokenomics/types/foundation_addrs_gen.go。
  参考 docs/DEPLOYMENT_GUIDE.md。"
  fi
  log "    主网密钥闸门已满足（团队公钥来自源码 gen 文件，基金会公钥来自环境变量）"
fi

# ---------------------------------------------------------------------------
# 步骤 1：初始化（产出零余额 genesis）
# ---------------------------------------------------------------------------
log "[1] mcchaind init（chain-id=$CHAIN_ID）"
rm -rf "$GEN_HOME"
mkdir -p "$GEN_HOME"
MC init validator --chain-id "$CHAIN_ID" --home "$GEN_HOME" >/dev/null 2>&1

# 团队地址的**权威来源** = 链上代码派生的 tokenomics.allocations[name=team].address。
# 直接读刚生成的 genesis，杜绝"文档/脚本里硬编码的地址随密钥轮换而过期"这一类地雷
# （硬编码地址会随团队密钥轮换而过期，照抄会把创世 1.2e14 团队池
#   打到一个没人有私钥的地址上）。
if [[ -z "$TEAM_ADDRESS" ]]; then
  TEAM_ADDRESS="$("$PY" -c "
import json,sys
as_ = json.load(open(sys.argv[1], encoding='utf-8'))['app_state']
for a in ((as_.get('tokenomics') or {}).get('allocations') or []):
    if a.get('name') == 'team':
        print(a.get('address')); break
" "$(win "$GEN_HOME/config/genesis.json")")"
  [[ -n "$TEAM_ADDRESS" ]] || die "genesis 中找不到 tokenomics.allocations[name=team].address"
  log "    链上团队多签地址（权威） = $TEAM_ADDRESS"
fi

# ---------------------------------------------------------------------------
# 步骤 2：恢复 5 把团队私钥 + 重建 3-of-5 多签，并校验地址
#         （这一步堵死「脚本里的助记词与链上团队公钥不一致」这一类静默错配）
# ---------------------------------------------------------------------------
log "[2] 恢复团队密钥并重建 3-of-5 多签"
mapfile -t ENTRIES < <("$PY" -c "
import json,sys
for e in json.load(open(sys.argv[1], encoding='utf-8')):
    print(e['name'] + '|' + e['mnemonic'])
" "$(win "$TEAM_KEYS")")
(( ${#ENTRIES[@]} == 5 )) || die "团队密钥文件应有 5 条记录，实际 ${#ENTRIES[@]}"

for e in "${ENTRIES[@]}"; do
  name="${e%%|*}"; mnem="${e#*|}"
  printf '%s\n' "$mnem" | MC keys add "$name" --recover --keyring-backend test \
    --home "$GEN_HOME" >/dev/null 2>&1 || die "恢复 $name 失败"
done
MC keys add teammultisig --multisig=team1,team2,team3,team4,team5 \
  --multisig-threshold=3 --keyring-backend test --home "$GEN_HOME" >/dev/null 2>&1 \
  || die "构建 teammultisig 失败"
MSADDR="$(MC keys show teammultisig -a --keyring-backend test --home "$GEN_HOME")"
log "    多签地址 = $MSADDR"
[[ "$MSADDR" == "$TEAM_ADDRESS" ]] || die "多签地址与链上 TeamAddress 不一致：
  重建得到   : $MSADDR
  链上期望值 : $TEAM_ADDRESS
  说明：链上 team_pubkeys_gen.go 的公钥与 $TEAM_KEYS 的助记词不是同一组，
  或者链上 TeamAddress 已变更。两者必须一致，否则创世拨付的 1.2e14 团队池
  会落到一个谁都签不动的地址上。修正后再重跑；如确为链上变更，
  用 TEAM_ADDRESS=<新地址> 覆盖并通过 governance 渠道确认。"

# ---------------------------------------------------------------------------
# 步骤 3：临时给团队地址记账（仅为了通过 gentx 的 ValidateAccountInGenesis）
#         额度 = 自抵押额，且必须在步骤 8 删除 —— 否则击穿总量硬顶。
# ---------------------------------------------------------------------------
log "[3] 临时记入团队地址余额 ${SELF_DELEGATION}umc（gentx 前置条件，第 8 步删除）"
MC add-genesis-account "$MSADDR" "${SELF_DELEGATION}umc" --home "$GEN_HOME" >/dev/null 2>&1

# ---------------------------------------------------------------------------
# 步骤 4：生成 gentx（多签 → 未签名）
# ---------------------------------------------------------------------------
log "[4] 生成 unsigned gentx（--from teammultisig）"
TMP="$GEN_HOME/_tmp"; mkdir -p "$TMP"
MC gentx teammultisig "${SELF_DELEGATION}umc" \
  --chain-id "$CHAIN_ID" --from teammultisig --home "$GEN_HOME" \
  --keyring-backend test --min-self-delegation "$MIN_SELF_DELEGATION" \
  --generate-only > "$TMP/unsigned_gentx.json" \
  || die "gentx 生成失败（检查 --min-self-delegation 与账户余额）"

# ---------------------------------------------------------------------------
# 步骤 5/6：team1/2/3 离线签名 → 多签合并
#   account-number/sequence 必须为 0/0：SDK 在高度 0 验签时强制 AccountNumber=0。
# ---------------------------------------------------------------------------
log "[5] team1/2/3 离线签名"
for k in team1 team2 team3; do
  MC tx sign "$(win "$TMP/unsigned_gentx.json")" --from "$k" --multisig=teammultisig \
    --signature-only --offline --account-number 0 --sequence 0 \
    --keyring-backend test --home "$GEN_HOME" --chain-id "$CHAIN_ID" \
    > "$TMP/sig_$k.json" || die "$k 签名失败"
done

log "[6] 多签合并"
MC tx multisign "$(win "$TMP/unsigned_gentx.json")" teammultisig \
  "$(win "$TMP/sig_team1.json")" "$(win "$TMP/sig_team2.json")" "$(win "$TMP/sig_team3.json")" \
  --offline --account-number 0 --sequence 0 --keyring-backend test \
  --home "$GEN_HOME" --chain-id "$CHAIN_ID" > "$TMP/signed_gentx.json" \
  || die "multisign 失败"

# ---------------------------------------------------------------------------
# 步骤 7：收集 gentx
# ---------------------------------------------------------------------------
log "[7] collect-gentxs"
mkdir -p "$GEN_HOME/config/gentx"
cp "$TMP/signed_gentx.json" "$GEN_HOME/config/gentx/gentx_teammultisig.json"
MC collect-gentxs --home "$GEN_HOME" >/dev/null 2>&1

# ---------------------------------------------------------------------------
# 步骤 8：删除临时 genesis 账户（auth + bank 两侧），使创世账本余额归零
# ---------------------------------------------------------------------------
log "[8] 删除临时 genesis 账户（供给恢复为 0，tokenomics 创世时铸 1e15）"
"$PY" - "$(win "$GEN_HOME/config/genesis.json")" "$MSADDR" <<'PY'
import json, sys
path, addr = sys.argv[1], sys.argv[2]
g = json.load(open(path, encoding="utf-8"))
as_ = g["app_state"]
as_.setdefault("auth", {})["accounts"] = [
    a for a in (as_.get("auth", {}).get("accounts") or []) if a.get("address") != addr]
as_.setdefault("bank", {})["balances"] = [
    b for b in (as_.get("bank", {}).get("balances") or []) if b.get("address") != addr]
json.dump(g, open(path, "w", encoding="utf-8"), indent=2, ensure_ascii=False)
print("    临时账户已删除：%s" % addr)
PY

# ---------------------------------------------------------------------------
# 步骤 9：生产创世规范化（make_genesis.py 内含五池不变量 + 账本供应不变量）
# ---------------------------------------------------------------------------
log "[9] make_genesis.py 规范化（denom / 五池 / 节点治理参数 / 供应不变量）"
"$PY" "$(win "$REPO_ROOT/scripts/make_genesis.py")" \
  --genesis "$(win "$GEN_HOME/config/genesis.json")" \
  --out "$(win "$GEN_HOME/config/genesis.json")" \
  --config "$(win "$CFG")"

# ---------------------------------------------------------------------------
# 步骤 10：validate-genesis + 终局自检（supply / gentx / phonenode 参数）
# ---------------------------------------------------------------------------
log "[10] validate-genesis"
MC validate-genesis --home "$GEN_HOME" >/dev/null 2>&1
log "    OK"

log "[11] 终局自检"
"$PY" - "$(win "$GEN_HOME/config/genesis.json")" <<'PY'
import json, sys
g = json.load(open(sys.argv[1], encoding="utf-8"))
as_ = g["app_state"]
fail = []

total = sum(int(b.get("amount", 0)) for b in (as_.get("bank", {}).get("balances") or []))
nbal = len(as_.get("bank", {}).get("balances") or [])
if total != 0:
    fail.append("bank 账本余额 %d umc（%d 个账户）!= 0" % (total, nbal))
if nbal == 0:
    print("    ✅ 创世账本余额 = 0（tokenomics 将在 InitGenesis 铸 1e15 并拨入五池）")

minted = int((as_.get("tokenomics") or {}).get("minted_supply", 0) or 0)
if minted != 0:
    fail.append("tokenomics.minted_supply=%d != 0（全新创世不应是恢复模式）" % minted)

gentxs = (as_.get("genutil") or {}).get("gen_txs") or []
if len(gentxs) != 1:
    fail.append("gentx 数量 = %d，期望 1" % len(gentxs))
else:
    print("    ✅ gentx = 1 笔（团队 3-of-5 多签）")

pn = (as_.get("phonenode") or {}).get("params") or {}
if not pn.get("attestation_required"):
    fail.append("phonenode.params.attestation_required 非 true —— 设备认证不强制，抗女巫失效")
else:
    print("    ✅ phonenode.attestation_required = true")

ea = (as_.get("edgeai") or {}).get("params") or {}
if not ea.get("arbitrator"):
    fail.append("edgeai.params.arbitrator 缺失")
else:
    print("    ✅ edgeai.arbitrator = %s" % ea["arbitrator"])

if fail:
    print("\n[FATAL] 终局自检未通过：")
    for f in fail:
        print("  - %s" % f)
    sys.exit(1)
PY

[[ "$KEEP" == "1" ]] || rm -rf "$TMP"

log "GENESIS READY -> $GEN_HOME/config/genesis.json"
cat <<EOF

下一步（在主网上线前逐项执行）：
  1) 公示 sha256：      sha256sum $GEN_HOME/config/genesis.json
  2) 复制到节点 home：  cp $GEN_HOME/config/genesis.json <节点>/config/genesis.json
                        （同时复制 config/gentx/ 与 config/priv_validator_key.json、node_key.json）
  3) 起链（必须带齐环境变量，见 docs/DEPLOYMENT_GUIDE.md）：
       MC_REQUIRE_REAL_GENESIS_KEYS=1 \\
       MC_FOUNDATION_EARLY_DEV_PUBKEY=... MC_FOUNDATION_OPS_PUBKEY=... \\
       MC_FOUNDATION_VESTING_PUBKEY=... mcchaind start --home <节点>
  4) 起链后验收：      ./scripts/launch_verify.sh http://127.0.0.1:26657 900
     重点核对：总供应 == 1000000000000000 umc、团队 vesting 账户 == 120000000000000 umc、
              验证人在线出块、edgeai.arbitrator == 团队多签地址。
EOF
