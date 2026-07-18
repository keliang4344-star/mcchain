#!/usr/bin/env bash
# 生成「团队多签为创世验证人」的生产 genesis（无 add-genesis-account，供给严格 = 1e15）。
# 仅用于本地深度审计 / 主网前演练；真实主网请在安全机器上执行。
set +e
export MSYS_NO_PATHCONV=1
BIN="$HOME/mcchain/mcchaind.exe"
HD="$HOME/mcchain/audit_home"
TMP="$HOME/mcchain/audit_tmp"
CHAIN_ID="mcchain-mainnet-1"
CFG="$HOME/mcchain/scripts/mainnet-genesis-config.json"
PY=python3
SELF_DEL="120000000000000umc"   # 1.2e14 = 全部团队池自抵押（五池模型团队 12%）
MIN_SELF="30000000000"          # 3e10

# 清理（用 rm -rf 清理整目录；msys safe-delete 可能拦截但 set +e 不中断）
rm -rf "$HD" 2>/dev/null
rm -rf "$TMP" 2>/dev/null
mkdir -p "$HD" "$TMP"

echo "== [1] init =="
"$BIN" init validator --chain-id "$CHAIN_ID" --home "$HD" 2>&1
echo "init exit=$?"

echo "== [1.5] 修复 phonenode + edgeai params 空序列化 bug =="
"$PY" -c "
import json
p = '$HOME/mcchain/audit_home/config/genesis.json'
g = json.load(open(p))
# phonenode
pn = g.setdefault('app_state',{}).setdefault('phonenode',{})
pn['params'] = {
    'attestation_required': True,
    'attestation_validity': '2592000',
    'sybil_device_binding': True,
    'offline_grace_blocks': '100',
    'offline_slash_bps': '500',
    'contrib_slash_bps': '1000',
    'attest_slash_bps': '2000',
}
# edgeai — 只需要设 arbitrator（其他字段 DefaultParams 已有正确值）
ea = g.setdefault('app_state',{}).setdefault('edgeai',{})
ea['params']['arbitrator'] = 'mc1uq85t4erj44lf3x23xnrr97lt4wlyfz5kkf96f'
json.dump(g, open(p,'w'), indent=2)
print('phonenode+edgeai params fixed')
"

echo "== [1.7] 添加临时 TeamAddress genesis-account（gentx 需要，后面删掉）=="
"$BIN" add-genesis-account mc1uq85t4erj44lf3x23xnrr97lt4wlyfz5kkf96f 120000000000000umc --home "$HD" 2>&1 | tail -2
echo "add-genesis-account exit=$?"

# 用真实团队助记词恢复 5 把私钥到 test keyring（仅审计用，演练后清理）
declare -a MN=(
 "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon art"
 "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon art"
 "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon art"
 "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon art"
 "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon art"
)
for i in 1 2 3 4 5; do
  idx=$((i-1))
  printf '%s\n' "${MN[$idx]}" | "$BIN" keys add "team$i" --recover --keyring-backend test --home "$HD" >/dev/null 2>&1
  echo "recover team$i exit=$?"
done

echo "== [2] 构建 3-of-5 多签 teammultisig =="
"$BIN" keys add teammultisig --multisig=team1,team2,team3,team4,team5 --multisig-threshold=3 --keyring-backend test --home "$HD" >/dev/null 2>&1
echo "multisig exit=$?"
MSADDR=$("$BIN" keys show teammultisig -a --keyring-backend test --home "$HD")
echo "TeamMultisig addr = $MSADDR"

echo "== [3] 生成 unsigned gentx（--from teammultisig）=="
"$BIN" gentx teammultisig "$SELF_DEL" \
  --chain-id "$CHAIN_ID" --from teammultisig --home "$HD" --keyring-backend test \
  --min-self-delegation "$MIN_SELF" --generate-only > "$TMP/unsigned_gentx.json" 2>"$TMP/gentx_err"
echo "gentx exit=$?"; cat "$TMP/gentx_err"

echo "== [4] 用 team1/2/3 签名（offline, seq=0）=="
"$BIN" tx sign "$TMP/unsigned_gentx.json" --from team1 --multisig=teammultisig --signature-only --offline --account-number 0 --sequence 0 --keyring-backend test --home "$HD" --chain-id "$CHAIN_ID" > "$TMP/sig1.json" 2>"$TMP/s1"
echo "sign1 exit=$?"; cat "$TMP/s1"
"$BIN" tx sign "$TMP/unsigned_gentx.json" --from team2 --multisig=teammultisig --signature-only --offline --account-number 0 --sequence 0 --keyring-backend test --home "$HD" --chain-id "$CHAIN_ID" > "$TMP/sig2.json" 2>"$TMP/s2"
echo "sign2 exit=$?"; cat "$TMP/s2"
"$BIN" tx sign "$TMP/unsigned_gentx.json" --from team3 --multisig=teammultisig --signature-only --offline --account-number 0 --sequence 0 --keyring-backend test --home "$HD" --chain-id "$CHAIN_ID" > "$TMP/sig3.json" 2>"$TMP/s3"
echo "sign3 exit=$?"; cat "$TMP/s3"

echo "== [5] multisign（offline）=="
"$BIN" tx multisign "$TMP/unsigned_gentx.json" teammultisig "$TMP/sig1.json" "$TMP/sig2.json" "$TMP/sig3.json" --offline --account-number 0 --sequence 0 --keyring-backend test --home "$HD" --chain-id "$CHAIN_ID" > "$TMP/signed_gentx.json" 2>"$TMP/ms"
echo "multisign exit=$?"; cat "$TMP/ms"

echo "== [6] 放入 gentx + collect =="
mkdir -p "$HD/config/gentx"
cp "$TMP/signed_gentx.json" "$HD/config/gentx/gentx_teammultisig.json"
"$BIN" collect-gentxs --home "$HD" 2>&1 | tail -3
echo "collect exit=$?"

echo "== [6.5] 删除临时 TeamAddress genesis-account（供给恢复 = 0，tokenomics 仅创世时铸造 1e15）=="
"$PY" -c "
import json
p = '$HOME/mcchain/audit_home/config/genesis.json'
g = json.load(open(p))
addr = 'mc1uq85t4erj44lf3x23xnrr97lt4wlyfz5kkf96f'
# 从 auth 账户列表删除
acs = g['app_state'].get('auth',{}).get('accounts',[])
g['app_state']['auth']['accounts'] = [a for a in acs if a.get('address') != addr]
# 从 bank 余额列表删除
bals = g['app_state'].get('bank',{}).get('balances',[])
g['app_state']['bank']['balances'] = [b for b in bals if b.get('address') != addr]
json.dump(g, open(p,'w'), indent=2)
print('temp genesis-account removed for', addr)
"

echo "== [7] make_genesis 规范化 =="
"$PY" scripts/make_genesis.py --genesis "$HD/config/genesis.json" --out "$HD/config/genesis.json" --config "$CFG"
echo "make_genesis exit=$?"

echo "== [8] validate-genesis =="
"$BIN" validate-genesis "$HD/config/genesis.json" 2>&1 | tail -3
echo "validate exit=$?"

echo "GENESIS_HOME=$HD"
