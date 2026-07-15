# MobileChain B3 批次 · EdgeAI（attested execution + 任务/结果/anti-cheat，关联 depin）· 增量架构设计 + 任务分解

**文档类型**：增量架构设计（基于 B1 `x/tokenomics` + B2 `x/phonenode`(attestation/slashing) + 现有 `x/depin`，仅描述变更）
**批次**：B3（EdgeAI）— 路线图「阶段四 经济模型与安全」延伸的移动端贡献即挖矿
**作者**：高见远（Gao），Architect
**语言**：简体中文
**技术栈**：Cosmos SDK v0.47.3 + cometbft v0.37.1 + Ignite。**不写实现代码**；验收以本机 `ignite chain build` + `go test` + `mcchaind q ...` / 事件监听为准。
**配套图**：`docs/b3_edgeai_class-diagram.mermaid`、`docs/b3_edgeai_sequence-diagram.mermaid`。

---

# Part A · 系统设计

## 1. 实现方案与框架选型

| 难点 | 方案 |
|------|------|
| 新模块 `x/edgeai` | 标准 Cosmos 模块模式（与 depin/phonenode/tokenomics 一致）：types / keeper / genesis / msg_server / module / client/cli + query / proto。状态机：Task(open→assigned→done/disputed)、Result(pending→accepted/cheated)、Dispute(open→resolved)。 |
| attested execution 验证 | 链上仅存 `proof_root` + `result_hash` + `attestation_status` 引用；提交时强制 `phonenodeKeeper.IsAttested(addr)`（B2 已加）；不重复实现硬件验证。 |
| anti-cheat / 争议 | optimistic：结果先 `pending` 接受，进入 `DisputePeriodBlocks` 挑战窗口；窗口内可 `MsgSubmitDispute`（提交者须 attest 节点）；窗口到期无未决争议 → `BeginBlock` 自动 `accepted` 并触发拨付；`MsgResolveDispute`（治理/仲裁账户）裁定作弊 → 调用 B2 `phonenodeKeeper.SlashIfBad` + 诚实挑战者经 depin 获奖励。 |
| 奖励联动（铁律：不破 B1 cap） | edgeai **绝不 MintCoins**；经 depin 新增的 `PayOutFromPool(ctx, toAddr, amt)` 从 depin 模块账户余额（= B1 生态池 1e14 umc InitialPool 切片，已计入 cap）拨付。minted_supply 不变。 |
| 与 B2 衔接 | 提交路径 `phonenodeKeeper.IsAttested` 前置校验；作弊裁定 `phonenodeKeeper.SlashIfBad(reason="edgeai_cheat"/"edgeai_false_dispute")`。 |

- **框架**：沿用 Cosmos SDK 标准模块模式。**无新第三方依赖**。
- **关键决策（推荐默认，不抛用户）**：(1) edgeai 不直接 mint，奖励 100% 走 depin 拨付函数 → 单一入口、cap 不被突破；(2) 争议仲裁 `MsgResolveDispute` 在 v1 限定由治理/仲裁账户（默认复用 B1 团队多签占位）发起，主网前经 B6 迁 DAO；(3) anti-cheat 阈值放入 edgeai params，可由 B2 最小治理调整；(4) edgeai 不持有资金、无新模块账户 → `config.yml` 与 `maccPerms` 均不变。

## 2. 文件列表及相对路径

