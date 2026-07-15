# MobileChain 移动端 SDK 对接文档

> 范围界定（Q7）：本文档为**轻量对接文档**，不新建独立 SDK 工程。移动端（手机即节点即设备）通过 **gRPC / REST** 直接调用 `mcchaind` 暴露的 `depin` 与 `phonenode` 模块服务即可完成节点注册、设备注册/证明、贡献提交与余额查询。
>
> 底座：Cosmos SDK v0.47.3 + cometbft v0.37.1；链 ID `mcchain-mainnet-1`；地址前缀 `mc`；**统一 denom = `umc`**（1 MC = 1e6 umc）。

## 1. 通用约定

| 项 | 值 |
|---|---|
| 链 ID | `mcchain-mainnet-1` |
| 地址前缀 | `mc`（账户/验证人：`mc...`；模块账户同样 `mc...`） |
| 基础 denom | `umc`（微 MC）。奖励、质押、转账均用 `umc`，**不存在 `stake`** |
| 代币换算 | 100k MC = `100000000000` umc（1e11 umc）；DePIN 初始池默认 `1e14` umc（=1e8 MC） |
| gRPC 端口 | 默认 `9090`（app.toml `grpc.enable = true`） |
| REST(grpc-gateway) 端口 | 默认 `1317`（app.toml `api.enable = true`） |

> 端点注册已由 Ignite 脚手架生成（`x/depin/module.go`、`x/phonenode/module.go` 的 `RegisterServices` / `RegisterGRPCGatewayRoutes`）。Query 服务带有 `google.api.http` 注解，因此**同时**暴露 gRPC 与 REST；Msg 服务**仅 gRPC**（无 HTTP 注解，无 REST 路由），移动端请用 gRPC 客户端或 `mcchaind tx` CLI 发交易。

## 2. gRPC / REST 端点清单

### 2.1 depin 模块（`mcchain.depin`）

| 类型 | gRPC 全名 | REST |
|---|---|---|
| Query | `mcchain.depin.Query/Params` | `GET /mcchain/depin/params` |
| Msg | `mcchain.depin.Msg/RegisterDevice` | （gRPC only） |
| Msg | `mcchain.depin.Msg/AttestDevice` | （gRPC only） |
| Msg | `mcchain.depin.Msg/SubmitContribution` | （gRPC only） |

> 说明：`device` / `contribution` 等明细**当前未**作为 gRPC Query 暴露（仅 `Params` 查询）。如需按设备/任务查询状态，可后续在 `proto/mcchain/depin/query.proto` 增加 RPC 并通过 `ignite chain build` 重新生成；本增量聚焦参数与拨付闭环。

### 2.2 phonenode 模块（`mcchain.phonenode`）

| 类型 | gRPC 全名 | REST |
|---|---|---|
| Query | `mcchain.phonenode.Query/Params` | `GET /mcchain/phonenode/params` |
| Msg | `mcchain.phonenode.Msg/RegisterNode` | （gRPC only） |
| Msg | `mcchain.phonenode.Msg/SubmitStateProof` | （gRPC only） |

## 3. 端到端流程（移动端）

```
1) MsgRegisterNode    (phonenode)  手机注册为移动全节点（Address = 设备地址）
2) MsgRegisterDevice  (depin)      设备注册（可选 attestation 前置）
3) MsgAttestDevice    (depin)      设备证明（提交 challenge + signature）
4) MsgSubmitContribution (depin)   提交贡献 → 计奖 → 发 umc（前提是步骤1已注册）
```

> **关联校验（P2/Q5/Q6）**：`MsgSubmitContribution` 的发币闸口要求 `Creator`（= 设备地址）已通过 `MsgRegisterNode` 注册为 phonenode 节点（关联键 = 节点 `Address`）。未注册的设备提交有效贡献（reward>0）会返回错误 `ErrPhonenodeNotRegistered`（code 1107），**不发币**；已注册则正常发放 `umc`。

