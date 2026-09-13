# 性能基线（PERF_BASELINES）

MC 公链节点在不同形态下的**预期性能数字** + **如何在自己的机器上复测**。
这些不是「宣传话术」，而是后续容量规划 / 异常告警 / 升级回滚的统一参照系。

## 1. 测量方法（所有人都按这套来）

```bash
# 1) 准备一条单节点 devnet
./scripts/devnet.sh clean
./scripts/devnet.sh init 1
./scripts/devnet.sh start

# 2) 待高度 > 5 之后，跑基准
./scripts/bench.sh all 2>&1 | tee bench-$(date +%Y%m%d-%H%M%S).log
```

`scripts/bench.sh` 提供下列基准用例：
| 编号 | 命令 | 关注指标 |
|---|---|---|
| B1 | `curl -s localhost:1317/cosmos/base/tendermint/v1beta1/blocks/latest` × 100 | REST 端点 p99 延迟 |
| B2 | `mcchaind query bank total --denom umc` × 1000 | LCD 序列化吞吐 |
| B3 | `mcchaind tx bank send ...` × 100（不同账户间转账） | tx 端到端吞吐、内存增长 |
| B4 | `mcchaind q block --type=height <H>` × 1000 | 历史回放延迟 |
| B5 | 并发 200 线程重 B3 | 在并发下不退化（关键 = 链的并发处理上限） |

## 2. 参考基线（4C/8GB/普通 NVMe，单节点 devnet）

> ⚠️ **这些数字是开发网形态的初步参考**，不是 mainnet SLA。
> 升级 PR 必须有同 PR 下的对比数字。

| 指标 | 数值 | 备注 |
|---|---|---|
| 区块出块间隔 | 4s ± 0.5s | 与 `timeout_commit` 一致；含空块 |
| 同步首块（state-sync） | ≤ 10 min | 4C/8GB 公网节点参考值 |
| 同步首块（区块回放） | ≥ 4 hour | 从创世重放；不推荐生产用 |
| B1 p99 REST 延迟 | < 200 ms | 取决于机器；gRPC 一般再低 30% |
| B3 端到端（单笔转账） | ~4.5 s | 一笔 tx 上链 + confirm |
| B3 并发 200 tx / 块 | 不应明显变慢 | 关键 = 验证 mempool 并发处理能力 |
| 内存常驻 | < 2 GB | devnet；mainnet + state-sync 短时会高 |
| 磁盘（archive）增长 | ~50 GB / 月 | 与 tx 频率线性相关，参考低频实测 |

## 3. 必须盯紧的「不正常」信号

- **出块间隔 > 8s**：某验证人可能没签，检查 `/consensus_state`。
- **REST 1317 p99 > 500 ms**：node 资源饱和或者 mnesia-like 序列化阻塞，先抓 pprof。
- **内存常驻 > 4 GB**（devnet）：IAVL 配置不对（`pruning-keep-recent` 太小或 `nothing`）。
- **磁盘日增长 > 200 MB**（archive）：先看是否 `tx_index.index_all_keys=true`。

## 4. 如何扩展到主网水平（性能层级）

1. **节点分形**：1 sentry + 1 signing validator + 2 个对外 RPC + 1 个 archive，互相隔离。
2. **IAVL 配置**：`pruning = "custom"` + `pruning-keep-recent = 362880` 是产线平衡点。
   archive 节点 `nothing`。
3. **gRPC 比 REST 优先**：大对象查询（list validator、history）gRPC 节省 ~30%。
4. **不要在验证人机器上跑 RPC 服务**：把 RPC 拆出来到独立节点，
   验证人只接 sentry 的 P2P。这是 DEPLOY 的标准实践。
5. **大对象查询分页**：`page.limit=100`（默认），历史超过 1000 条请分批。

## 5. 与告警的对应

`docs/ALERTS.md` 里的告警阈值都基于本基线；
一旦在某个 PR 里发现基线变了，请同步更新两边以保持一致。