# 跨链（IBC）对接指南

MC 公链原生支持 Cosmos IBC（inter-blockchain communication）。
本节是「上线一条新链 → 与 MC 互通」的工程速查，不是协议设计文档。

## 1. 当前开启的功能

| 功能 | 状态 | 备注 |
|---|---|---|
| IBC transfer（ICS-20） | ✅ | `x/ibc-transfer`（bank 包装） |
| Interchain Accounts（ICA）controller | ✅ | 允许在 MC 上控制别的链上的账户 |
| Interchain Accounts（ICA）host | ✅ | 允许别的链在 MC 上开账户 |
| IBC classic light client（07-tendermint） | ✅ | 用于非 IBC-native 链 |
| Localhost client（09-localhost） | ✅ | 仅测试网 |
| Solomachine（06-solomachine） | ❌ | SDK 自带但不在白皮书承诺内 |
| IBC-hooks / IBC middleware | ❌ | 暂未启用，避免无端面扩攻击面 |

## 2. 上限设置（mainnet）

```json
{
  "client_state": {
    "trusting_period":   "1209600s",   // 14 天（与 cosmoshub 一致）
    "unbonding_period":  "1814400s",   // 21 天（与 staking 对齐）
    "max_clock_drift":   "10s",        // 与 cosmoshub 一致
    "allowed_clients":   ["07-tendermint", "09-localhost"]
  },
  "connection_params": {
    "max_expected_time_per_block": "30000000000"  // 30s
  },
  "channel_params": {
    "max_msg_size": "200000"           // 单条 msg ≤ 200 KB
  },
  "transfer_params": {
    "send_enabled":    true,
    "receive_enabled": true
  }
}
```

这些值在创世时由 `scripts/make_genesis.py` 一次性写入；
事后需要改的话是治理提案（`x/ibc.MsgUpdateParams`）。

## 3. 端到端连通测试（E2E）

### 3.1 准备一条对端 devnet

```bash
# 启动 MC devnet
./scripts/devnet.sh clean
./scripts/devnet.sh init 2

# 启动对端（这里假设是 gaia 或别的 cosmos 链）
<对端链> init ...
<对端链> start
```

### 3.2 在两端建客户端 / 连接 / 通道

```bash
# 1) 在 MC 上记下 client / connection / channel ID
mcchaind query ibc client states  --node http://127.0.0.1:26657

# 2) 握手：以 cosmoshub 测试网为例
hermes --config ~/.hermes/config.toml create client \
    --host-chain mcchain-dev-1 --counterparty-chain cosmoshub-test

hermes create connection \
    --a-chain mcchain-dev-1 --a-client 07-tendermint-0 \
    --b-chain cosmoshub-test --b-client 07-tendermint-0

hermes create channel \
    --a-chain mcchain-dev-1 --a-connection connection-0 \
    --a-port transfer --b-port transfer --order unordered
```

### 3.3 转账 E2E

```bash
# MC → cosmoshub
hermes transfer \
    --source-chain mcchain-dev-1 \
    --target-chain cosmoshub-test \
    --recipient <cosmoshub_addr> \
    --amount 1000000umc

# cosmoshub → MC
hermes transfer \
    --source-chain cosmoshub-test \
    --target-chain mcchain-dev-1 \
    --recipient <mc_addr> \
    --amount 1000000uatom
```

### 3.4 失败定位清单

| 症状 | 大概率原因 | 怎么查 |
|---|---|---|
| `connection_not_found` | handshake 没走完 | `hermes query connections --chain X` |
| `packet timeout` | trusting_period 太短 / 对端被挡 | 看 timeout_height vs 实际高度 |
| `invalid proof` | 对端轻客户端被罚 / 信任期到期 | 重做 create client |
| `ack error` | 对端不支持 transfer（业务层拒绝） | 看对端事件 |

## 4. ICA（Interchain Accounts）用法

### 4.1 在 MC 上注册 controller 账户

```bash
mcchaind tx interchainaccounts controller register \
    --connection-id connection-0 \
    --from <key> -y
```

成功后查询：
```bash
mcchaind query interchainaccounts controller interchain-account <mc_addr> --connection connection-0
```

### 4.2 用 ICA 在对端发起 tx

```bash
mcchaind tx interchainaccounts controller send-tx \
    --connection-id connection-0 \
    --ica-account <ica_addr> \
    --msgs '{"@type":"/cosmos.bank.v1beta1.MsgSend",...}' \
    --from <key> -y
```

## 5. 监控与告警

跨链的告警都挂在 `docs/ALERTS.md` 的 `chain_invariants` 组里：

- IBC 客户端超过 `trusting_period / 2` 没更新 → 告警（轻客户端即将失效）。
- 通道里 pending packets 超过 100 → 告警（relay 中断）。
- ICA 控制器创建账户失败 → 告警（连接已断）。

## 6. 安全边界

- **不接受 06-solomachine 客户端**（ICS-07 之外的高风险客户端，移到白名单外）。
- **transfer 通道只开给业务需要的链**，不「链链通」（每多一条通道 = 新的信任边界）。
- **ICA controller 账户权限范围** = 在通道上创建后即不可降级。需要降权只能关通道、重开。