## 4. 交易（Msg）示例

> 字段名来自 `proto/mcchain/{depin,phonenode}/tx.proto`。gRPC 调用可用 `grpcurl`；或本机用 `mcchaind tx ... --from <key> --chain-id mcchain-mainnet-1 --fees <x>umc -y`。

### 4.1 MsgRegisterNode（phonenode）

```json
{
  "creator": "mc1...device_or_operator...",
  "address": "mc1...node_address...",   // 节点 Address == 后续 depin 设备地址
  "model":   "pixel-8",
  "os":      "android-14",
  "role":    "validator-edge"
}
```

grpcurl 示例：

```bash
grpcurl -plaintext -d '{
  "creator": "mc1...", "address": "mc1...",
  "model": "pixel-8", "os": "android-14", "role": "validator-edge"
}' localhost:9090 mcchain.phonenode.Msg/RegisterNode
```

### 4.2 MsgRegisterDevice（depin）

```json
{
  "creator": "mc1...",
  "address": "mc1...device_address...",
  "model":   "sensor-x",
  "os":      "rtos-2.3"
}
```

### 4.3 MsgAttestDevice（depin）

```json
{
  "creator": "mc1...",
  "address": "mc1...device_address...",
  "challenge": "base64-challenge",
  "signature": "base64-signature"
}
```

### 4.4 MsgSubmitContribution（depin）

```json
{
  "creator":  "mc1...device_address...",  // 必须 == phonenode 已注册节点 Address
  "taskId":   "task-0001",
  "taskType": "inference",                // inference | data_label | bandwidth
  "score":    "80"                         // 字符串，[0,100]；>=30 才有奖励
}
```

奖励公式（链上 `x/depin/keeper/reward.go`）：`reward = score * RewardRate(taskType)`，封顶 `MaxRewardPerTask=500`；`score<30` 或非法类型 → `reward=0`（不发币）。`inference=5x, data_label=3x, bandwidth=1x`。

发币示例（奖励成功时）：设备地址收到 `umc`，denom 来自 `Params.RewardDenom`（默认 `umc`）：

```bash
# 查询设备余额
mcchaind q bank balances mc1...device_address...
# => 应出现 <reward>umc 增量
```

## 5. 查询（Query）示例

### 5.1 DePIN 参数（含初始池与奖励 denom）

```bash
# REST
curl http://localhost:1317/mcchain/depin/params

# gRPC
grpcurl -plaintext localhost:9090 mcchain.depin.Query/Params
```

返回（JSON 近似）：

```json
{
  "params": {
    "initial_pool": "100000000000000",
    "reward_denom": "umc"
  }
}
```

### 5.2 phonenode 参数

```bash
curl http://localhost:1317/mcchain/phonenode/params
grpcurl -plaintext localhost:9090 mcchain.phonenode.Query/Params
```

### 5.3 模块账户总览（确认 DePIN 池已铸入）

```bash
mcchaind q bank total
# 或仅看 depin 模块账户
mcchaind q bank balances $(mcchaind q auth module-address depin -o json | jq -r .address)
# 初始应有 initial_pool (=1e14) umc
```

## 6. 移动端接入提示

1. 使用任意 gRPC 客户端（grpcurl / grpc-web / 原生 gRPC）。Msg 服务走 gRPC（9090）；Query 可走 REST（1317）或 gRPC。
2. 交易需先 `mcchaind keys add` 导入/创建账户，并用 `umc` 支付 fees（若启用）。
3. **务必先 `MsgRegisterNode` 再用同一地址 `MsgSubmitContribution`**，否则贡献计奖成功但发币被 `ErrPhonenodeNotRegistered` 拒绝。
4. denom 一律 `umc`；序列化/展示时金额均为整数 umc（如 `100000000000` 表示 100k MC）。
5. 出块间隔默认 4s（`config/config.toml` 的 `timeout_commit`，见 `docs/runbook.md`）。
