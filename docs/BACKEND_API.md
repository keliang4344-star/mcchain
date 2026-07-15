# MobileChain 后端 API 与接入点

本文档列出主网节点对外暴露的全部接口，供区块浏览器、钱包、移动端 SDK 与监控集成使用。
所有端口已在 `docker-compose.yml` 中映射；裸机部署时按 `config.toml` / `app.toml` 自行开放。

## 1. 端口总览

| 端口 | 协议 | 用途 | 是否需公网 |
|---|---|---|---|
| 26656 | TCP (P2P) | 节点间共识与区块同步 | 是（验证人/全节点） |
| 26657 | HTTP (RPC) | Tendermint RPC（状态/广播/ABCI 查询） | 是（钱包/前端连此） |
| 1317  | HTTP (REST) | Cosmos SDK REST/gRPC-Gateway | 是（前端/钱包） |
| 9090  | gRPC | 原生 gRPC 查询（含 reflection） | 建议内网/带鉴权 |
| 26660 | HTTP (Prometheus) | cometbft 指标 | 否（仅监控） |
| 26661 | HTTP (Prometheus) | cosmos-sdk app 指标 | 否（仅监控） |

> 钱包与移动端 SDK 默认连 **26657（RPC）** 或 **1317（REST）**；生产建议在 1317 前加反向代理（Nginx）做 TLS 与限流。

## 2. REST API（端口 1317，grpc-gateway）

基础前缀 `/cosmos/` 与自定义模块前缀 `/mcchain.*`。常用：

- `GET /cosmos/bank/v1beta1/balances/{address}` — 查询余额（umc）
- `GET /cosmos/staking/v1beta1/validators` — 验证人列表
- `GET /cosmos/staking/v1beta1/validators/{val_addr}/delegations` — 委托
- `GET /cosmos/tx/v1beta1/txs/{hash}` — 交易详情
- `POST /cosmos/tx/v1beta1/simulate` — 手续费模拟
- `GET /cosmos/base/tendermint/v1beta1/node_info` — 节点/链信息
- `GET /cosmos/base/tendermint/v1beta1/blocks/latest` — 最新区块
- 自定义模块查询（需 grpc-gateway 注册）：
  - `GET /mcchain/phonenode/v1/node/{address}` — 手机节点状态
  - `GET /mcchain/depin/v1/device/{address}` — 设备 attestation 状态
  - `GET /mcchain/edgeai/v1/task/{task_id}` — EdgeAI 任务与拨付结果
  - `GET /mcchain/tokenomics/v1/params` — 代币上限/分配

## 3. gRPC（端口 9090）

启用 reflection（`grpc.reflection.v1alpha.ServerReflection` 已注册），可用 `grpcurl` 直接探查：
```bash
grpcurl -plaintext localhost:9090 list
grpcurl -plaintext localhost:9090 mcchain.phonenode.v1.Query/Node
```
主要 Query 服务：`cosmos.bank.v1beta1.Query`、`cosmos.tx.v1beta1.Service`、
`mcchain.phonenode.v1.Query`、`mcchain.depin.v1.Query`、`mcchain.edgeai.v1.Query`、
`mcchain.tokenomics.v1.Query`。

## 4. Tendermint RPC（端口 26657）

- `GET /status` — 链高度、节点地址、是否同步中
- `GET /broadcast_tx_sync?tx=0x...` — 广播交易（移动端 SDK 用此）
- `GET /abci_query` — 直接查状态
- `GET /health` — 健康

## 5. 事件订阅（链→后端 集成点）

`cmd/event-subscriber` 订阅 8 类 MC 链上事件（见 `docs/sdk_event_contract.md`）：
`RewardPaid`(depin)、`NodeRegistered`/`DeviceAttested`(phonenode)、`TaskOpened`/`TaskResolved`(edgeai)、
`TokenMinted`(tokenomics) 等。后端/区块浏览器应以此为索引接入点，而非轮询 RPC。

事件契约：`docs/sdk_event_contract.md`
移动端 SDK 字段约定：`docs/mobile_sdk_integration.md`

## 6. 链下预言机（T2 生产 attestation 闭环）

- 服务：`mcchaind oracle` 或独立 `cmd/oracle` 二进制（见 `internal/oraclesvc`）。
- 接口：
  - `GET /pubkey` → 返回预言机地址与 `pubkey_base64`（注入验证人 `MC_ORACLE_PUBKEY` 启用 TeeOracle）。
  - `POST /sign` `{device_addr, challenge}` → 返回 `signature_base64`（对 `deviceAddr|challenge` 的 secp256k1 签名）。
- 部署：建议与验证人同内网，仅暴露给设备中转层；密钥用 `ORACLE_KEY` 固定。

## 7. 监控（端口 26660/26661/9090/9093/3000）

见 `monitoring/` 与 `docker-compose.yml`：Prometheus 抓取链指标，Grafana 看板，
Alertmanager 在出块停止/对等节点过少/预言机宕机时告警。