### 2.1 新增 `x/edgeai`
| 文件 | 关键变更 |
|------|----------|
| `x/edgeai/types/params.go` | `Params{DisputePeriodBlocks int64, AntiCheatDupWindow int64, SlashBpsOnCheat uint32, ChallengerRewardBps uint32, MaxTasksPerNode int32, ResolverAddress string}`；`ParamKeyTable`/`ParamSetPairs`/`Validate`/`DefaultParams` 补全 |
| `x/edgeai/types/keys.go` | `ModuleName="edgeai"`、`StoreKey`、`MemStoreKey="mem_edgeai"`；KV key：`TaskKey`、`ResultKey`、`DisputeKey`(均 prefix+id) |
| `x/edgeai/types/task.go` | `Task{Id, Creator, Spec, RewardAmount sdk.Int, Status, AssignedTo, CreatedHeight, DisputeUntil}` |
| `x/edgeai/types/result.go` | `Result{TaskId, Executor, ProofRoot, ResultHash, Attested bool, Status(pending/accepted/cheated), SubmittedHeight}` |
| `x/edgeai/types/dispute.go` | `Dispute{Id, TaskId, Challenger, Reason, Status(open/resolved), Cheated bool, ResolveHeight}` |
| `x/edgeai/types/genesis.go` | `DefaultGenesis`(空任务集+默认 params+空争议)/`Validate` |
| `x/edgeai/types/expected_keepers.go` | `BankKeeper`(SendCoinsFromModuleToAccount)、`PhonenodeKeeper`(HasNode/IsAttested/SlashIfBad)、`DepinKeeper`(PayOutFromPool) 接口（仅用 sdk 类型，避免 import cycle） |
| `x/edgeai/types/tx.proto` / `tx.pb.go`(生成) | `MsgCreateTask`、`MsgAssignTask`、`MsgSubmitResult`、`MsgSubmitDispute`、`MsgResolveDispute` |
| `x/edgeai/types/query.proto` / `query.pb.go`(生成) | `QueryTask`/`QueryResult`/`QueryDispute`/`QueryParams` |
| `x/edgeai/genesis.go` | `InitGenesis`/`ExportGenesis` |
| `x/edgeai/keeper/keeper.go` | 注入 `BankKeeper`/`PhonenodeKeeper`/`DepinKeeper`；核心方法：`CreateTask`/`AssignTask`/`SubmitResult`/`SubmitDispute`/`ResolveDispute`/`SetTask`/`GetTask`/`SetResult`/`GetResult`/`SetDispute`/`GetDispute` |
| `x/edgeai/keeper/task.go` | 任务状态机：Create(→open)、Assign(→assigned，校验 IsAttested + 未重复领取) |
| `x/edgeai/keeper/result.go` | `SubmitResult`：校验 IsAttested(executor) + HasNode；写 Result(pending) + 设 `DisputeUntil=cur+DisputePeriodBlocks`；`BeginBlock` 到期无未决争议 → `acceptResult`(→accepted) |
| `x/edgeai/keeper/dispute.go` | `SubmitDispute`(挑战者须 attest 节点)、`ResolveDispute`(仲裁账户)：cheated→`phonenodeKeeper.SlashIfBad(executor)` + `depinKeeper.PayOutFromPool(challenger, reward)`；false→`SlashIfBad(challenger)` |
| `x/edgeai/keeper/payout_bridge.go` | 封装 `payout(ctx, toAddr, amt)` → `depinKeeper.PayOutFromPool`；`acceptResult` 内部调用 |
| `x/edgeai/keeper/msg_server.go` + `msg_server_*.go` | 各 Msg 处理（创建/领取/提交结果/争议/裁定） |
| `x/edgeai/keeper/query.go` + `query_*.go` | `q edgeai task/result/dispute/params` |
| `x/edgeai/module.go` | `BeginBlock`(扫描到期 pending→accepted+payout)、`RegisterInvariants`(可选：sum paid ≤ depin pool 余额) |
| `x/edgeai/client/cli/*.go` | 各 Msg/Query 的 CLI 子命令 |
| `x/edgeai/genesis_test.go` / `keeper/*_test.go` | 任务机/anti-cheat 模拟测试 |

### 2.2 修改 `x/depin`（新增可被 edgeai 调用的拨付函数）
| 文件 | 关键变更 |
|------|----------|
| `x/depin/keeper/payout.go` | 新增 `PayOutFromPool(ctx sdk.Context, toAddr sdk.AccAddress, amt sdk.Coins) error`：内部 `bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, toAddr, amt)`（从 depin 模块账户余额 = B1 生态池 InitialPool 切片拨付）。**不 MintCoins**。 |
| `x/depin/types/expected_keepers.go` | 不变（depin 自身已持 bankKeeper）。edgeai 侧通过 `DepinKeeper` 接口引用 `PayOutFromPool`。 |

### 2.3 修改 `app/app.go`（**串行共享，务必精确**）
- **imports**：新增 `edgeaimodule "mcchain/x/edgeai"`、`edgeaimodulekeeper`、`edgeaimoduletypes`。
- **ModuleBasics**：追加 `edgeaimodule.AppModuleBasic{}`。
- **NewKVStoreKeys**：追加 `edgeaimoduletypes.StoreKey`；并追加 `edgeaimoduletypes.MemStoreKey`（memKeys）。
- **App struct**：新增 `EdgeaiKeeper edgeaimodulekeeper.Keeper`。
- **New() 装配**：在 `app.DepinKeeper = *depinmodulekeeper.NewKeeper(...)` 之后创建
  `app.EdgeaiKeeper = *edgeaimodulekeeper.NewKeeper(appCodec, keys[edgeaimoduletypes.StoreKey], keys[edgeaimoduletypes.MemStoreKey], app.GetSubspace(edgeaimoduletypes.ModuleName), app.BankKeeper, app.PhonenodeKeeper, app.DepinKeeper)`；
  注意 `PhonenodeKeeper` 须已是 B2 实现后带 `IsAttested`/`SlashIfBad` 的版本；`DepinKeeper` 须已含 `PayOutFromPool`（本批新增）。
