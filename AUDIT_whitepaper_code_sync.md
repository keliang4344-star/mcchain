# MC 白皮书 ↔ 代码 双向一致性审计

> 审计对象：`WHITEPAPER.md`（根目录 EN 版，附录 A 为权威参数表）
> 代码基线：`E:/mc-build/mcchain` @ `main`（最近提交 `29a2cdc`）
> 审计日期：2026-08-05
> 审计原则：**只列清单、不改代码/文档**，待决策后做第二轮修整。

---

## 0. 结论速览

| 维度 | 结果 |
|------|------|
| 附录 A 全部数值常量（约 45 项） | ✅ **全部与代码一致** |
| 主要 `Status: Live / Delivered` 机制（slash 分流、drip 续期、gas 销毁回流、企业费分账、DEX 销毁、推荐、节点日津贴、云端共签、离链结算批、治理移交、流动性质押、CosmWasm、IBC、解锁曲线、预言机） | ✅ **均已落地，与白皮书一致** |
| 白皮书↔代码 不同步项 | ⚠️ **共 6 项**，其中中等 1 项、低 5 项 |
| 严重数值错误 / 直接矛盾 | ❌ 未发现 |

> 销毁口径（这是前几轮反复改动的重心）已完全对齐：gas 7% / DEX 50%（成交额 0.15%）/ 罚没 40% 三处销毁，均以代码常量为准，白皮书已统一，**无残留旧口径（如 0.05%）**。

---

## 1. 正向核对（白皮书说的，代码做没做）

### 1.1 已验证一致（节选，全部 PASS）

| 白皮书位置 | 声明 | 代码真值 | 状态 |
|---|---|---|---|
| §5 三处销毁 | gas 7% / DEX 50% / 罚没 40% | `GasBurnRatioBps=700` / `FeeBurnBps=5000` / `SlashBurnRatioBps=4000` | ✅ |
| §6 五池占比 | 55/15/12/13/5 | `DeviceIncentivePercentBps=5500` 等 | ✅ |
| §6 推荐预算 | 8250 万 MC（占设备池 15%） | `ReferralEcosystemBudget=82.5e12` | ✅ |
| §6 基金会 | T0 解锁 5000 万 + 2 年线性 8000 万 | `FoundationT0Unlock=50e12` + `CreateVestingAccount(2yr)` | ✅ |
| §6 团队 | 3-of-5 多签 + 1 年 cliff + 3 年线性 | `CreateTeamVestingAccount(cliff 1yr, linear 3yr)` | ✅ |
| §7 drip 续期 | 100 块/次；A 耗尽后由国库 B 以 1–2% APR 续期 | `DripIntervalBlocks=100` / `DripWithRenewal` + `RenewalFloorAPRBps=100/200` | ✅ |
| §8.1 gas | 7% 销毁 + 10% 回流安全池 | `GasBurnRatioBps=700` / `GasRebateRatioBps=1000` | ✅ |
| §8.2 DEX | 0.30% 手续费，50% 销毁(0.15%)/50% 给 LP | `FeeBurnBps=5000` / `FeeLPBps=5000` | ✅ |
| §8.3 企业费 | 1.5%，40% 节点 / 60% 国库，EdgeAI 路径 | edgeai `msg_server_create_task.go` 40%→fee_collector / 60%→`protocol_treasury` | ✅ |
| §9.1 罚没分流 | 40% 烧 / 60% 回流安全池 | `phonenode/keeper/slash.go` `slashAndRoute` → 黑洞 + 安全池 | ✅ |
| §9.3 设备层罚则 | 离线 5% / 作弊 10% / 伪造 20% / 宽限 100 / 冷却 43200 | `phonenode/types/params.go` 默认值全对 | ✅ |
| §9.4 认证 | 30 天有效、过期暂停设备奖励但不重置推荐关系 | `AttestationValidity=2592000`；推荐关系独立（代码未因认证过期重置） | ✅ |
| §10 设备层 | 质量门槛 30；节点日津贴 | `ContributionThreshold=30`；`phonenode/keeper/node_allowance.go` 30MC/节点/日（默认启用） | ✅ |
| §11 推荐 | 3 级 10/5/2%；日上限 500/全网 20600；最小发放 100；最大 100 推荐 | `MaxReferralDepth=3` / `DefaultLevel1..3=1000/500/200` / caps 全对 | ✅ |
| §12 验证人门槛 | 最低自抵押 3 万 MC | `app/ante.go` `MinSelfDelegationLowerBound=3e10` | ✅ |
| §12 流动性质押 | Live | `x/liquidstaking` 模块存在且接入 `app.go` | ✅ |
| §12 云端共签 | Live | `x/phonenode/keeper/cosign.go` + CLI | ✅ |
| §13 EdgeAI | 提交 85% / 验证 15%；单任务上限 1000MC；争议窗口 100；反作弊 50% | `EdgeAISubmitterRatioBps=8500` / `EdgeAIVerifierReserveRatioBps=1500` / `MaxTaskReward=1e9` / `DisputePeriodBlocks=100` / `AntiCheatThresholdBps=5000` | ✅ |
| §14 DEX | 初始 500 万 MC 流动性；7 天锁仓 | `DexInitialLiquidityMC=5e12`；`LpLockBlocksDefault=100800`（见 M#2） | ⚠️ 锁仓时长见 M#2 |
| §15 预言机 | Live | `internal/oraclesvc` 存在 | ✅ |
| §16 治理移交 | 渐进移交 + 时间锁（见 M#1） | `x/mcchain` `CmdInitiateHandover`/`CmdCompleteHandover` | ✅（但 timelock 见 M#1） |
| §17 CosmWasm | 经 x/wasm 集成，CGO 构建启用 | `wasmd v0.45` 接入 `app.go`（CGO 构建注册 wasm AppModule） | ✅ |
| §18 IBC | 经 ICA + IBC transfer 交付 | `ibc-go/v7` 接入 `app.go`（ica host/controller + transfer） | ✅ |
| §18 离链结算批 | 交付 | `x/dex` `CmdSubmitSettlementBatch`/`FinalizeSettlementBatch` | ✅ |
| §A.1 版本 | SDK v0.47.14 / CometBFT v0.37.6 / Go 1.21 / wasmd v0.45 / wasmvm v1.5 / ibc-go v7 | `go.mod` 完全一致 | ✅ |

