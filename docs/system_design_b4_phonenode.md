# MobileChain B4 批次 · PhoneNode（链上轻量同步 + 弱网参数；客户端 App 链下快速模式）· 增量架构设计 + 任务分解

**文档类型**：增量架构设计（基于 B1–B3 已落地模块，仅描述变更）
**批次**：B4（PhoneNode 轻量接入）— 路线图移动端接入部分
**作者**：高见远（Gao），Architect
**语言**：简体中文
**技术栈**：Cosmos SDK v0.47.3 + cometbft v0.37.1 + Ignite。**不写实现代码**；验收以本机 `ignite chain build` + `go test` + `mcchaind q ...` / proof 查询 + 链下契约评审为准。
**配套图**：`docs/b4_phonenode_class-diagram.mermaid`、`docs/b4_phonenode_sequence-diagram.mermaid`。

---

# Part A · 系统设计

## 1. 实现方案与框架选型

| 难点 | 方案 |
|------|------|
| 状态裁剪 / snapshot | 复用 cosmos SDK 既有 pruning + snapshot（节点 `app.toml` 配置驱动）；B4 负责**固化移动端友好默认值模板** + 启动/运维说明，不新增链上逻辑。 |
| 弱网/带宽/电量参数 | 扩展 `x/phonenode` params（`BandwidthCapBytesPerBlock`/`HeartbeatIntervalBlocks`/`SyncThrottleWindow` 等）；与 B2 `OfflineGraceBlocks` 共用；`ParamSetPairs`/`Validate` 补全，且断言 `OfflineGraceBlocks >= HeartbeatIntervalBlocks`（防误 slash）。 |
| 轻客户端状态证明 | 复用 cosmos `abci.Query` proof + IAVL：baseapp 已对 `/store/<storekey>/key?prove=true` 返回承诺证明；B4 新增 `q phonenode proof-state <addr>` CLI 便捷封装（用 `clientCtx.QueryStoreWithProof`），无需改 keeper。 |
| 客户端 App 链下契约 | 仅产出 `docs/client_app_contract.md`：gRPC/REST 端点清单、消息契约、同步/重连/节流协议、弱网低电量验收标准。**本批次不实现 App**。 |

- **框架**：沿用 Cosmos SDK 标准模块模式；pruning/snapshot 经节点配置（符合 cosmos 惯例）。**无新第三方依赖**。
- **关键决策（推荐默认，不抛用户）**：(1) pruning/snapshot 不写链上代码，只固化 `app.toml` 默认值模板；(2) 弱网参数并入 `x/phonenode` params（沿用 B1 subspace，可由 B2 治理调）；(3) proof 复用 baseapp 原生 IAVL proof，仅加 CLI 便捷封装；(4) 离线宽限 `OfflineGraceBlocks`（B2）须 ≥ `HeartbeatIntervalBlocks`（B4），由 param `Validate` 强约束；(5) 客户端 App 仅交付契约文档，链下工程后续批次/链下团队实现。

## 2. 文件列表及相对路径

### 2.1 修改 `x/phonenode`
| 文件 | 关键变更 |
|------|----------|
| `x/phonenode/types/params.go` | 扩展 params：`BandwidthCapBytesPerBlock int64`、`HeartbeatIntervalBlocks int64`、`SyncThrottleWindow int64`、`MaxMobilePeers int32`；`ParamSetPairs`/`Validate` 补全，校验 `OfflineGraceBlocks >= HeartbeatIntervalBlocks` 与弱网参数非负 |
| `x/phonenode/types/genesis.go` | `DefaultGenesis` 纳入新 params 默认值 |
| `x/phonenode/client/cli/query_proof.go` | 新增 `proof-state <addr>` 命令：调 `clientCtx.QueryStoreWithProof(nodeKey, storeName)` 返回值 + IAVL 证明并打印 |
| `x/phonenode/client/cli/query_params.go` | 既有 `params` 命令可显示新增弱网参数 |

### 2.2 节点配置模板（pruning / snapshot 默认值）
| 文件 | 关键变更 |
|------|----------|
| `config/config.go`（若 Ignite 生成）或 `config.yml` 注释模板 + `docs/node_config_pruning_snapshot.md` | 固化移动端友好 `app.toml` 片段：`pruning="custom"`、`pruning-keep-recent=100`、`pruning-keep-every=0`、`pruning-interval=10`；`state-sync`/`snapshot`：`snapshot-interval=1000`、`snapshot-keep-recent=2`；附「弱网节点启动建议」 |

### 2.3 链下契约（文档，不实现）
| 文件 | 关键变更 |
|------|----------|
| `docs/client_app_contract.md`（新增） | gRPC/REST 端点清单（auth/tokenomics/phonenode/depin/edgeai 关键 query + tx）、消息契约、同步/重连/节流协议、弱网低电量验收标准（覆盖 R1–R3 链上能力） |

### 2.4 修改 `app/app.go`
- **无代码变更**：pruning/snapshot 由节点 `app.toml` 配置驱动（cosmos 惯例）；弱网参数已并入 `x/phonenode` params，沿用既有 subspace，B2 治理可改。`config.yml`：不变（不新增币）。

## 3. 数据结构与接口（类图，见 `docs/b4_phonenode_class-diagram.mermaid`）

