#!/usr/bin/env bash
# ============================================================================
#  MC 公链 · 一键拉起「solo 创世节点」（新手友好）
#  用法：把 mcchaind 和本脚本放到同一目录，然后：
#        chmod +x server_setup.sh
#        sudo ./server_setup.sh
#  脚本会自动完成：初始化 → 创世规范化 → 建验证人 → 拨款 → 生成 gentx → 启动
#  注意：这是单验证人「原始/创世节点」（solo 网络，0 容错），不是去中心化主网。
# ============================================================================

set -e

BIN="$(cd "$(dirname "$0")" && pwd)/mcchaind"
HOME_DIR="$HOME/.mcchain"
CHAIN_ID="mcchain-mainnet-1"
MONIKER="${MONIKER:-mc-genesis}"
KEY="validator"

# 金额（umc，1 MC = 1,000,000 umc）
SELF_BOND="30000000000umc"         # 自抵押 30k MC（>= ante 最低 3e10umc）
GENESIS_AMT="200000000000umc"      # 创世账户拨款 200k MC
MIN_SELF_DELEGATION="30000000"     # 必须 >= 30k MC，否则创世执行 panic（整数，不带 umc）

echo "=============================================="
echo " MC 公链 solo 创世节点一键部署"
echo " chain_id = $CHAIN_ID"
echo " home     = $HOME_DIR"
echo "=============================================="

# 0) 杀掉可能残留的 mcchaind（避免数据目录被锁）
pkill -9 -f "$BIN" 2>/dev/null || true
sleep 2

# 1) 初始化
echo ">> [1/7] 初始化节点"
"$BIN" init "$MONIKER" --chain-id "$CHAIN_ID" --home "$HOME_DIR"

# 2) 规范化创世（denom=umc、通胀清零、治理参数、tokenomics 上限）
#    —— 必须用脚本内联 python 完成，否则默认币种是 stake 而非 umc
echo ">> [2/7] 规范化创世（denom=umc, 通胀清零, 治理参数）"
python3 - "$HOME_DIR/config/genesis.json" <<'PY'
import json, sys
p = sys.argv[1]
g = json.load(open(p, encoding="utf-8"))
as_ = g["app_state"]
denom = "umc"
as_["staking"]["params"]["bond_denom"] = denom
as_["mint"]["params"]["mint_denom"] = denom
ZERO = "0.000000000000000000"
mp = as_["mint"]["params"]
for k in ("inflation_rate_change", "inflation_max", "inflation_min"):
    if k in mp:
        mp[k] = ZERO
mtr = as_["mint"].get("minter", {})
if mtr:
    mtr["inflation"] = ZERO
    mtr["annual_provisions"] = ZERO
    as_["mint"]["minter"] = mtr
for d in (as_.get("gov", {}).get("params", {}) or {}).get("min_deposit", []) or []:
    if d.get("denom") == "stake":
        d["denom"] = denom
cf = (as_.get("crisis", {}) or {}).get("constant_fee")
if cf and cf.get("denom") == "stake":
    cf["denom"] = denom
gov = as_.get("gov", {})
gp = gov.get("params", {})
if gp:
    md = gp.get("min_deposit", []) or []
    for d in md:
        if d.get("denom") == "stake":
            d["denom"] = denom
    if md:
        md[0]["amount"] = "10000000"
    gp["voting_period"] = "172800s"
    gp["max_deposit_period"] = "172800s"
    gp["quorum"] = "0.334000000000000000"
    gp["threshold"] = "0.500000000000000000"
    gp["veto_threshold"] = "0.334000000000000000"
    gov["params"] = gp
    as_["gov"] = gov
if "depin" in as_:
    as_["depin"]["params"]["reward_denom"] = denom
    as_["depin"]["params"]["initial_pool"] = "100000000000000"
if "tokenomics" in as_:
    as_["tokenomics"]["denom"] = denom
    as_["tokenomics"]["total_supply_cap"] = 1000000000000000
if "edgeai" in as_ and "tokenomics" in as_:
    team = None
    for a in as_["tokenomics"].get("allocations", []) or []:
        if a.get("name") == "team":
            team = a.get("address")
            break
    if team:
        as_["edgeai"]["params"]["arbitrator"] = team
g["chain_id"] = "mcchain-mainnet-1"
json.dump(g, open(p, "w", encoding="utf-8"), indent=2, ensure_ascii=False)
PY
echo "    创世已规范化"

# 3) 创建验证人密钥（test 后端免密，仅适合 demo/原始节点！）
#    ⚠️ 生产环境请用 --keyring-backend file 并妥善保管助记词
echo ">> [3/7] 创建验证人密钥（test 后端，免密）"
"$BIN" keys add "$KEY" --keyring-backend test --home "$HOME_DIR" --output json > validator_key.json 2>/dev/null
ADDR=$("$BIN" keys show "$KEY" -a --keyring-backend test --home "$HOME_DIR")
echo "    验证人地址: $ADDR"
echo "    ⚠️ 助记词已保存到 validator_key.json，请妥善备份！"

# 4) 创世账户拨款
echo ">> [4/7] 创世账户拨款（200k MC）"
"$BIN" add-genesis-account "$ADDR" "$GENESIS_AMT" --home "$HOME_DIR"

# 5) 生成 gentx（自抵押 30k MC；min-self-delegation 必须 >= 30k MC）
echo ">> [5/7] 生成 gentx（自抵押 30k MC）"
"$BIN" gentx "$KEY" "$SELF_BOND" \
  --chain-id "$CHAIN_ID" \
  --moniker "$MONIKER" \
  --commission-rate 0.05 \
  --min-self-delegation "$MIN_SELF_DELEGATION" \
  --keyring-backend test \
  --home "$HOME_DIR"

# 6) 收集 gentx + 校验创世
echo ">> [6/7] 收集 gentx + 校验创世"
"$BIN" collect-gentxs --home "$HOME_DIR"
"$BIN" validate-genesis --home "$HOME_DIR"

# 7) 启动节点（后台运行，日志写 chain.log）
echo ">> [7/7] 启动节点（后台，日志 chain.log）"
nohup "$BIN" start --home "$HOME_DIR" > chain.log 2>&1 &
echo ""
echo "✅ 完成！节点已在后台启动。"
echo "   查看出块:  tail -f chain.log"
echo "   停止节点:  pkill -9 -f mcchaind"
echo "   验证人地址已保存到 validator_key.json（务必备份助记词）"
echo ""
echo "   稍等 10 秒后执行： tail -f chain.log  应能看到 height=1,2,3... 持续出块"
