# MobileChain B2 批次 · 安全（硬件 attestation 反女巫 / Slashing / 最小链上治理）· 增量 PRD

**文档类型**：增量 PRD（基于现有 `mcchain` 代码，仅描述变更，不含实现代码）
**批次**：B2（安全）— 归属路线图「阶段四 经济模型与安全」的安全部分
**作者**：许清楚（Xu），Product Manager
**语言**：简体中文
**验收总原则**：沙箱无法运行 `go`/`protoc`，本 PRD 不写代码、不执行命令；验收一律以用户本机 `ignite chain build` + `go test` + 链上观测（`mcchaind q ...`、事件监听）为准。

**产品目标（一句话）**：在 B1 经济模型之上补上链上安全底座——硬件 attestation 反女巫、Slashing 惩罚、最小链上治理，使移动节点「真实可信、作恶必罚、关键参数可治理」。

## 0. 背景现状（已侦察，采信）
- B1 已落地经济模型链上基础：新增 `x/tokenomics`（总量 cap、分配、释放、查询），`config.yml` 不额外加三大池币、拨付由 tokenomics `InitGenesis` 程序化完成。
- 当前安全红线尚未实现：无硬件 attestation 反女巫、无 Slashing 落地、治理仅基础 `gov` 模块装配（无针对安全/经济参数的专用提案钩子）。
- 代码事实（B1 已验证 + 本批次沿用）：`app/app.go` 已 import 并装配 `slashing`、`gov`、`mint`、`crisis`、`staking`；`x/depin` 与 `x/phonenode` 已存在且 depin→phonenode 关联校验已落地（P2）；`x/phonenode` 为移动端节点模块。
- 路线图：B2 为「阶段四」安全部分；DAO 级治理留待 B6。

## 1. 增量范围说明（基于现有 mcchain，仅列变更）
- **新增模块（推荐默认）**：默认**扩展 `x/phonenode` 增加 attestation 子状态**（避免新增模块带来的装配复杂度），承载硬件 attestation 证明登记与反女巫校验。
- **Slashing 接线**：复用既有 `x/slashing` + `x/staking`；定义 slash 条件与比例参数，由 `x/phonenode`/`x/depin` 在检测到恶意/离线时通过 keeper 接口触发（或经 gov 设定阈值后 begin blocker 触发）。
- **最小链上治理**：在既有 `gov` 基础上，开放对安全/经济关键参数（slashing 比例、女巫阈值、离线宽限等）的 param change 提案；DAO/复杂治理留 B6。
- **genesis 调整**：固化安全参数（slashing 比例、attestation 要求、离线宽限）；`x/phonenode` genesis 增加 attestation 占位。
- **查询/事件**：`mcchaind q phonenode ...` 暴露 attestation 状态与 slash 记录；slash/attestation 失败事件上链。
- **不变**：B1 经济模型（cap/分配/释放）语义不变；B3 edgeai、B4 客户端、B5 生态链下部分不在本批次实现。

## 2. 需求池（P0–P2）

### R1 · 硬件 attestation 反女巫（P0 / Must have）
- **需求描述**：移动节点（phonenode）在注册与领取生态奖励前，须提交硬件 attestation 证明（证明运行于真实移动设备/可信执行环境）。链上登记证明根/哈希与一次性 nonce（防重放），未通过校验的节点不能注册或领取。
- **验收标准**：
  - 未提交有效 attestation 的 phonenode 注册被拒；已注册但证明失效/过期的节点在领取时被拦截。
  - `mcchaind q phonenode attestation <addr>` 返回证明状态（valid/pending/invalid）与登记时间。
  - 反女巫生效：同一设备标识（绑定 attestation）无法以多账户重复领取（与 B1 质押门槛 `MinSelfDelegation` 形成双重防女巫）。
- **关键约束**：链上仅存证明根/哈希与 nonce，原始证明链下验证或轻量校验（受链上算力约束）；attestation 状态须可被 `x/edgeai`（B3）复用，作为任务领取的前置条件。

### R2 · Slashing 基础（P0 / Must have）
- **需求描述**：定义 slash 触发条件（长期离线、伪造贡献/作弊、attestation 伪造）与 slash 比例，触发后扣减对应质押与/或生态拨付，并上链事件。
- **验收标准**：
  - 模拟离线超宽限 / 伪造贡献，触发 slash 事件；被 slash 账户的质押余额与（若涉及）生态池拨付正确扣减。
  - slash 不产生新增 mint；`minted_supply` 不受 slash 影响（slash 为 burn/扣留，不突破 B1 cap）。
  - `mcchaind q slashing params` 与 `mcchaind q phonenode slashes <addr>` 可见参数与记录。
