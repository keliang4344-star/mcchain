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

---

## 6. DEX 交易 Gas 与费率

### 6.1 DEX Swap 消息的 Gas 消耗

DEX 的 swap 消息（`MsgSwap`）涉及 AMM 常量积计算、流动性池状态更新、代币转入转出等链上操作，Gas 消耗明显高于普通转账：

| 消息类型 | 典型 Gas | 说明 |
|---|---|---|
| `MsgSend`（普通转账） | ~80,000 | 单一 bank 模块转账 |
| `MsgSwap`（DEX 兑换） | ~180,000–250,000 | 含 AMM 计算 + 双币种流转 |
| `MsgAddLiquidity`（添加流动性） | ~200,000–280,000 | 含 LP 代币铸造 |
| `MsgRemoveLiquidity`（移除流动性） | ~180,000–250,000 | 含 LP 代币销毁 + 双币返还 |

> 以上为预估值，实际 Gas 取决于池子状态与交易规模。建议钱包在构造 DEX 交易时**先 `simulate` 获得精确 Gas**，避免因 Gas 不足导致交易失败。

### 6.2 DEX 手续费分配

DEX 每笔 swap 收取 **0.3% 协议手续费**（protocol fee，在 AMM 常量积公式中自动扣除），分配规则如下：

| 分配方向 | 比例 | 说明 |
|---|---|---|
| **Burn（销毁）** | 50% | 直接销毁，永久退出流通，通缩效应 |
| **Treasury（国库）** | 30% | 进入社区金库，由 DAO 治理支配 |
| **LP（流动性提供者）** | 20% | 按 LP 代币持有比例分配给该池的 LP |

### 6.3 与普通转账费用的区别

| 对比维度 | 普通转账 (`MsgSend`) | DEX Swap |
|---|---|---|
| Gas 费（链上） | 仅 Cosmos SDK 标准 Gas 费 | Gas 费更高（约 2-3 倍） |
| 协议费（额外） | 无 | 0.3% swap fee（从兑换金额中扣除） |
| 费用去向 | 进入区块 Proposer 奖励池 | 50% 销毁 + 30% 国库 + 20% LP |
| 对总量的影响 | 无（Gas 费在系统内流转） | 有（50% 销毁减少流通量） |

> **关键区别**：普通转账只支付 Gas 费给验证人作为出块奖励；DEX swap 除 Gas 费外，还需额外支付 0.3% 协议费，其中一半被永久销毁，实现链上通缩。
