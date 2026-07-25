#!/usr/bin/env bash
# MobileChain 主网初始化脚本（在目标 Linux 服务器上以 root 或专用用户执行）
# 前置：已构建镜像或已安装 mcchaind；链 home 目录为空。
set -euo pipefail

CHAIN_ID="${CHAIN_ID:-mcchain-mainnet-1}"
HOME_DIR="${HOME_DIR:-$HOME/.mcchain}"
MONIKER="${MONIKER:-mc-validator}"

echo ">> init chain $CHAIN_ID"
mcchaind init "$MONIKER" --chain-id "$CHAIN_ID" --home "$HOME_DIR"

# 启用监控 telemetry：cometbft prometheus 监听 26660，cosmos-sdk app telemetry 监听 26661
# （两者端口不同，避免与 cometbft 冲突；monitoring/prometheus.yml 同时抓取两者）
sed -i 's/^prometheus = false/prometheus = true/' "$HOME_DIR/config/config.toml"
sed -i 's#^prometheus_listen_addr = ".*"#prometheus_listen_addr = "0.0.0.0:26660"#' "$HOME_DIR/config/config.toml"
sed -i 's/^prometheus = false/prometheus = true/' "$HOME_DIR/config/app.toml"
sed -i 's#^prometheus_listen_addr = ".*"#prometheus_listen_addr = "0.0.0.0:26661"#' "$HOME_DIR/config/app.toml"

# 最小 gas 价格（防 spam）。默认 0umc 方便移动端 0 手续费挖矿联调；
# 生产主网设环境变量 MIN_GAS_PRICES=0.0025umc 再跑 init.sh。
MIN_GAS="${MIN_GAS_PRICES:-0umc}"
sed -i "s/^minimum-gas-prices = .*/minimum-gas-prices = \"$MIN_GAS\"/" "$HOME_DIR/config/app.toml"
echo ">> min-gas-prices set to: $MIN_GAS"

# 可选：app.toml max_gas 单 tx 上限（防无限 gas DoS）。默认留空 = 不改（0=不限）。
# 生产建议设例如 MAX_GAS=10000000 再跑 init.sh。
if [ -n "${MAX_GAS:-}" ]; then
  sed -i "s/^max_gas = .*/max_gas = ${MAX_GAS}/" "$HOME_DIR/config/app.toml"
  echo ">> max_gas set to: $MAX_GAS"
fi

# 用生产 genesis 生成器规范化（denom=umc / DePIN 池 / 上限 / chain_id）
# 注意：部署前需先 add-genesis-account 把验证人/团队/生态账户写入 genesis，
# 再把 genesis.json 路径传给 make_genesis.py。
python3 scripts/make_genesis.py \
  --genesis "$HOME_DIR/config/genesis.json" \
  --out "$HOME_DIR/config/genesis.json" \
  --config scripts/mainnet-genesis-config.json

echo ">> validate genesis"
mcchaind validate-genesis "$HOME_DIR/config/genesis.json"

echo "init done. 接下来：收集 gentx 并 collect-gentxs，再 start。"
