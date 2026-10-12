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
- 自定义模块查询（路径与 `proto/mcchain/*/query.proto` 的 `google.api.http` 注解一一对应）：
  - `GET /mcchain/phonenode/params`、`/mcchain/phonenode/attestation/{address}`、`/mcchain/phonenode/slashes/{address}`
  - `GET /mcchain/depin/params`
  - `GET /mcchain/edgeai/params`、`/mcchain/edgeai/task/{task_id}`、`/mcchain/edgeai/tasks`、`/mcchain/edgeai/results`、`/mcchain/edgeai/disputes`
  - `GET /mcchain/tokenomics/supply`、`/mcchain/tokenomics/allocations`、`/mcchain/tokenomics/release`
  - `GET /mcchain/liquidstaking/params`、`/mcchain/liquidstaking/pool`、`/mcchain/liquidstaking/unbondings/{delegator}`
  - `GET /mcchain/mcchain/params`、`/mcchain/mcchain/chain_info`
  - 注意：`dex` / `referral` 模块未配置 REST 注解，仅暴露 gRPC（见下）

### 2.1 REST 网关生产配置（Nginx：限流 / TLS / 访问控制）

> Cosmos SDK REST/gRPC-Gateway（1317）**默认无鉴权、无限流**，直接暴露公网存在
> 被滥用（高频查询拖垮节点、UDP 放大、敏感接口被扫描）风险。**主网部署必须在
> 1317 之前放置反向代理**，且默认**只读**——所有写操作（广播交易）走 26657 RPC
> 或带签名的 SDK 客户端，REST 仅承担查询。

```nginx
# /etc/nginx/conf.d/mcchain-rest.conf
upstream mcchain_rest {
    # 可挂多全节点做负载均衡；健康检查由 upstream 自动剔除宕机节点
    server 127.0.0.1:1317 max_fails=3 fail_timeout=10s;
    keepalive 32;
}

# 全局限流区（共享内存）：按 IP 限速
limit_req_zone $binary_remote_addr zone=rest_global:10m rate=30r/s;
# 敏感查询（区块/交易历史）更严
limit_req_zone $binary_remote_addr zone=rest_heavy:10m rate=5r/s;

server {
    listen 443 ssl http2;
    server_name rest.mcchain.example;   # 换成实际域名

    ssl_certificate     /etc/nginx/tls/rest.mcchain.example.crt;
    ssl_certificate_key /etc/nginx/tls/rest.mcchain.example.key;
    ssl_protocols TLSv1.2 TLSv1.3;

    # 安全头
    add_header X-Content-Type-Options nosniff always;
    add_header Cache-Control "no-store" always;

    # 全局限流（默认 30 r/s/IP），超出返回 429
    limit_req zone=rest_global burst=60 nodelay;
    limit_req_status 429;

    location / {
        proxy_pass http://mcchain_rest;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 30s;
    }

    # 重查询单独限速（区块/交易/txs/validators 等大响应）
    location ~ ^/(cosmos/base/tendermint|cosmos/tx|cosmos/staking) {
        limit_req zone=rest_heavy burst=15 nodelay;
        limit_req_status 429;
        proxy_pass http://mcchain_rest;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # 拒绝写类路径（REST 网关只读原则：POST 一律 405）
    location ~* ^/.*\.(tx|broadcast|submit) {
        if ($request_method = POST) { return 405; }
        limit_req zone=rest_heavy burst=15 nodelay;
        proxy_pass http://mcchain_rest;
    }

    # 健康检查端点不限流
    location = /health { proxy_pass http://mcchain_rest; limit_req off; }
}

# HTTP 强制跳转 HTTPS
server {
    listen 80;
    server_name rest.mcchain.example;
    return 301 https://$host$request_uri;
}
```

**说明与进阶**：

- **限流参数**：`rate` 按自身容量测算（单节点全节点约承受 50~100 r/s 轻查询；此处给
  保守值，误伤时调大 `burst` 而非 `rate`）。生产建议按节点数与前端 QPS 基准测试后固化。
- **鉴权**：若 REST 仅内网使用，可在 Nginx 加 `allow 10.0.0.0/8; deny all;` 或
  Basic Auth（`auth_basic`）；对外只读场景可用 API Key 头校验（`if ($http_x_api_key != "...") { return 403; }`）。
- **只读边界**：Cosmos SDK 的 REST 网关仍会转发 `POST /cosmos/tx/v1beta1/simulate` 等
  只读模拟接口，属安全范围；**广播交易必须走 26657 RPC 或 SDK 客户端**，不要把
  1317 的 POST 透传到公网。
- **联动防护**：配合 `docs/DEPLOYMENT_GUIDE.md` 哨兵架构，1317/26657 均只经代理暴露，
  节点本机不直接监听公网。

## 3. gRPC（端口 9090）

启用 reflection（`grpc.reflection.v1alpha.ServerReflection` 已注册），可用 `grpcurl` 直接探查：
```bash
grpcurl -plaintext localhost:9090 list
grpcurl -plaintext localhost:9090 mcchain.phonenode.Query/Attestation
```
主要 Query 服务（pkg 为 `mcchain.<module>`，无 `.v1` 段）：
`cosmos.bank.v1beta1.Query`、`cosmos.tx.v1beta1.Service`、
`mcchain.phonenode.Query`（Params/Attestation/Slashes）、
`mcchain.depin.Query`（Params）、`mcchain.edgeai.Query`（Params/Task/Tasks/Results/Disputes）、
`mcchain.dex.Query`（Pool/Pools/EstimateSwap/Price/LiquidityLock）、
`mcchain.tokenomics.Query`（Supply/Allocations/Release）、
`mcchain.referral.Query`（Referral/ReferralsByInviter/PendingRewards）、
`mcchain.liquidstaking.Query`（Params/Pool/Unbondings）、`mcchain.mcchain.Query`（Params/ChainInfo）。

## 4. Tendermint RPC（端口 26657）

- `GET /status` — 链高度、节点地址、是否同步中
- `GET /broadcast_tx_sync?tx=0x...` — 广播交易（移动端 SDK 用此）
- `GET /abci_query` — 直接查状态
- `GET /health` — 健康

## 5. 事件订阅（链→后端 集成点）

`cmd/event-subscriber` 订阅 MC 链上事件（完整清单见 `docs/sdk_event_contract.md`，共 61 类，可由 `scripts/gen_events_doc.py` 重新生成）。
核心事件：`depin.RewardPaid` / `depin.DeviceAttested`、`phonenode.Attestation` / `phonenode.Slash`、
`edgeai.TaskCreated` / `edgeai.Settled` / `edgeai.DisputeResolved` / `edgeai.RewardPaid`、
`tokenomics.SecurityDripped` / `tokenomics.GasBurned` 等。后端/区块浏览器应以此为索引接入点，而非轮询 RPC。

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
