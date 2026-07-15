# MobileChain B5 批次 · 生态工具（统一查询/事件接口；SDK/浏览器/钱包/DePIN 链下契约）· 增量架构设计 + 任务分解

**文档类型**：增量架构设计（基于 B1–B4 已落地模块，仅描述变更）
**批次**：B5（生态工具）— 路线图生态扩展部分
**作者**：高见远（Gao），Architect
**语言**：简体中文
**技术栈**：Cosmos SDK v0.47.3 + cometbft v0.37.1 + Ignite。**不写实现代码**；验收以本机 `ignite chain build` + `go test` + 事件订阅 / 查询覆盖 + 链下契约评审为准。
**配套图**：`docs/b5_ecosystem_class-diagram.mermaid`、`docs/b5_ecosystem_sequence-diagram.mermaid`。

---

# Part A · 系统设计

## 1. 实现方案与框架选型

| 难点 | 方案 |
|------|------|
| 统一查询接口 | 新增**只读** `x/ecosystem` 聚合模块：注入 `tokenomics`/`phonenode`/`edgeai`/`depin` 的只读 keeper 接口，暴露 `Query{Overview,NodeDetail,TaskDetail}`；无状态、无 Msg、无 BeginBlock、无模块账户。autocli/gRPC 自动暴露各模块既有 query。 |
| 标准化事件 | 定义统一 topic 约定（`mcchain.<module>.<Event>`），在状态变更点 EmitEvent：tokenomics(Mint/Allocation)、depin(Contribution/Payout)、phonenode(Attestation/Slash/StateProof)、edgeai(TaskCreated/ResultAccepted/Dispute/Resolved)；治理沿用 cosmos gov 事件。 |
| SDK/浏览器/钱包/DePIN | **仅产出接口契约文档**（OpenAPI + proto + 验收清单），链下工程后续实现；本批次不写链下代码。 |

- **框架**：沿用 Cosmos SDK 标准模块模式；只读聚合模块零状态。**无新第三方依赖**。
- **关键决策（推荐默认，不抛用户）**：(1) `x/ecosystem` 为纯只读聚合，不改任何既有状态/mint/slash 语义；(2) 事件仅在既有状态变更点补 EmitEvent，不新增链上逻辑分支；(3) 链下契约覆盖 R1–R4 的链上能力，明确「链下不在本批次」；(4) 移动钱包契约复用 B4 轻客户端 proof，DePIN 需求契约复用 B3 任务类型。

## 2. 文件列表及相对路径

### 2.1 新增 `x/ecosystem`（只读聚合查询模块）
| 文件 | 关键变更 |
|------|----------|
| `x/ecosystem/types/keys.go` | `ModuleName="ecosystem-query"`（避免与 B1 生态池账户名冲突）、`StoreKey`、`MemStoreKey="mem_ecosystem_query"` |
| `x/ecosystem/types/expected_keepers.go` | 只读接口：`TokenomicsKeeper{GetMintedSupply, GetAllocations}`、`PhonenodeKeeper{GetNode,HasNode,IsAttested,GetAttestation,CountNodes}`、`EdgeaiKeeper{GetTask,GetResult,GetDispute,CountTasks}`、`DepinKeeper{GetDevice,CountDevices}`（均用 sdk/各模块已导出类型，单向 import，无 cycle） |
| `x/ecosystem/types/query.proto` / `query.pb.go`(生成) | `QueryOverviewRequest/Response`、`QueryNodeDetailRequest/Response`、`QueryTaskDetailRequest/Response` |
| `x/ecosystem/genesis.go` | `InitGenesis`/`ExportGenesis` 空实现（无状态） |
| `x/ecosystem/keeper/keeper.go` | 注入上述只读 keeper；`NewKeeper(appCodec, storeKey, memKey, tk, pn, ea, dp)` |
| `x/ecosystem/keeper/query.go` | `QueryOverview`（聚合 tokenomics 总量/分配 + phonenode 节点/attest 计数 + edgeai 任务计数 + depin 设备计数）、`QueryNodeDetail`、`QueryTaskDetail` |
| `x/ecosystem/keeper/msg_server.go` | 无 Msg（空实现占位以满足 module 接口） |
| `x/ecosystem/module.go` | `AppModule`/`AppModuleBasic`；`RegisterServices` 仅注册 Query；`BeginBlock`/`EndBlock` 空；`ConsensusVersion=1` |
| `x/ecosystem/client/cli/query.go` | `q ecosystem overview` / `node-detail <addr>` / `task-detail <id>` |

