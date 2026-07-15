# MobileChain B5 批次 · 生态工具（SDK / 浏览器 / 真实 DePIN / 移动钱包，链下快速模式）· 增量 PRD

**文档类型**：增量 PRD（基于现有 `mcchain` 代码，仅描述变更，不含实现代码）
**批次**：B5（生态工具）— 归属路线图生态扩展部分
**作者**：许清楚（Xu），Product Manager
**语言**：简体中文
**验收总原则**：沙箱无法运行 `go`/`protoc`，本 PRD 不写代码、不执行命令；验收一律以用户本机 `ignite chain build` + 链上观测（事件订阅、查询字段覆盖）+ 链下契约评审为准。

**产品目标（一句话）**：在 B1–B4 链上能力之上，补齐生态工具所需的链上查询/事件接口；JS/Python SDK、区块浏览器、真实 DePIN 需求、移动钱包按「链下快速模式」只交付接口契约与验收标准，不实现链下代码。

## 0. 背景现状（已侦察，采信）
- B1–B4 提供链上能力：tokenomics 查询、phonenode（含 attestation/弱网）、edgeai（任务/结果/anti-cheat）、安全（slash/治理）。
- 生态工具（SDK/浏览器/钱包/DePIN 接入）属链下工程；本批次链上仅做「接口与事件补齐」，链下给契约。
- 路线图书面方向：移动端贡献即挖矿、主流 DePIN 方向。

## 1. 增量范围说明（基于现有 mcchain，仅列变更）
- **链上（本批次实现）**：补齐统一查询/事件接口——tokenomics/phonenode/edgeai 的 gRPC query 字段覆盖 + 标准化事件（mint、分配、slash、任务、结果、attestation、治理）供订阅。
- **链下快速模式（仅契约 + 验收）**：JS/Python SDK、区块浏览器、真实 DePIN 需求样例、移动钱包——各自接口契约 + 验收标准，不实现。

## 2. 需求池（P0–P2）

### R1 · 标准化事件接口（P0 / Must have）
- **需求描述**：各模块关键事件统一结构与 topic（mint/分配/slash/任务/结果/attestation/治理），可外部订阅。
- **验收标准**：
  - 事件可被消费（用户本机 query tx events / 区块浏览器）；结构一致。
- **关键约束**：用 cosmos 标准 Event + 自定义 type；模块状态变更时 EmitEvent。

### R2 · 统一查询接口（P0 / Must have）
- **需求描述**：tokenomics/phonenode/edgeai 查询字段满足 SDK/浏览器需求（余额、分配、任务、节点状态、证明状态）。
- **验收标准**：
  - `mcchaind q ...` 覆盖生态所需字段；autocli/gRPC 自动暴露 + 必要聚合 query。
- **关键约束**：仅查询，不改状态。

### R3 · SDK 接口契约（链下，P1 / Should have）
- **需求描述**：JS/Python SDK 的端点/消息契约 + 验收（查询余额、提交任务、监听事件）。
- **验收标准**：
  - 契约完整，链上端点满足。
- **关键约束**：链下不实现。

### R4 · 浏览器/钱包/真实 DePIN 契约（链下，P1 / Should have）
- **需求描述**：区块浏览器（消费事件/状态）、移动钱包（轻客户端集成，联动 B4）、真实 DePIN 需求样例（联动 B3 edgeai 任务类型）的接口契约 + 验收。
- **验收标准**：
  - 契约完整，覆盖对应链上能力。
- **关键约束**：链下不实现。

### R5 · 示例/脚手架（链下，P2 / Nice to have）
- **需求描述**：提供示例查询脚本/接入样例（链下），验证契约可跑通。
- **验收标准**：
  - 样例文档完整。

## 3. 关键设计建议（给架构师）
1. **链上只做接口与事件补齐**，不写链下代码。
2. **事件**：tokenomics/phonenode/edgeai 在状态变更时 EmitEvent，统一 type 命名（如 `mcchain.tokenomics.Mint`）。
3. **查询**：优先 autocli/gRPC 自动暴露；为生态总览加必要聚合 query（如 `q ecosystem overview`）。
4. **链下契约**：OpenAPI + proto + 验收清单；明确链下不在本批次。
5. **与 B3/B4 衔接**：真实 DePIN 需求契约复用 B3 任务类型；钱包契约复用 B4 轻客户端 proof。

## 4. 文件清单（新增/修改）
- 修改：`x/tokenomics`、`x/phonenode`、`x/edgeai` keeper（EmitEvent + query 字段）、`app/app.go`（autocli 已注册）、事件 topic 约定文档。
- 新增（链下，文档）：`docs/sdk_contract.md`、`docs/explorer_contract.md`、`docs/wallet_contract.md`、`docs/depin_demand_contract.md`（各自契约 + 验收）。
- 不变：B1–B4 业务逻辑。

## 5. 验收总原则
- 沙箱无法运行 `go`/`protoc`，本 PRD 不写代码、不执行命令。
- 验收一律以用户本机 `ignite chain build` + 链上观测（事件可订阅、查询覆盖字段）+ 链下契约评审为准。
- 链下实现不在本批次，以契约完整性与链上端点满足契约为验收。
