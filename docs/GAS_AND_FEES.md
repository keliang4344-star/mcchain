# MobileChain Gas 与费用策略

## 1. 现状与默认

- 链上 `denom = umc`（1 MC = 1e6 umc），代币固定上限 1e15 umc，零二次通胀（mint 模块通胀已清零）。
- 节点 `minimum-gas-prices` 默认值由 `deploy/init.sh` 设为 **`0umc`**（移动端 0 手续费挖矿友好）。
  - 生产主网建议设 `MIN_GAS_PRICES=0.0025umc` 后重跑 `init.sh`，以抵御 mempool spam。
- 移动端矿工（`mc-miner` App）当前用固定 `gas: '200000'`、空 `amount` 的费用结构，兼容 0 手续费场景。

## 2. 手续费如何计算

```
fee = gas * min_gas_price
```
- 当 `minimum-gas-prices = "0umc"`：任意交易（含空 `amount`）均可通过 ante 校验。
- 当 `minimum-gas-prices = "0.0025umc"`：交易须带 `fee.amount` ≥ `gas * 0.0025umc`，
  否则 `checkTx` 拒绝（错误 `insufficient fees`）。

## 3. 移动端 SDK / 钱包接出建议

- **联调/测试网**：保持 `0umc`，矿工用 `fee = {amount: [], gas: '200000'}` 即可广播。
- **主网（防 spam）**：钱包在构造交易时 `simulate` 得到 gas，再乘 `0.0025umc` 作为 `fee.amount`，
  例如 `gas=200000 → fee=500umc`。
- 费用接收方：Cosmos SDK 默认将手续费焚烧或归入区块 proposer 奖励池（取决于 ante 配置）；
  本链未做特殊手续费分账，保持 SDK 默认（进入 proposer 奖励）。

## 4. Gas 上限与DoS 防护

- 单笔交易 `gas` 上限由 `app.toml` 的 `max_gas` 控制（默认 0 = 不限）；生产建议设合理上限（如 5e6）。
- 移动端交易均为轻量 Msg（注册/ attest / 贡献），`gas=200000` 充足；如引入复杂 Msg 需重新评估。
- 配合 `minimum-gas-prices` 非零值，可有效阻止空投/刷量类 spam。

## 5. 运维切换

```bash
# 主网：用非零最小 gas 价重新初始化
MIN_GAS_PRICES=0.0025umc CHAIN_ID=mcchain-mainnet-1 bash deploy/init.sh

# 临时调整已运行节点：改 app.toml 的 minimum-gas-prices 后重启
```

> 注意：修改 `minimum-gas-prices` 会影响所有新交易；切换前务必公告，避免旧版 0 手续费钱包广播失败。