### 2.2 修改既有模块（标准化 Event 补点）
| 文件 | 关键变更 |
|------|----------|
| `x/tokenomics/genesis.go` + `keeper/vesting.go` | `InitGenesis` 铸池后 `EmitEvent("mcchain.tokenomics.Mint", cap, minted)`；分配后 `EmitEvent("mcchain.tokenomics.Allocation", pool, amount)` |
| `x/depin/keeper/msg_server_submit_contribution.go` | 拨付成功 `EmitEvent("mcchain.depin.Payout", toAddr, amount)`；贡献落盘 `EmitEvent("mcchain.depin.Contribution", device, taskType, score)` |
| `x/phonenode/keeper/attestation.go` + `slash.go` + `state.go` | 分别 `EmitEvent("mcchain.phonenode.Attestation" / "Slash" / "StateProof")` |
| `x/edgeai/keeper/task.go` + `result.go` + `dispute.go` | 分别 `EmitEvent("mcchain.edgeai.TaskCreated" / "ResultAccepted" / "Dispute" / "Resolved")` |

### 2.3 修改 `app/app.go`（**串行共享，务必精确**）
- **imports**：新增 `ecosystemmodule "mcchain/x/ecosystem"`、`ecosystemmodulekeeper`、`ecosystemmoduletypes`。
- **ModuleBasics**：追加 `ecosystemmodule.AppModuleBasic{}`。
- **NewKVStoreKeys / memKeys**：追加 `ecosystemmoduletypes.StoreKey` / `MemStoreKey`。
- **App struct**：新增 `EcosystemQueryKeeper ecosystemmodulekeeper.Keeper`。
- **New() 装配**：在 `app.EdgeaiKeeper` 之后创建
  `app.EcosystemQueryKeeper = *ecosystemmodulekeeper.NewKeeper(appCodec, keys[ecosystemmoduletypes.StoreKey], keys[ecosystemmoduletypes.MemStoreKey], app.TokenomicsKeeper, app.PhonenodeKeeper, app.EdgeaiKeeper, app.DepinKeeper)`；
  注意 `PhonenodeKeeper` 须已是 B2 版（含 `IsAttested`/`GetAttestation`），`EdgeaiKeeper` 须已落地（B3）。
- **mm 模块列表**：追加 `ecosystemModule`（置于末尾）。
- **SetOrderBeginBlockers/EndBlockers/InitGenesis**：追加 `ecosystemmoduletypes.ModuleName`；InitGenesis 空实现，置于最末。
- **initParamsKeeper**：ecosystem 无 params（纯只读），**不新增 subspace**。
- **maccPerms**：**不变**（ecosystem 查询模块无模块账户）。
- **config.yml**：**不变**（不新增币）。

### 2.4 链下契约（文档，不实现）
| 文件 | 关键变更 |
|------|----------|
| `docs/events.md`（新增） | 标准化事件 topic + 字段 + 订阅示例 |
| `docs/sdk_contract.md`（新增） | JS/Python SDK 端点/消息契约 + 验收（查询余额、提交任务、监听事件） |
| `docs/explorer_contract.md`（新增） | 区块浏览器消费事件/状态契约 + 验收 |
| `docs/wallet_contract.md`（新增） | 移动钱包轻客户端集成契约（复用 B4 proof）+ 验收 |
| `docs/depin_demand_contract.md`（新增） | 真实 DePIN 需求样例契约（复用 B3 任务类型 inference/data_label/bandwidth）+ 验收 |

## 3. 数据结构与接口（类图，见 `docs/b5_ecosystem_class-diagram.mermaid`）

**ecosystem 聚合查询结构（proto）**
```proto
message QueryOverviewResponse {
  uint64 total_supply_cap = 1;     // tokenomics.TotalSupplyCap
  uint64 minted_supply   = 2;      // tokenomics.GetMintedSupply
  repeated PoolView allocations = 3;// tokenomics.GetAllocations
  int64  node_count      = 4;       // phonenode.CountNodes
  int64  attested_count  = 5;       // phonenode（IsAttested 过滤）
  int64  task_total      = 6;       // edgeai.CountTasks
  int64  task_open       = 7;       // edgeai
  int64  task_disputed   = 8;       // edgeai
  int64  depin_devices   = 9;       // depin.CountDevices
}
message QueryNodeDetailResponse { NodeState node=1; Attestation attestation=2; int64 proof_count=3; }
message QueryTaskDetailResponse { Task task=1; Result result=2; Dispute dispute=3; }
```
**标准事件类型**（约定，非独立 struct）：`mcchain.tokenomics.{Mint,Allocation}`、`mcchain.depin.{Contribution,Payout}`、`mcchain.phonenode.{Attestation,Slash,StateProof}`、`mcchain.edgeai.{TaskCreated,ResultAccepted,Dispute,Resolved}`。