**phonenode params 扩展要点（proto/Go 结构）**
```go
// x/phonenode/types/params.go 扩展字段
type Params struct {
    // —— B2 既有（安全/离线）——
    AttestationRequired  bool
    AttestationValidity  int64
    SybilDeviceBinding   bool
    OfflineGraceBlocks   int64
    OfflineSlashBps      uint32
    ContribSlashBps      uint32
    AttestSlashBps       uint32
    // —— B4 新增（弱网/带宽/电量）——
    BandwidthCapBytesPerBlock int64  // 每块带宽上限（节流）
    HeartbeatIntervalBlocks    int64  // 期望 state proof 提交节奏
    SyncThrottleWindow        int64  // 同步批处理窗口
    MaxMobilePeers            int32  // 移动端最大对等连接
}
```
**核心方法（复用既有）**：`phonenode.Keeper.{GetNode,HasNode,SubmitStateProof}`（B2 已用 `LastRoot` 心跳）；`q phonenode proof-state`（B4 新增 CLI，基于 `QueryStoreWithProof`）。

## 4. 程序调用流程（时序图，见 `docs/b4_phonenode_sequence-diagram.mermaid`）

- **① 轻客户端 proof 验证**：移动端 `q phonenode proof-state <addr>` → `clientCtx.QueryStoreWithProof(nodeKey, "phonenode")` → baseapp 返回 IAVL 承诺证明 → 客户端本地验证（无需全节点），验证余额/节点 attestation/任务状态同理（tokenomics/edgeai 同机制）。
- **② 弱网参数生效 + 离线协调**：节点按 `HeartbeatIntervalBlocks` 节奏提交 `SubmitStateProof`（更新 `LastRoot`）；B2 `BeginBlock` 用 `OfflineGraceBlocks` 判定离线（因 `OfflineGraceBlocks >= HeartbeatIntervalBlocks`，弱网不误 slash）；`BandwidthCapBytesPerBlock`/`SyncThrottleWindow` 指导客户端节流（链下）。
- **③ snapshot/pruning 快速同步**：节点按 `app.toml`（`pruning="custom"` + `snapshot-interval`）运行；新移动端从最近 snapshot 初始化并追赶，不全量重放；`q` 不受影响。

## 5. 任务列表（有序、含依赖、优先级）
| Task | 名称 | 依赖 | 优先级 | 涉及文件 |
|------|------|------|--------|----------|
| **T1** | phonenode 弱网参数扩展 + 校验（与 B2 离线宽限协调） | 无（设计上依赖 B2 已落地 params 机制） | P0 | `x/phonenode/types/params.go`、`types/genesis.go` |
| **T2** | 轻客户端 proof 查询便捷封装（CLI） | T1 | P1 | `x/phonenode/client/cli/query_proof.go`、`query_params.go` |
| **T3** | 节点配置模板（pruning/snapshot 默认值 + 启动说明） | 无 | P0 | `config/config.go`(或 `config.yml` 模板) + `docs/node_config_pruning_snapshot.md` |
| **T4** | 链下客户端 App 接口契约文档 | T2,T3 | P2 | `docs/client_app_contract.md` |
| **T5** | 集成验证（build + pruning/snapshot 冒烟 + proof 查询冒烟） | T1-T4 | P0 | `x/phonenode/..._test.go`（参数校验测试）、本地冒烟脚本/文档 |
**P0 验收锚点**：`q phonenode params` 可见弱网参数且 `OfflineGraceBlocks>=HeartbeatIntervalBlocks` 校验生效；`q phonenode proof-state` 返回可验证 IAVL 证明；节点以 mobile pruning/snapshot 配置启动正常、状态一致、`q` 不受影响；链下契约文档完整覆盖 R1–R3。

## 6. 依赖包列表
- `github.com/cosmos/cosmos-sdk` v0.47.3（pruning/snapshot/IAVL proof 均内置）；`x/phonenode`（已落地）。**无需新增任何第三方依赖**。

## 7. 共享知识（跨文件约定，含与 B1–B3 交互边界）
- **denom/单位**：同 B1（`umc`）。
- **离线宽限协调（铁律约束）**：`OfflineGraceBlocks`（B2，slash 用）必须 ≥ `HeartbeatIntervalBlocks`（B4，心跳节奏），由 `phonenode` params `Validate` 强校验；改任一参数经 B2 治理时须保持该不等式，避免弱网误 slash。
- **pruning/snapshot 配置驱动**：pruning 策略与 snapshot 间隔经节点 `app.toml` 设置，链上代码零侵入（cosmos 惯例）；B4 只固化默认值模板，不写链上逻辑。
- **proof 复用 baseapp**：所有模块 KVStore 的 key 均可通过 `/store/<module>/key?prove=true` 获取 IAVL 承诺证明；B4 `proof-state` 仅作 CLI 便捷封装，验证逻辑在客户端。
- **不破 B1–B3 语义**：B4 不新增 mint/分配/slash 逻辑；弱网参数仅作节流与节奏提示，不改变 B2 slash 行为本身。
- **事件约定**：B4 不新增链上事件（pruning/snapshot 为节点运维侧）。

## 8. 待明确事项（仅真正需后续定稿，占位并注明）
1. **snapshot 对外分发协议**：轻客户端从 snapshot 拉取的 P2P/HTTP 端点与校验（hash 清单）由 B5 生态工具/链下工程细化，本批只固化生成配置。
2. **state-sync 启用阈值**：是否默认开启 state-sync、trusting period 取值，列为 B6 主网配置项（本批给默认值，主网前复核）。
3. **客户端 App 实现归属**：App 属链下工程，不在 B4 实现；其 gRPC/REST 具体 SDK 封装留待 B5 SDK + 链下团队，本批仅契约。
4. **团队多签/治理**：弱网参数治理沿用 B2 白名单（不含 cap），无新增。

> 其余均按推荐默认定稿，未向用户抛待确认。