- **mm 模块列表**：追加 `edgeaiModule`。
- **SetOrderBeginBlockers/EndBlockers/InitGenesis**：追加 `edgeaimoduletypes.ModuleName`；InitGenesis 顺序置于 `phonenodemoduletypes.ModuleName` 之后（依赖 phonenode+depin 已 Init）。
- **initParamsKeeper**：追加 `paramsKeeper.Subspace(edgeaimoduletypes.ModuleName)`。
- **maccPerms**：**不变**（edgeai 不持有资金、无模块账户）。
- **config.yml**：**不变**（沿用 B1 决定，不新增币；edgeai params 走 genesis 而非 config.yml）。

## 3. 数据结构与接口（类图，见 `docs/b3_edgeai_class-diagram.mermaid`）

**proto/结构要点**
```proto
// x/edgeai/types/tx.proto 新增
message MsgCreateTask   { string creator=1; string spec=2; string reward_amount=3; } // reward_amount 单位 umc
message MsgAssignTask   { string creator=1; string task_id=2; }
message MsgSubmitResult { string creator=1; string task_id=2; string proof_root=3; string result_hash=4; }
message MsgSubmitDispute{ string creator=1; string task_id=2; string reason=3; }
message MsgResolveDispute { string resolver=1; string task_id=2; bool cheated=3; }
// x/edgeai/types/query.proto 新增
message QueryTaskRequest { string id=1; }   message QueryTaskResponse { Task task=1; }
message QueryResultRequest { string task_id=1; } message QueryResultResponse { Result result=1; }
message QueryDisputeRequest { string id=1; }    message QueryDisputeResponse { Dispute dispute=1; }

message Task    { string id=1; string creator=2; string spec=3; string reward_amount=4; string status=5; string assigned_to=6; int64 created_height=7; int64 dispute_until=8; }
message Result  { string task_id=1; string executor=2; string proof_root=3; string result_hash=4; bool attested=5; string status=6; int64 submitted_height=7; }
message Dispute { string id=1; string task_id=2; string challenger=3; string reason=4; string status=5; bool cheated=6; int64 resolve_height=7; }
```
**edgeai params（新增）**：`dispute_period_blocks`、`anti_cheat_dup_window`、`slash_bps_on_cheat`、`challenger_reward_bps`、`max_tasks_per_node`、`resolver_address`。
**核心方法**：`edgeai.Keeper.{CreateTask,AssignTask,SubmitResult,SubmitDispute,ResolveDispute}`；`depin.Keeper.PayOutFromPool`；`phonenode.Keeper.{IsAttested,SlashIfBad,HasNode}`（B2）。

## 4. 程序调用流程（时序图，见 `docs/b3_edgeai_sequence-diagram.mermaid`）

- **① 任务发布→领取→提交**：`CreateTask`(→open) → `AssignTask`(校验 IsAttested + 未重复领取 →assigned) → `SubmitResult`(校验 IsAttested(executor)+HasNode → 写 Result(pending)，设 DisputeUntil)。
- **② optimistic 自动接受 + 拨付**：`BeginBlock` 扫描 `pending` 且 `cur >= DisputeUntil` 且无未决 `Dispute` → `acceptResult` → `depinKeeper.PayOutFromPool(executor, reward_amount)`（从 depin 池拨付，不 mint）。
- **③ 争议 → 裁定 → slash/奖励**：`SubmitDispute`(挑战者须 attest 节点) → `MsgResolveDispute`(resolver=治理/仲裁账户)：cheated=true → `phonenodeKeeper.SlashIfBad(executor,"edgeai_cheat")` + `PayOutFromPool(challenger, challengerReward)`；cheated=false → `SlashIfBad(challenger,"edgeai_false_dispute")`。**无 MintCoins**。
- **④ 与 B1 边界**：拨付经 depin 模块账户余额（B1 生态池切片），`minted_supply` 不变；`q tokenomics allocations` 中生态池余额减少可见（生态池→depin 切片→节点）。

