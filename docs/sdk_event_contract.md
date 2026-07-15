# MobileChain SDK 事件契约（B5-R3/R4）

> 本文档是移动端 SDK 对接的**事件契约**部分，与 `docs/mobile_sdk_integration.md`（gRPC/REST 端点）配套。
> 移动端通过 Tendermint WebSocket 订阅链上事件，实现「挖矿到账提醒、节点被 slash 告警、认证状态变更」等实时 UX，无需轮询。
>
> 底座：Cosmos SDK v0.47.3 + cometbft v0.37.1；链 ID `mcchain-mainnet-1`（主网）/ `mcchain-testnet-1`（测试网）；denom `umc`。

## 1. 事件总览

每个事件都自动带 `message.module`（处理该消息的模块名）与 `message.sender`（交易发起地址）属性，SDK 可按模块过滤。

| 事件名 | 模块 | 触发时机 | 关键属性 |
|---|---|---|---|
| `depin.RewardPaid` | depin | 贡献即挖矿拨付成功（发币到账） | task_id, device, task_type, score, reward, denom |
| `phonenode.Attestation` | phonenode | 节点提交 attestation 成功 | address, nonce, device_id_hash |
| `phonenode.Slash` | phonenode | 节点因离线/作弊被 slash | address, reason, penalty_bps |
| `edgeai.TaskCreated` | edgeai | 创建 EdgeAI 任务 | task_id, creator |
| `edgeai.ResultSubmitted` | edgeai | 提交任务结果 | task_id, submitter |
| `edgeai.DisputeOpened` | edgeai | 开启争议 | task_id, challenger |
| `edgeai.RewardPaid` | edgeai | EdgeAI 任务悲观/乐观结算拨付 | task_id, submitter, amount |
| `edgeai.DisputeResolved` | edgeai | 争议结案 | task_id, resolution |

## 2. 事件属性明细

### 2.1 depin.RewardPaid（最重要 —— 挖矿到账）
| 属性 | 类型 | 说明 |
|---|---|---|
| `task_id` | string | 贡献任务 ID（如 `task-0001`） |
| `device` | bech32 addr | 收到代币的设备/节点地址 |
| `task_type` | string | `inference` / `data_label` / `bandwidth` |
| `score` | string(int) | 贡献评分 [0,100] |
| `reward` | string(uint) | 本次发放 umc 数量（= score × 倍率，封顶 500） |
| `denom` | string | 固定 `umc` |

> 移动端收到该事件后：刷新 `device` 余额、弹出「+<reward/1e6> MC 到账」通知、更新今日挖矿统计。

### 2.2 phonenode.Slash
| 属性 | 类型 | 说明 |
|---|---|---|
| `address` | bech32 addr | 被 slash 节点地址 |
| `reason` | string | `offline` / `cheat_contrib` / `fake_attest` 等 |
| `penalty_bps` | string(int) | 惩罚基点（万分之一）；验证人扣自质押，非验证人仅吊销认证 |

### 2.3 phonenode.Attestation
| 属性 | 类型 | 说明 |
|---|---|---|
| `address` | bech32 addr | 节点地址 |
| `nonce` | string | 防重放 nonce |
| `device_id_hash` | string | 设备身份哈希 |

## 3. 订阅方式（Tendermint WebSocket）

### 3.1 全量 Tx + 链下过滤（推荐，最稳）
```bash
# 连接 ws://<node>:26657/websocket，订阅 tm.event='Tx'
# 链下按事件 Type 过滤 mcEventTypes（见 cmd/event-subscriber/main.go）
```

### 3.2 按模块过滤（可选）
```bash
# 仅 depin 发币事件（cometbft 不支持多类型 OR，按模块过滤后链下再筛）
tm.event='Tx' AND message.module='depin'
```

### 3.3 grpcurl / 原生 gRPC 等价
事件订阅走 Tendermint RPC（26657/websocket），**不走 gRPC 9090**（gRPC 仅暴露 Query/Msg 服务）。移动端用 cometbft 客户端的 `Subscribe`。

## 4. gRPC / REST 端点速查（完整版见 mobile_sdk_integration.md）

| 用途 | 服务 | 协议 |
|---|---|---|
| 注册手机节点 | `mcchain.phonenode.Msg/RegisterNode` | gRPC(9090) |
| 节点认证 | `mcchain.phonenode.Msg/SubmitAttestation` | gRPC(9090) |
| 心跳保活 | `mcchain.phonenode.Msg/SubmitStateProof` | gRPC(9090) |
| 设备注册 | `mcchain.depin.Msg/RegisterDevice` | gRPC(9090) |
| 设备证明 | `mcchain.depin.Msg/AttestDevice` | gRPC(9090) |
| 提交贡献(挖矿) | `mcchain.depin.Msg/SubmitContribution` | gRPC(9090) |
| 查余额 | `mcchain.bank.Query/Balance` 或 `GET /cosmos/bank/v1beta1/balances/{addr}` | gRPC / REST(1317) |
| 查 DePIN 参数 | `mcchain.depin.Query/Params` | gRPC / REST(1317) |

## 5. 移动端事件→UX 映射示例

```
depin.RewardPaid     → 余额卡片 +<reward>umc，推送「挖矿到账」
phonenode.Slash      → 红色告警「节点被处罚(<reason>)」，提示检查心跳
phonenode.Attestation→ 节点状态变为「已认证」
edgeai.DisputeOpened → 「任务<task_id>被质疑，结算暂停」
edgeai.RewardPaid    → EdgeAI 任务结算到账提示
```

## 6. 参考实现
- 链下订阅最小实现：`cmd/event-subscriber/main.go`（Go，连接任意 RPC，打印上述事件）。
- 构建：`go run ./cmd/event-subscriber http://<node>:26657`