## 4. 程序调用流程（时序图，见 `docs/b5_ecosystem_sequence-diagram.mermaid`）

- **① 生态总览聚合查询**：`q ecosystem overview` → `EcosystemQueryKeeper.QueryOverview` → 并发读取 `tokenomics.GetMintedSupply/GetAllocations` + `phonenode.CountNodes` + `edgeai.CountTasks` + `depin.CountDevices` → 聚合返回（只读，零状态变更）。
- **② 标准化事件流**：任意状态变更（mint/分配/贡献/拨付/attestation/slash/StateProof/任务/结果/争议/裁定）触发对应 `EmitEvent` → Tendermint 索引 → 区块浏览器/SDK 订阅 `mcchain.<module>.<Event>`。
- **③ 链下契约消费**：SDK/浏览器/钱包/DePIN 约定调用既有 gRPC/REST + `q ecosystem *` + 订阅事件（钱包复用 B4 proof，DePIN 复用 B3 任务类型）。

## 5. 任务列表（有序、含依赖、优先级）
| Task | 名称 | 依赖 | 优先级 | 涉及文件 |
|------|------|------|--------|----------|
| **T1** | 标准化事件约定 + 各模块 EmitEvent 补点 | 无（依赖 B2/B3 已落地状态变更点） | P0 | `docs/events.md`、`x/tokenomics/{genesis,keeper/vesting}.go`、`x/depin/keeper/msg_server_submit_contribution.go`、`x/phonenode/keeper/{attestation,slash,state}.go`、`x/edgeai/keeper/{task,result,dispute}.go` |
| **T2** | 只读 `x/ecosystem` 聚合模块 + app.go 装配 | T1 | P0 | `x/ecosystem/**`、`app/app.go` |
| **T3** | 各模块 query 字段补全（autocli + 必要明细） | T2 | P1 | `x/tokenomics/keeper/grpc_query.go`、`x/phonenode/keeper/query_*.go`、`x/edgeai/keeper/query_*.go` |
| **T4** | 链下契约文档（sdk/explorer/wallet/depin_demand） | T2,T3 | P1 | `docs/{sdk_contract,explorer_contract,wallet_contract,depin_demand_contract}.md` |
| **T5** | 集成验证（build + 事件订阅 + 查询覆盖冒烟） | T1-T4 | P0 | `x/ecosystem/keeper/*_test.go`、`x/*/..._test.go`（事件/查询）、本地冒烟脚本/文档 |
**P0 验收锚点**：`q ecosystem overview` 返回聚合视图且与各模块 `q` 一致；关键状态变更均产生可订阅标准事件；autocli 暴露余额/分配/任务/节点/证明字段；链下契约文档完整且链上端点满足。

## 6. 依赖包列表
- `github.com/cosmos/cosmos-sdk` v0.47.3（autocli/grpc/事件均内置）；`x/tokenomics`、`x/phonenode`、`x/edgeai`、`x/depin`（已落地）。**无需新增任何第三方依赖**。

## 7. 共享知识（跨文件约定，含与 B1–B4 交互边界）
- **denom/单位**：同 B1（`umc`）。
- **事件命名铁律**：统一前缀 `mcchain.<module>.<Event>`（小写 module/event），属性用 `sdk.EventAttribute`；便于 B5 浏览器/SDK 单一订阅规则。
- **ecosystem 模块为纯只读**：绝不持有资金、绝不 MintCoins、绝不改状态；仅聚合读取他模块 keeper。**minted_supply / cap 不变**（延续 B1 铁律）。
- **模块名避让**：聚合模块名 `ecosystem-query`，与 B1 生态池账户名 `ecosystem`（模块账户）区分，避免 maccPerms/app 冲突。
- **与 B4 衔接**：钱包契约复用 B4 `q phonenode proof-state` IAVL proof；与 B3 衔接：DePIN 需求契约复用 B3 任务类型（inference/data_label/bandwidth）。
- **治理事件**：沿用 cosmos gov 原生事件，不自定义；B2 治理参数变更由 gov 事件覆盖。

## 8. 待明确事项（仅真正需后续定稿，占位并注明）
1. **聚合查询性能/分页**：Overview 全量计数在大规模下可能变重；v1 直接计数可接受，B6 主网前评估是否改增量计数器或链下索引（本批实现为直接读）。
2. **事件字段 Schema 版本化**：事件属性命名/类型若后续变更需向后兼容策略，列为 B6 文档化事项。
3. **链下工程归属**：SDK/浏览器/钱包/DePIN 实现留待链下团队，本批仅契约。
4. **团队多签/治理**：事件与查询不涉及新权限，沿用既有。

> 其余均按推荐默认定稿，未向用户抛待确认。