## 5. 任务列表（有序、含依赖、优先级）
| Task | 名称 | 依赖 | 优先级 | 涉及文件 |
|------|------|------|--------|----------|
| **T1** | edgeai 脚手架 + params/genesis/keys 占位 | 无（设计上依赖 B2 已落地） | P0 | `x/edgeai/types/*`、`genesis.go`、`module.go`、`client/cli/*`、`tx.proto`/`query.proto` |
| **T2** | depin 新增 `PayOutFromPool` 拨付桥 | T1 | P0 | `x/depin/keeper/payout.go` |
| **T3** | 任务状态机（Create/Assign/SubmitResult + 查询/CLI） | T1,T2 | P0 | `x/edgeai/keeper/task.go`、`result.go`、`msg_server_*.go`、`query_*.go` |
| **T4** | anti-cheat 争议（SubmitDispute/ResolveDispute + BeginBlock 自动接受 + payout 桥接） | T3 | P0 | `x/edgeai/keeper/dispute.go`、`payout_bridge.go`、`module.go(BeginBlock)` |
| **T5** | app.go 装配（ModuleBasics/storekey/mm 顺序/subspace）+ 模拟测试 | T1-T4 | P0 | `app/app.go`、`x/edgeai/genesis_test.go`、`x/edgeai/keeper/*_test.go` |
**P0 验收锚点**：无 attestation 的结果提交被拒；窗口到期无争议自动拨付且 `minted_supply` 不变；作弊裁定触发 `phonenode.SlashIfBad` 事件；`q edgeai task/result/dispute` 可见；拨付计入生态池（`q tokenomics allocations` 可见生态池余额下降）。

## 6. 依赖包列表
- `github.com/cosmos/cosmos-sdk` v0.47.3（已含 bank/staking/slashing/params）；`x/phonenode`、`x/depin`、`x/tokenomics`（已落地）。**无需新增任何第三方依赖**。

## 7. 共享知识（跨文件约定，含与 B1 交互边界）
- **denom/单位**：同 B1（`umc`，`1 MC=1e6 umc`）；`reward_amount` 以 `umc` 字符串表达，解析为 `sdk.Int`。
- **不破 cap（铁律）**：edgeai **绝不调用 `tokenomics.MintCoins`**；奖励 100% 经 `depin.PayOutFromPool` 从 depin 模块账户余额（= B1 生态池 `DepinInitialPoolSlice=1e14 umc` 切片）拨付；`minted_supply` 不受 edgeai 影响（B1 记账不变）。
- **attestation 前置**：`SubmitResult` / `AssignTask` / `SubmitDispute` 均强制 `phonenodeKeeper.IsAttested(addr)`（B2 提供），与 depin `HasNode`（`SubmitResult` 同时校验）形成双闸口。
- **slash 复用 B2**：裁定作弊仅调 `phonenodeKeeper.SlashIfBad`（扣自质押/Jail/吊销 attestation，详见 B2），不新增 slash 路径、不 mint。
- **depin 拨付接口约定**：`PayOutFromPool(ctx, toAddr, amt)` 仅做 `bankKeeper.SendCoinsFromModuleToAccount(depinModule, toAddr, amt)`；调用方须保证 depin 模块账户余额充足（= InitialPool 切片），不足即报错（由 B5 观测/补充配额治理处理）。
- **争议仲裁权限**：v1 `MsgResolveDispute.resolver` 限定为 `Params.ResolverAddress`（默认复用 B1 团队多签占位），主网前由 B6 迁 DAO 治理。
- **事件约定**：`edgeai.TaskCreated` / `edgeai.ResultAccepted` / `edgeai.Dispute` / `edgeai.Resolved`（type 命名供 B5 订阅）；`phonenode.Slash` 由 B2 已定义。

## 8. 待明确事项（仅真正需后续定稿，占位并注明）
1. **真实 AI 执行证明格式 / 可验证计算方案**：链上仅存 `proof_root`+`result_hash`，重验证（如 TEE 证明、ZK 证明、复算挑战）的具体链下/预言机职责列为 B4/B5 链下契约，本批不实现。
2. **争议仲裁去中心化**：v1 用 `ResolverAddress`（团队多签占位）集中裁定；DAO 化方案（B6 治理 + 陪审质押）占位，主网前定稿。
3. **depin 生态池余额补充/再平衡**：若 1e14 切片耗尽，补充机制（治理参数调整或新拨付提案）列为 B6 治理事项，本批不实现自动补池。
4. **团队多签真实密钥 / Resolver 账户**：沿用 B1 占位（主网前替换）。

> 其余均按推荐默认定稿，未向用户抛待确认。
