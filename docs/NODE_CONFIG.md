# 节点配置模板（NODE_CONFIG）

本文档给出 MC 公链节点在四种典型部署形态下的**完整可复制**配置模板，
所有参数都和 `docs/PROTOCOL_PARAMS.md`、`scripts/make_genesis.py`、`scripts/devnet.sh`
保持同一套口径。复制后只需改 `moniker` / `external_address` / 持久对等即可上线。

## 0. 公用参数（所有形态都要遵守）

```toml
# config.toml（CometBFT）
timeout_commit = "4s"               # 主网标准出块间隔
moniker         = "your-moniker"
persistent_peers = "id1@ip1:26656,id2@ip2:26656,..."
addr_book_strict = false            # 多 IP / NAT 环境下必须 false
allow_duplicate_ip = false
prometheus      = true              # 暴露 :26660/metrics，给告警抓取
pex             = true              # 主网打开；私有网（如 devnet）建议 false
```

```toml
# app.toml（应用层）
minimum-gas-prices = "0.0001umc"    # 主网下限；私有网可用 0umc
pruning             = "custom"
pruning-keep-recent = "362880"      # ≈3 天（21,600 块 × 16.8）
pruning-interval    = "100"
api                 = { enable = true, address = "tcp://0.0.0.0:1317" }
grpc                = { enable = true, address = "0.0.0.0:9090" }
grpc-web            = { enable = true, address = "0.0.0.0:9091" }
state-sync          = { snapshot-interval = 1000, snapshot-keep-recent = 2 }
```

> Devnet 用 `pruning = "nothing"`（scripts/devnet.sh 内置），便于反复调试。
> 生产 mainnet **必须** `custom` + 上面三档，不然 IAVL 膨胀导致 OOM。

## 1. 主网验证人（Signing Validator）

硬件建议：4 核 / 8 GB RAM / ≥500 GB NVMe SSD / 静态 IP / DDoS 防护。

```toml
# config.toml
[p2p]
seed_mode   = false
max_num_inbound_peers  = 40
max_num_outbound_peers = 10

[rpc]
laddr = "tcp://127.0.0.1:26657"     # 仅本机；不要对外
# 反向代理建议 nginx + TLS；模板见 docs/DEPLOYMENT_GUIDE.md

[consensus]
double_sign_check_height = 10        # 10 块窗口检测双签
```

**必做**：
1. `tmkms` 或 `horcrux` 管理签名私钥（不要用文件 keyring 上主网）。
2. 配置 `sentry` 节点隔离 P2P（见下文）。
3. 监控 `mcchaind comet show-node-id`、`mcchaind query slashing signing-info <consPubKey>`。

## 2. Sentry 节点（CDN / 防火墙保护层）

Sentry 是验证人的「门神」，把验证人藏在它后面。
**Sentry 本身不签块**，只负责 P2P 转发；它的 RPC/REST 也对外暴露。

```toml
# config.toml
persistent_peers = "<validator-node-id>@<validator-internal-ip>:26656"
private_peer_ids = "<validator-node-id>"
pex              = true
```

**反向代理**（nginx，节选）：
```nginx
upstream validator_rpc { server 10.0.0.10:26657; keepalive 32; }
server {
  listen 443 ssl http2;
  server_name rpc.your-domain.com;

  location / {
    proxy_pass http://validator_rpc;
    proxy_set_header X-Forwarded-For $remote_addr;
    proxy_read_timeout 60s;
  }
}
```

## 3. 全节点 / RPC Provider（对外服务查询 + gRPC）

不签块，只同步链 + 提供查询。可水平扩展（多实例）。

```toml
# config.toml
[rpc]
laddr = "tcp://0.0.0.0:26657"
max_open_connections = 600

[grpc]
enable = true

# 限制每 IP 的 RPC 速率（防滥用）
[rpc-rate-limit]
max-throughput = 50000    # bytes/sec
interval       = 1s
global-cap     = 50000
```

**state-sync 加速首轮同步**（10 分钟级而非数天）：
```toml
[statesync]
enable      = true
rpc_servers = "https://rpc1.your-domain.com:443,https://rpc2.your-domain.com:443"
trust_height   = 1000000           # 信任高度，最近一周内
trust_hash     = "ABCDEF..."       # 对应区块 hash
trust_period   = "168h0m0s"        # 信任期 7 天
```

## 4. 归档节点（Archive Node）

给 explorer / indexer / 监管审计用，保留全部历史状态。

```toml
# app.toml
pruning              = "nothing"
min-retain-blocks    = 0
disable-fast-node    = true          # 关闭 IAVL 快速节点以保留所有版本
```

硬件建议：8 核 / 32 GB RAM / ≥4 TB NVMe（IAVL 增长非线性）。

## 5. 出块参数清单（生产必对齐）

| 参数 | 主网值 | 出处 |
|---|---|---|
| `timeout_commit` | `4s` | `docs/PROTOCOL_PARAMS.md` |
| `block.max_gas`  | `100000000` | `scripts/make_genesis.py` |
| `evidence.max_age` | `362880s` | `scripts/make_genesis.py`（21600 × 16.8 ≈ 4 天） |
| `slashing.signed_blocks_window` | `21600` | 1 天（21600 × 4s） |
| `slashing.downtime_jail_duration` | `600s` | 10 分钟 |
| `slashing.slash_fraction_downtime` | `0.01` | 1% |
| `staking.unbonding_time` | `1814400s` | 21 天 |
| `staking.max_validators` | `100` | 启动期硬顶 |

## 6. 配置自检脚本

把下列命令贴进上线 SOP，确认 5 项关键参数没有被 SDK 默认值覆盖：

```bash
mcchaind query slashing params --node tcp://localhost:26657
mcchaind query staking params   --node tcp://localhost:26657
mcchaind query consensus params --node tcp://localhost:26657
mcchaind query mint params      --node tcp://localhost:26657
curl -s localhost:26657/consensus_params | jq '.result.consensus_params.block.max_gas'
```

期望值：
- `signed_blocks_window` = `"21600"`，`downtime_jail_duration` = `"600s"`
- `unbonding_time` = `"1814400s"`，`max_validators` = `"100"`
- `inflation_min`/`max`/`rate_change` 全为 `"0.000000000000000000"`
- `block.max_gas` = `"100000000"`（不是默认 `-1`「不限」）

任一项不符就停下来排查：devnet 已经用同一套 patch_genesis 校准，
如果 production genesis 跑出来不一样，说明 deploy 流程里某一步在裸 mcchaind init
之后没接 patch_genesis。**这是上线 SLA 中的必查项。**