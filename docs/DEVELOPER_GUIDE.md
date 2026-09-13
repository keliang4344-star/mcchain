# 开发者指南（DEVELOPER_GUIDE）

从「知道 MC 公链是什么」到「在上面写一个能跑的 dApp」需要的全部工程信息。

## 1. 五个接入入口

| 入口 | 适用 | 端口/路径 | 文档 |
|---|---|---|---|
| JSON-RPC（CometBFT） | 区块 / 交易查询 | 26657/tcp | `docs/sdk_event_contract.md` |
| REST（LCD） | 浏览器 / 低频业务 | 1317/tcp | `docs/BACKEND_API.md` |
| gRPC（Protobuf） | 后端服务、高频 | 9090/tcp | `docs/BACKEND_API.md` |
| gRPC-Web | 浏览器直连 | 9091/tcp | — |
| WebSocket | 订阅事件 / 新块 | 26657/websocket | `docs/sdk_event_contract.md` |

## 2. 地址体系

| 用途 | bech32 前缀 | 示例 |
|---|---|---|
| 用户账户 | `mc` | `mc1abc...` |
| 验证人操作地址 | `mcvaloper` | `mcvaloper1abc...` |
| 验证人共识公钥 | `mcvalcons` | `mcvalcons1abc...` |

私钥派生路径（与 Cosmos 兼容）：
```
m/44'/118'/0'/0/0     # 默认
```

> HDCoinType 118 来自 Cosmos Hub，与现有硬件钱包（Keplr / Ledger）兼容。

## 3. 最小 dApp：30 行转账

### 3.1 CLI
```bash
mcchaind tx bank send <from-key> <to-addr> 1000000umc \
    --chain-id mcchain-mainnet-1 \
    --node https://rpc.mcchain.io:443 \
    --gas auto --gas-adjustment 1.5 \
    --fees 5000umc -y
```

### 3.2 REST
```bash
curl -X POST https://api.mcchain.io/cosmos/bank/v1beta1/send \
    -H "Content-Type: application/json" \
    -d '{
      "from_address": "mc1...",
      "to_address":   "mc1...",
      "amount":       [{"denom":"umc","amount":"1000000"}],
      "mode":         "BROADCAST_MODE_SYNC"
    }' | jq .
```

### 3.3 gRPC（Node.js / @grpc/grpc-js）

```javascript
import * as proto from '@bufbuild/protobuf';
import { createPromiseClient } from '@bufbuild/grpc';
import { CosmosBank } from 'mcchain-sdk-ts/gen/cosmos/bank/v1beta1';

const client = createPromiseClient(CosmosBank.BankService, transport);
const resp = await client.send({ /* ... */ });
```

详细 SDK 使用见 `typescript-sdk/README.md`（如果已发布）；或在迁移期使用
`@cosmjs/stargate` 配 `mcchain-proto`。

## 5. 常见查询速查

```bash
# 当前区块高度
mcchaind status 2>&1 | jq .SyncInfo.latest_block_height

# 任意账户余额
mcchaind query bank balances <mc_addr>

# 历史交易
mcchaind query tx <tx-hash>

# 按事件过滤
mcchaind query txs --events 'transfer.sender=mc1abc' --limit 50

# 治理提案列表
mcchaind query gov proposals --status voting_period
```

## 6. 订阅事件（WebSocket）

```javascript
import WebSocket from 'ws';
const ws = new WebSocket('wss://rpc.mcchain.io:443/websocket');
ws.on('open', () => ws.send(JSON.stringify({
  jsonrpc: '2.0', method: 'subscribe',
  id: '1', params: { query: "tm.event='NewBlock'" }
})));
ws.on('message', m => console.log(m.toString()));
```

事件全集见 `docs/sdk_event_contract.md`。

## 7. Gas 与手续费

- 最低 gas 价：`0.0001 umc`（主网）。
- 单 tx 推荐起步 gas：200,000。
- 默认 `--gas auto --gas-adjustment 1.5` 已足够；复杂 msg 用 2.0。
- 总费用 = gas_used × gas_price；不达最低价 tx 会被丢弃。
- **gas 7% 销毁 + 10% 返利**机制自动生效，不需要 dApp 配合。

## 8. 测试方法

```bash
# 1) 起一条本地 devnet
./scripts/devnet.sh init 3
./scripts/devnet.sh start

# 2) 等高度 > 5，做一笔转账测试
./scripts/devnet.sh fund <to-addr> 1000000

# 3) 跑 SDK 自带的 E2E
cd typescript-sdk && pnpm install && pnpm test:e2e
```

## 9. 已知陷阱（FAQ）

- **「tx 一直 Pending」**：常见原因是 `--gas auto` 在旧版本里估算偏低；
  显式 `--gas 300000` 重试。
- **「transaction not found」**：确认 chain-id；不同网络的 tx 哈希空间互不干扰。
- **「insufficient fees」**：检查 denoms；很多浏览器默认填的是 `ATOM` 等。
- **「account sequence mismatch」**：本地缓存的 sequence 过期，重 query account 一次。

## 10. 工具与脚本

| 工具 | 用途 |
|---|---|
| `mcchaind` | 全功能节点 + CLI |
| `scripts/devnet.sh` | 本地 3 节点一键 devnet |
| `scripts/bench.sh` | 性能基准 |
| `cmd/indexer/` | 链上事件索引器（自建 explorer 用） |
| `cmd/oracle/` | TEE 预言机服务端 |

## 11. 出错反馈

- SDK / dApp bug → 仓库 issue（链无关）
- 链节点 bug → 公开治理论坛 + `internal/initguard/` 触发的 panic stack
- 安全披露 → `security@mcchain.io`（白皮书安全章节的联系方式）