- **关键约束**：slash 比例参数须可由最小治理（R3）调整；与 B1 `x/tokenomics` 记账衔接——生态池拨付被 slash 扣留时，已 mint 计数不变（仅余额减少）。

### R3 · 最小链上治理（P1 / Should have）
- **需求描述**：在既有 `gov` 模块上，支持对安全/经济关键参数（slashing 比例、女巫阈值、离线宽限、B1 相关可调参数）的 param change 提案与投票；采用 validator 集 + 治理多签投票。
- **验收标准**：
  - 提交参数变更提案 → 投票期 → 通过后链上参数生效（以 `mcchaind q` 验证）。
  - 治理不触及 B1 `total_supply_cap`（本批次锁定为常量，防超发）；仅开放明确列出的可调参数。
  - 文本提案可用，作为社区 signaling。
- **关键约束**：仅最小治理；DAO/社区池支配/timelock 等留 B6；提案处理的权限边界需与 B1 设计一致。

### R4 · 安全事件可观测（P2 / Nice to have）
- **需求描述**：slash、attestation 失败/过期、治理提案生效等关键事件上链，供 SDK/浏览器（B5）订阅。
- **验收标准**：事件具备统一 type/topic，可在用户本机通过 query tx events 或区块浏览器观测。
- **关键约束**：仅事件发射，不改变既有状态机。

## 3. 关键设计建议（给架构师）
1. **attestation 落地位置**：默认扩展 `x/phonenode` 增加 `Attestation` 状态（证明根、nonce、过期时间、设备标识哈希），而非新建模块；新增 `MsgSubmitAttestation` 与查询。链上校验只验根/nonce，重验证在客户端/预言机侧。
2. **Slashing 接线**：复用 `x/slashing`；在 `x/phonenode` keeper 提供 `SlashIfBad(ctx, addr, reason)` 封装，内部调用 `stakingKeeper.Slash`/`Jail`；begin blocker 检测心跳超时触发离线 slash（宽限参数化）。作弊（伪造贡献）由 `x/depin`/`x/edgeai`（B3）在拨付/结果校验路径调用。
3. **最小治理**：复用 cosmos `gov v1`；param change 提案通过 `x/params` subspace 修改 `x/phonenode`、`x/slashing`、`x/depin` 的可调参数；`app.go` 的 `govRouter` 已注册 `paramproposal`。
4. **与 B1 衔接**：slash 扣留/扣减不影响 `minted_supply`；生态池拨付前由 depin 校验 phonenode attestation 状态（depin→phonenode 依赖已存在，扩展校验即可）。
5. **genesis 顺序**：`x/phonenode` 已在 mcchain 之后、depin 之后；attestation 状态在 phonenode InitGenesis 内初始化。

## 4. 文件清单（新增/修改）
- 修改：`x/phonenode/types/params.go`（新增安全阈值/离线宽限/attestation 要求参数）、`x/phonenode/types/genesis.go`、`x/phonenode/genesis.go`（初始化 attestation 状态）、`x/phonenode/keeper/`（attestation 登记/校验、slash 封装、心跳/离线检测 begin blocker）、`x/phonenode/keeper/msg_server.go`（新增 `MsgSubmitAttestation`）、`x/phonenode/module/`（gov proposal 注册如需）。
- 修改：`x/depin/keeper/`（拨付前校验 phonenode attestation）、`app/app.go`（若有新 keeper 方法或 subspace 调整；治理路由已具备）。
- 修改/新增：`x/phonenode/client/cli/`（查询 attestation/slashes）、`docs/`（安全参数说明）。
- 不变：B1 `x/tokenomics` 业务逻辑；`config.yml` 不新增币（沿用 B1 决定）。

## 5. 验收总原则
- 沙箱无法运行 `go`/`protoc`，本 PRD 不写代码、不执行命令。
- 验收一律以用户本机 `ignite chain build` + `go test`（含 slash/attestation 单元与模拟测试）+ 链上观测（`mcchaind q phonenode ...`、`mcchaind q slashing ...`、事件监听）为准。
- 安全属性硬验收：任意路径下 `minted_supply <= total_supply_cap` 不被破坏（slash 不增 mint）。
