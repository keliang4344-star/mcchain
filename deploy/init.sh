#!/usr/bin/env bash
# MobileChain 主网初始化脚本（在目标 Linux 服务器上以 root 或专用用户执行）
# 前置：已构建镜像或已安装 mcchaind；链 home 目录为空。
set -euo pipefail

CHAIN_ID="${CHAIN_ID:-mcchain-mainnet-1}"
HOME_DIR="${HOME_DIR:-$HOME/.mcchain}"
MONIKER="${MONIKER:-mc-validator}"

echo ">> init chain $CHAIN_ID"
mcchaind init "$MONIKER" --chain-id "$CHAIN_ID" --home "$HOME_DIR"

# 启用监控 telemetry。
#
# CometBFT 侧（config.toml）：prometheus + prometheus_listen_addr = 26660，
# 有这两个字段，sed 有效。
sed -i 's/^prometheus = false/prometheus = true/' "$HOME_DIR/config/config.toml"
sed -i 's#^prometheus_listen_addr = ".*"#prometheus_listen_addr = "0.0.0.0:26660"#' "$HOME_DIR/config/config.toml"

# 应用层（app.toml）：
#
# ⚠️ 不要对 app.toml 执行 `sed 's/^prometheus = false/prometheus = true/'` 与
# `prometheus_listen_addr = "0.0.0.0:26661"` —— 那是**静默空操作**：SDK v0.47.14 的
# app.toml [telemetry] 段只有 `enabled` 与 `prometheus-retention-time` 两个字段，
# 根本没有 prometheus / prometheus_listen_addr（那是 CometBFT 的 config.toml 字段）。
# 日志照样打印「telemetry 已启用」，但链上 telemetry 从未打开 →
# Prometheus 抓 26661 永远 up=0，mcchain_* 指标一个都不存在，
# deploy/mcchain_alerts.yml 里依赖它们的规则永不触发（含停块、供应、认证停滞）。
#
# 正确写法（SDK v0.47）：telemetry.enabled=true + prometheus-retention-time>0，
# 指标由 **API server 的 /metrics** 暴露（server/api/server.go registerMetrics），
# 因此还需要把 [api] 打开并监听 0.0.0.0:1317 供 Prometheus 抓取。
# app.toml 里 `enabled = false` 出现多次（api/grpc/telemetry/rosetta…），
# 必须按段修改，用 awk 段感知替换而不是裸 sed。
awk '
  /^\[/ { sec = $0 }
  sec == "[telemetry]" && /^enabled = / { print "enabled = true"; next }
  sec == "[telemetry]" && /^prometheus-retention-time = / { print "prometheus-retention-time = 60"; next }
  sec == "[api]" && /^enable = / { print "enable = true"; next }
  sec == "[api]" && /^address = / { print "address = \"tcp://0.0.0.0:1317\""; next }
  { print }
' "$HOME_DIR/config/app.toml" > "$HOME_DIR/config/app.toml.tmp" \
  && mv "$HOME_DIR/config/app.toml.tmp" "$HOME_DIR/config/app.toml"

# 落盘校验：任何一项没生效就 fail-fast（避免又一次「打印成功但没写进去」）。
_app_ok=1
grep -A3 '^\[telemetry\]' "$HOME_DIR/config/app.toml" | grep -q '^enabled = true' || _app_ok=0
grep -A5 '^\[telemetry\]' "$HOME_DIR/config/app.toml" | grep -q '^prometheus-retention-time = 60' || _app_ok=0
grep -A9 '^\[api\]' "$HOME_DIR/config/app.toml" | grep -q 'tcp://0.0.0.0:1317' || _app_ok=0
if [[ "$_app_ok" != "1" ]]; then
  echo "[FATAL] app.toml telemetry/api 配置未生效——Prometheus 将抓不到 mcchain_* 指标（告警全部失效）" >&2
  echo "        请检查 $HOME_DIR/config/app.toml 是否为 SDK v0.47 格式" >&2
  exit 1
fi
echo ">> telemetry 已启用（指标经 API server /metrics 暴露，见 prometheus-mcchain.yml）"

# 最小 gas 价格（防 spam）。默认 0umc 方便移动端 0 手续费挖矿联调；
# 生产主网设环境变量 MIN_GAS_PRICES=0.0025umc 再跑 init.sh。
MIN_GAS="${MIN_GAS_PRICES:-0umc}"
sed -i "s/^minimum-gas-prices = .*/minimum-gas-prices = \"$MIN_GAS\"/" "$HOME_DIR/config/app.toml"
echo ">> min-gas-prices set to: $MIN_GAS"

# 可选：区块 gas 上限（防无限 gas DoS）。
#
# 注意：max_gas 是 **genesis 的共识参数** consensus_params.block.max_gas，
# app.toml 里并没有这个字段。旧版本此处对 app.toml 执行 `sed s/^max_gas = .*/`
# 是一次静默空操作——日志照样打印 ">> max_gas set to"，但配置从未被写入，
# 区块 gas 上限实际仍是 CometBFT 默认的 -1（不限）。这里改为直接改写 genesis。
#
# 未设 MAX_GAS 时：下方 make_genesis.py 兜底写入 100000000；即便绕开脚本建链，
# 链上 InitChainer 也会把无界的 max_gas 钳制到 DefaultBlockMaxGas（见 app/app.go）。
if [ -n "${MAX_GAS:-}" ]; then
  python3 - "$HOME_DIR/config/genesis.json" "$MAX_GAS" <<'PY'
import json, sys

path = sys.argv[1]
# int() 兼作校验：MAX_GAS 非整数时直接报错退出，不写出非法 genesis。
max_gas = str(int(sys.argv[2]))
with open(path, "r", encoding="utf-8") as f:
    genesis = json.load(f)
block = genesis.setdefault("consensus_params", {}).setdefault("block", {})
block["max_gas"] = max_gas
with open(path, "w", encoding="utf-8") as f:
    json.dump(genesis, f, indent=2)
PY
  echo ">> consensus_params.block.max_gas set to: $MAX_GAS"
fi

# 用生产 genesis 生成器规范化（denom=umc / DePIN 池 / 上限 / chain_id / 五池不变量 /
# 账本供应不变量）。
#
# ⚠️ 重要：**不要**在本脚本之前用 add-genesis-account
# 「把验证人/团队/生态账户写入 genesis」。全链 1e15 umc 由 tokenomics 在 InitGenesis
# 一次性铸造并拨入五池，创世 bank.balances 必须为空；add-genesis-account 是凭空记账，
# 会让生产链在创世供应上限校验处拒绝启动（app.assertGenesisSupplyCap）。
#
# 本脚本适用于「genesis 已在离线安全机器上产出、现在只需在目标主机拉链」的场景
# （即 genesis.json 已由 scripts/gen_genesis_teamval.sh 生成并拷入 $HOME_DIR/config/）。
# 完整主网创世流程见 docs/DEPLOYMENT_GUIDE.md；四道闸门环境变量见同文档，
# 模板文件 deploy/mainnet.env.example。
python3 scripts/make_genesis.py \
  --genesis "$HOME_DIR/config/genesis.json" \
  --out "$HOME_DIR/config/genesis.json" \
  --config scripts/mainnet-genesis-config.json

echo ">> validate genesis"
mcchaind validate-genesis "$HOME_DIR/config/genesis.json"

echo "init done. 接下来：收集 gentx 并 collect-gentxs，再 start。"
