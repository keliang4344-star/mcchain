#!/usr/bin/env bash
#
# 团队资金可控性终极判据：用真实 mcchaind 从 team_keys_gen.json 的助记词恢复 5 把私钥，
# 按标准 `keys add --multisig`（默认按地址排序）重建 3-of-5 多签，比较其地址是否与
# 链上编译的 TeamAddress 一致。不一致 = 创世拨付的 1.2e14 团队池会落到没人签得动的地址。
#
# 期望值**不硬编码**：从 `mcchaind init` 产出的 genesis 里读
# tokenomics.allocations[name=team].address —— 那就是链上代码派生的 TeamAddress。
# （硬编码地址会随团队密钥轮换而过期，照抄会把团队资金打到一个废弃地址。）
#
# 该检查同时是 scripts/gen_genesis_teamval.sh 的第 2 步，二者共用同一判据。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
JSON="${TEAM_KEYS:-$REPO_ROOT/team_keys_gen.json}"
VERIFY_HOME="${VERIFY_HOME:-$REPO_ROOT/.teamverify}"

log()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31m[FATAL]\033[0m %s\n' "$*" >&2; exit 1; }
win()  { if command -v cygpath >/dev/null 2>&1; then cygpath -w "$1"; else printf '%s' "$1"; fi; }

PY="$(command -v python3 || command -v python || true)"
[[ -n "$PY" ]] || die "需要 python3"

BIN=""
for cand in "$REPO_ROOT/build/mcchaind" "$REPO_ROOT/build/mcchaind.exe"; do
  [[ -x "$cand" ]] && { BIN="$cand"; break; }
done
[[ -n "$BIN" ]] || die "未找到 mcchaind（go build -o build/mcchaind ./cmd/mcchaind）"
[[ -f "$JSON" ]] || die "团队密钥文件不存在：$JSON"

MC() {
  local -a argv=()
  while (( $# )); do
    if [[ "$1" == "--home" && $# -ge 2 ]]; then argv+=("--home" "$(win "$2")"); shift 2
    else argv+=("$1"); shift; fi
  done
  "$BIN" "${argv[@]}"
}

rm -rf "$VERIFY_HOME"; mkdir -p "$VERIFY_HOME"

log "从链上代码派生期望地址（init 产物的 tokenomics.allocations[team].address）"
MC init verify --chain-id "${CHAIN_ID:-mcchain-mainnet-1}" --home "$VERIFY_HOME" >/dev/null 2>&1
EXPECTED="$("$PY" -c "
import json,sys
as_=json.load(open(sys.argv[1],encoding='utf-8'))['app_state']
print(next((a['address'] for a in ((as_.get('tokenomics') or {}).get('allocations') or [])
            if a.get('name')=='team'), ''))
" "$(win "$VERIFY_HOME/config/genesis.json")")"
[[ -n "$EXPECTED" ]] || die "无法从 genesis 取到 team 分配地址"
log "链编译 TeamAddress = $EXPECTED"

log "恢复 5 把私钥并重建 3-of-5 多签"
mapfile -t MNS < <("$PY" -c "
import json,sys
for e in json.load(open(sys.argv[1],encoding='utf-8')): print(e['mnemonic'])
" "$(win "$JSON")")
(( ${#MNS[@]} == 5 )) || die "团队密钥文件应有 5 条助记词，实际 ${#MNS[@]}"
for i in 1 2 3 4 5; do
  printf '%s\n' "${MNS[$((i-1))]}" | MC keys add "team$i" --recover \
    --keyring-backend test --home "$VERIFY_HOME" >/dev/null 2>&1 || die "恢复 team$i 失败"
done
MC keys add teammultisig --multisig=team1,team2,team3,team4,team5 --multisig-threshold=3 \
  --keyring-backend test --home "$VERIFY_HOME" >/dev/null 2>&1 || die "构建多签失败"
GOT="$(MC keys show teammultisig -a --keyring-backend test --home "$VERIFY_HOME")"
log "重建多签地址 = $GOT"

rm -rf "$VERIFY_HOME"

if [[ "$GOT" == "$EXPECTED" ]]; then
  echo "✅ 通过：助记词重建的多签 == 链编译 TeamAddress，团队资金可控。"
else
  echo "❌ 失败：重建多签($GOT) != 链编译 TeamAddress($EXPECTED)"
  echo "   团队资金可能永久锁定。禁止在此状态下创世。"
  exit 1
fi