---

## 2. 不同步清单（待第二轮修整）

> 分级：🔴 高（直接矛盾/安全误导）｜🟠 中（声明与实现不符，需决策）｜🟡 低（文档完整度/措辞）

### M#1 🟠 国库"时间锁(timelock)"声明与实现不符
- **白皮书原文**：
  - §8.4："It is spent only through governance, behind a multisig and a **timelock**."
  - §16："Treasury spending requires both a multisig and a **timelock**, so no single approval moves funds…"
- **代码真值**：全仓 `.go` 仅 `app/app.go:233` 一行注释 `// Spent via governance multisig + timelock.`，**无任何 timelock 强制逻辑 / 模块**。
- **佐证**：内部设计文档明确把 timelock 列为"v1 不做 / 可选 / 路线图"——
  - `docs/system_design_b6_mainnet.md:107`："timelock 模块：v1 不做"
  - `docs/dao_roadmap.md:32`："timelock（可选）"
  - `docs/prd_b6_mainnet.md:55`："timelock（可选新模块）"
- **严重度**：🟠 中。白皮书把"时间锁"写成硬约束，但代码未实现；属安全/治理声明的过度承诺。
- **建议修法（二选一，待猴哥决策）**：
  1. **实现**：新增 `x/timelock` 或治理提案执行延迟，真正强制国库支出走时间锁；或
  2. **收敛措辞**：白皮书改为"国库由治理多签支配，时间锁为建设路线图项（尚未启用）"，与 `dao_roadmap.md` 对齐。

### M#2 🟡 DEX 锁仓"7 天"与实际块数在 4s 出块下≈4.7 天
- **白皮书原文**：§14 "Liquidity positions carry a **seven-day** minimum lock."
- **代码真值**：`x/dex/types/params.go:15` `LpLockBlocksDefault = 100800`，注释写"7 days at **~6s/block**"。
- **矛盾点**：白皮书自身 §A.1 / §7 声称出块约 **4s**。按 4s/块，`100800 × 4 = 403200s ≈ 4.67 天`，并非 7 天。即：DEX 代码注释假定 6s 出块，与白皮书主文档的 4s 出块自相矛盾。
- **附带不一致**：§A.1 "Block time ~4 s" 与 DEX 注释"~6s/block" 是同一问题的一体两面。
- **严重度**：🟡 低。仅时长差约 2.3 天，不影响"有锁仓"这一事实，但数字不自洽。
- **建议修法（待决策）**：
  1. 若坚持"7 天"且出块确为 4s：将 `LpLockBlocksDefault` 改为 `151200`（7×24×3600/4）；或
  2. 若维持 100800：白皮书改为"约 4.7 天锁仓"，并统一全文档出块时间表述（4s）。

