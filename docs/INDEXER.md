# 自建索引器（INDEXER）

MC 公链节点自带 tx_index（CometBFT 自带的 KV 索引），够用；
但要建 explorer / dashboard / 风控大盘，需要一个**专用的事件索引器**。
本文档给出 MC 官方维护的最小可用实现 `cmd/indexer/`，可在此基础上扩展。

## 1. 设计目标

| 优先级 | 目标 | 怎么实现 |
|---|---|---|
| 0 | 不影响节点（不抢节点资源） | 独立进程，只读访问节点 RPC |
| 0 | 不丢块（最多回放一次） | 高水位 + ack 重连 |
| 1 | 任意时刻可重启 | 从最后持久化的高度继续 |
| 1 | 简单存储后端 | 默认 SQLite（单机）/ Postgres（多机） |
| 2 | 可插拔 event 适配器 | 实现一个 EventSink 接口即可接入 |

## 2. 用法

### 2.1 启动

```bash
# 编译（与 mcchaind 同源）
go build -o build/indexer ./cmd/indexer

# 启动：连本节点，写 SQLite
./build/indexer \
    --node http://127.0.0.1:26657 \
    --db /var/lib/mcchain/indexer.db \
    --start-height 1
```

### 2.2 查询（CLI 子命令）

```bash
./build/indexer query \
    --db /var/lib/mcchain/indexer.db \
    --type transfer \
    --sender mc1abc... \
    --from-time 2026-09-01T00:00:00Z \
    --limit 50
```

返回 JSON 行数组。

## 3. 存储 schema（v1）

### 默认后端：JSONL（append-only）

```text
/var/lib/mcchain/indexer/
  blocks.jsonl    # {"height":1,"hash":"...","time":..,"proposer":"...","tx_count":..}
  events.jsonl    # {"height":1,"tx_hash":"...","type":"transfer","attrs":{"k":"v",...}}
  cursor          # 单行文本：最后写入的高度
```

JSONL 的好处：
- 无外部依赖（不需要 CGO / 数据库驱动），单文件即「数据库」；
- 离线分析直接 `jq .` / `awk` / 灌进 pandas；
- 故障时 `head -n 1000 blocks.jsonl` 看现场。

如需 Postgres / ClickHouse，请实现 `EventSink` 接口（见 §7）。

### 可选 SQLite 后端（更结构化）

```sql
CREATE TABLE blocks (
    height      INTEGER PRIMARY KEY,
    hash      TEXT NOT NULL,
    time       INTEGER NOT NULL,           -- unix second
    proposer   TEXT NOT NULL,
    tx_count   INTEGER NOT NULL
);

CREATE TABLE events (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    height        INTEGER NOT NULL,
    tx_hash       TEXT,                     -- 空 = 块级事件
    type          TEXT NOT NULL,            -- e.g. 'transfer', 'message', 'coin_received'
    attr_key      TEXT NOT NULL,
    attr_value    TEXT,
    attr_idx      INTEGER NOT NULL,         -- 同 height+tx+type 内序号
    UNIQUE (height, tx_hash, type, attr_idx)
);
CREATE INDEX idx_events_type_value ON events (type, attr_value);
CREATE INDEX idx_events_height     ON events (height);

CREATE TABLE cursor (
    name  TEXT PRIMARY KEY,                 -- 'main'
    height INTEGER NOT NULL
);
```

## 4. 高水位与重连

- 每块 commit 后，原子 `UPDATE cursor SET height = ?`。
- 重启时读 cursor，从 `height + 1` 开始。
- 节点短时不可达时：每 5s 重试，**不退水位**，等重连后补。
- 节点永远追不上时（高度差 > 100）：告警，但**不丢数据**（保留已索引的高度）。

## 5. 性能参考

| 形态 | 块/s 持续吞吐 | 备注 |
|---|---|---|
| SQLite + 普速 SSD | 10–30 | 单机足够 devnet |
| Postgres + 16 GB | 100+ | 适合 mainnet 索引 |
| SQLite + 归档节点 | 30–50 | 追高度差 = 落后天数 × 21600 |

## 6. 安全

- **只读节点 RPC**：建议给 indexer 一个 `indexer-readonly` 用户、只允许
  `/subscribe`、`/block`、`/tx` 等查询类接口（nginx 限流 + ACL）。
- **不要复用节点签名 key**：indexer 进程即使被攻陷，也无任何提权通道。
- **DB 文件权限**：`chmod 600 /var/lib/mcchain/indexer.db`。

## 7. 扩展点

- **EventSink 接口**：`Sink.Write(ctx, block *Block, events []Event) error`。
  默认实现是 SQLite；可替换为 Kafka / S3 / ClickHouse。
- **额外过滤**：`--filter "transfer.amount>10000000umc"` 只写大额转账。
- **多链聚合**：起多个 indexer 实例，每个 `--chain-id` 写不同 DB 表，
  上层做 cross-chain join。

## 8. 已知限制（v1）

- 不解析 wasm 事件（CosmWasm 自定义 event 仍可读，但字段是 raw）。
- 不支持 pruning 节点（节点必须有完整 history）。
- 单进程 = 单 DB writer；要横向扩展请用 Postgres。

## 9. 故障处置

| 症状 | 处置 |
|---|---|
| 高度差 > 1000 | 检查节点是否还活着（`/status`）；不要清 DB |
| DB 损坏 | 备份后用 `sqlite3 .recover`；不要在损坏状态下继续写 |
| 重复写入 | UNIQUE 约束会兜底，不会污染数据；日志里搜 "duplicate" 看频率 |