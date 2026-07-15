# MobileChain B4 批次 · PhoneNode（链上轻量同步 + 弱网参数；客户端 App 链下快速模式）· 增量 PRD

**文档类型**：增量 PRD（基于现有 `mcchain` 代码，仅描述变更，不含实现代码）
**批次**：B4（PhoneNode 轻量接入）— 归属路线图移动端接入部分
**作者**：许清楚（Xu），Product Manager
**语言**：简体中文
**验收总原则**：沙箱无法运行 `go`/`protoc`，本 PRD 不写代码、不执行命令；验收一律以用户本机 `ignite chain build` + `go test` + 链上观测（`mcchaind q ...`、proof 查询）为准。

**产品目标（一句话）**：让移动端在弱网/低电量条件下也能低成本接入链——链上固化状态裁剪、snapshot、弱网带宽电量参数与轻客户端状态证明；客户端 App 本批次按「链下快速模式」只交付接口契约与验收标准。

## 0. 背景现状（已侦察，采信）
- `x/phonenode` 已存在（B2 已加 attestation）；B1 经济、B2 安全、B3 edgeai 提供业务与安全底座。
- 移动端场景特征：弱网、高延迟、电量敏感、存储有限——需要轻客户端、状态裁剪、快照、节流参数。
- 客户端 App（移动端钱包/节点壳）属链下工程；本批次不实现 App，只给链上需支撑的能力 + 链下接口契约。

## 1. 增量范围说明（基于现有 mcchain，仅列变更）
- **链上（本批次实现）**：
  - 状态裁剪/pruning：固化移动端友好默认值（pruning strategy/intervals），可经参数调。
  - Snapshot：启用/配置状态快照，便于移动端快速初始化与追赶。
  - 弱网/带宽/电量参数：`x/phonenode` params 新增带宽上限、心跳间隔、离线宽限、同步节流等。
  - 轻客户端状态证明：暴露带 IAVL/ICS proof 的状态查询端点（账户/余额/任务/节点状态）。
- **链下快速模式（仅契约 + 验收，不实现）**：客户端 App↔链 的 gRPC/REST 接口契约、消息格式、同步与重连协议、弱网/低电量验收标准。

## 2. 需求池（P0–P2）

### R1 · 状态裁剪与 snapshot（P0 / Must have）
- **需求描述**：链支持 pruning + snapshot，移动端可快速初始化与同步，不全量重放。
- **验收标准**：
  - 配置 pruning 后链正常运行、状态一致；snapshot 可生成并被轻客户端/外部消费；`mcchaind q` 不受影响。
- **关键约束**：复用 cosmos SDK 既有 pruning/snapshot 能力，B4 负责移动端默认值固化 + 查询暴露。

### R2 · 弱网/带宽/电量参数（P0 / Must have）
- **需求描述**：`x/phonenode` params 含带宽上限、心跳间隔、离线宽限、同步节流；移动端省电省流量。
- **验收标准**：
  - 参数可配置且生效（心跳/离线检测按参数运行）；`mcchaind q phonenode params` 可见；可由 B2 治理调整。
- **关键约束**：离线宽限须与 B2 slash 宽限协调（避免误 slash）。

### R3 · 轻客户端状态证明（P1 / Should have）
- **需求描述**：暴露 ICS/状态证明查询，移动端可验证账户/余额/任务状态而无需全节点。
- **验收标准**：
  - 轻客户端可用证明验证关键状态（余额、节点 attestation、任务状态）；proof 可被独立验证。
- **关键约束**：复用 cosmos light client / IAVL proof；新增便捷 query 返回带 proof 的状态。

### R4 · 客户端 App 接口契约（链下快速模式，P2 / Nice to have）
- **需求描述**：给出 App↔链 的 gRPC/REST 端点清单、消息契约、同步/重连/节流协议、弱网低电量验收标准。
- **验收标准**：
  - 契约文档完整，链上端点满足契约（App 实现留待链下工程）。
- **关键约束**：链下不在本批次实现；契约须覆盖 R1–R3 的链上能力。

## 3. 关键设计建议（给架构师）
1. **pruning/snapshot**：以 `app.toml` 默认值 + 启动参数固化移动端友好配置；snapshot 用 cosmos snapshot store。
2. **弱网参数**：放 `x/phonenode` params（已有 params 机制），begin blocker 用心跳检测离线（与 B2 slash 宽限共用参数）。
3. **轻客户端 proof**：复用 cosmos 的 `abci.Query` proof 模式 + IAVL；为 tokenomics/phonenode/edgeai 关键状态提供带 proof 的查询。
4. **链下契约**：以 OpenAPI + proto + 验收清单文档表达，明确「链下不在本批次实现，仅定义契约」。

## 4. 文件清单（新增/修改）
- 修改：`x/phonenode/types/params.go`（弱网参数）、`x/phonenode/genesis.go`、`x/phonenode/keeper/`（心跳/离线检测、proof 查询）、`app/app.go`（pruning/snapshot 默认配置）、`config/`（app.toml 默认）、`x/phonenode/client/cli/`（proof 查询）。
- 新增（链下，文档）：`docs/client_app_contract.md`（gRPC/REST 契约 + 同步/重连协议 + 弱网低电量验收标准）。
- 不变：B1–B3 业务逻辑。

## 5. 验收总原则
- 沙箱无法运行 `go`/`protoc`，本 PRD 不写代码、不执行命令。
- 验收一律以用户本机 `ignite chain build` + 链上观测（pruning/snapshot 生成、params 查询、proof 查询）+ 链下契约评审为准。
- 链下 App 实现不在本批次，以契约完整性与链上端点满足契约为验收。