### M#3 🟡 §4 架构图仅列 6 个模块，与"八个原生模块"口径不符
- **白皮书原文**：§4 应用层图只画了 `tokenomics/depin/phonenode/edgeai/dex/referral` 6 个自定义模块；但 §18 路线图写 "**Eight native modules**"，实际 `x/` 下确有 8 个（`+liquidstaking`、`+mcchain`）。
- **代码真值**：存在 `x/liquidstaking`、`x/mcchain`（含治理移交），均与白皮书其他处"Live/Delivered"一致。
- **严重度**：🟡 低。章节内图例不全，非数值错误。
- **建议修法**：在 §4 图中补 `liquidstaking` 与 `mcchain`（系统锚定模块），或在图注标注"含流动性质押与系统锚定模块，共八模块"。

### M#4 🟡 附录 A.9 代码地图漏列已交付模块
- **白皮书原文**：附录 A.9 列出 mcchain/tokenomics/depin/phonenode/edgeai/dex/referral/ante/oracle/dashboard，**未列** liquidstaking、wasm(CosmWasm)、ibc(ibc-go)、治理移交(mcchain handover)。
- **代码真值**：上述四者均已接入 `app.go` 并标记为 Live/Delivered（见 1.1）。
- **严重度**：🟡 低。附录完整度问题，不影响参数正确性。
- **建议修法**：A.9 增补 `liquidstaking`、`wasm (wasmd)`、`ibc (ibc-go)`、`governance handover (x/mcchain)` 四行。

### M#5 🟡 §10 "节点日津贴来自设备池"的归口措辞
- **白皮书原文**：§10 "Attested, active nodes also receive a daily node allowance **from the device pool**."
- **代码真值**：日津贴实际由 `x/phonenode/keeper/node_allowance.go` 管理（`DefaultNodeCapitalAllowancePerDay=30MC`），资金从设备激励总盘切出、由 phonenode 模块按日发放，而非由 `x/depin` 设备池直接拨付。
- **严重度**：🟡 低。功能与数值(30MC/节点/日)均对，仅"资金归口"措辞略粗。
- **建议修法**：白皮书可改为"节点建设溢价日津贴（源自设备激励总盘，由 phonenode 模块按日发放）"，与代码模块边界一致。

### M#6 🟡 代码有、白皮书未点名：黑洞销毁地址
- **代码真值**：`tokenomics/types/keys.go` `BlackHoleAddress()` 由模块名 `black_hole` 经 sha256 确定性派生、无私钥、创世即存在，是全链唯一销毁去向（gas/DEX/罚没均打入此处）。
- **白皮书现状**：§5 仅抽象描述"永久减少流通量 / permanent reduction against the fixed cap"，**未把"black_hole 模块账户"作为可公开查询的权威销毁地址点出**。
- **严重度**：🟡 低（反向缺失，非矛盾）。
- **建议修法**：在 §5 或附录补充"所有销毁统一进入确定性派生的黑洞地址 `black_hole`，任何人可随时查询其余额作为累计销毁量"。

---

## 3. 反向核对（代码有、白皮书是否写了）

- 代码已实现且白皮书已在对应 `Status: Live/Delivered` 处记载的：slash 分流、drip 续期、gas、企业费、DEX 销毁、推荐、节点日津贴、共签、离链批、移交、流动性质押、CosmWasm、IBC、解锁、预言机 —— **均已双向覆盖**。
- 唯一反向缺口即 **M#6（黑洞地址未点名）** 与 **M#4（A.9 漏列模块）**，均为"可补充"级，无遗漏的重大已实现未记载机制。

---

## 4. 第二轮修整建议优先级

| 优先级 | 项 | 动作类型 |
|---|---|---|
| 1 | M#1 timelock | 决策：实现 或 收敛措辞 |
| 2 | M#2 DEX 锁仓时长 | 决策：改常量 或 改文案，并统一出块时间表述 |
| 3 | M#3 §4 图补模块 | 文档补全 |
| 4 | M#4 A.9 补模块 | 文档补全 |
| 5 | M#5 津贴归口措辞 | 文档措辞 |
| 6 | M#6 黑洞地址点名 | 文档补充 |

> 全部为文档侧或单常量微调，**不涉及销毁口径/总量铁律等已定稿核心逻辑**。
> 待猴哥对 M#1、M#2 拍板后，再统一执行第二轮改动（代码常量 / 文档六载体同步）。
