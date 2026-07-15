# MobileChain B1 批次 · 经济模型链上基础 · 增量 PRD

**文档类型**：增量 PRD（基于现有 `mcchain` 代码，仅描述变更，不含实现代码）
**批次**：B1（经济模型链上基础）— 归属路线图「阶段四 经济模型与安全」的链上部分
**作者**：许清楚（Xu），Product Manager
**语言**：简体中文
**验收总原则**：沙箱无法运行 `go`/`protoc`，本 PRD 不写代码、不执行命令；验收一律以用户本机 `ignite chain build` + `go test` + 链上观测（`mcchaind q ...`）为准。

---

## 0. 背景与现状（已侦察，采信）

- 总量恒定 10 亿 MC 目前**仅存在于注释/口头约定**，链上未固化：无总量 cap、无已 mint 累计、无分配占比/释放曲线、无透明查询。
- 已落地：KVStore 迁移、P0（质押参数 + 出块 4s）、P1（DePIN 池拨付）、P2（depin↔phonenode 关联 + 移动端 SDK 文档）。
- 代码事实（已 Read 验证）：
  - `x/depin/types/params.go`：`InitialPool=1e14 umc`（= 1e8 MC）、`RewardDenom="umc"`；注释写明「`1e14 umc == 1e8 MC (about 10% of the 1B total supply)`」—— 10 亿总量仅是注释。
  - `x/depin/genesis.go`：`InitGenesis` 内 `k.MintCoins` 一次性铸入 depin 模块账户；注释声明「`the chain never mints again at runtime, keeping total supply fixed at 1B MC`」—— cap 仅靠约定保证，无链上强制。
  - `app/app.go`：`maccPerms` 中 `depin` 持有 `{Minter, Burner, Staking}`；当前**无 `tokenomics` 模块**（已 Glob 确认 `x/tokenomics` 不存在）；genesis 顺序 depin 在 mcchain 之后、phonenode 之前；`BondDenom` 强制为 `umc`，`1 MC = 1e6 umc`。
  - `x/depin/keeper/keeper.go`：`MintCoins` 调用 `bankKeeper.MintCoins(ctx, types.ModuleName, amt)` 铸入 depin 模块账户。

---

## 1. 产品目标（一句话）

把「10 亿总量 + 团队/社区/生态分配 + 释放曲线」从口头约定固化为链上可查、可验证、不可超发的经济模型。

---

## 2. 增量范围说明（基于现有 mcchain，仅列变更）

- **新增模块**：`x/tokenomics`（承载总量 cap、各池余额/占比跟踪、已 mint 累计、释放曲线元数据、查询端点）。当前 `x/tokenomics` 不存在。
- **app 装配变更**：`app/app.go` — 注册 `tokenomics` 模块：`ModuleBasics`、`maccPerms`（新增 `tokenomics` 模块账户，授予 `Minter` 权限）、store key、`keys`、keeper 装配、`mm` 模块列表、Begin/End/InitGenesis 顺序、`initParamsKeeper` 的 subspace。
- **genesis 调整**：三大池拨付由 `x/tokenomics` 的 `InitGenesis` **程序化完成**，`config.yml` 不额外新增三大池账户币；InitialPool 拨付计入生态池并受 cap 约束；depin 的 genesis 自铸逻辑需与 tokenomics 对齐（见 §4）。
- **查询 CLI**：新增 `mcchaind q tokenomics <subcommand>`（总量上限、已发行/已 mint、各池余额与占比、释放进度）。
- **文档**：补充经济模型说明（总量、分配占比、释放曲线、查询方式），供社区与 SDK 引用。
- **不变**：P0 质押参数、P1 depin 池拨付的业务语义、P2 关联与 SDK 文档；B2 安全红线（硬件 attestation / Slashing / 治理）不在本批次。

---

## 3. 需求池（按 B1 拆 3 条）

### R1 · 总量恒定 10 亿 MC 的链上固化

- **需求描述**：新增链上机制，使运行期任何 mint（含 genesis InitialPool 拨付与其他池拨付）都计入总量上限，且总量上限不可被突破。提供可查询的「总量上限」与「已发行/已 mint 累计」，并固化 `denom=umc`、`1 MC = 1e6 umc`。
- **验收标准**：
  - 链上可查：执行 `mcchaind q tokenomics supply` 返回 `total_supply_cap = 1e15 umc`（= 1e9 MC），`minted_supply` 初始值等于 genesis 实际拨付总额。
  - 不超发：任意导致累计 mint 超过 `total_supply_cap` 的拨付（含 genesis 校验）必须 panic/失败；`minted_supply <= total_supply_cap` 恒成立（含 Invariant 校验）。
  - 固化：`total_supply_cap`、`minted_supply`、`denom` 持久化于 `x/tokenomics` state，节点重启后一致。
- **优先级**：**P0（Must have）**
- **关键约束**：
  - cap 由 `x/tokenomics` param 持有，单一真相源；depin 的 InitialPool 自铸必须被 tokenomics 记账（计入生态池且受 cap 约束），**不得另起一套独立的、不受 cap 约束的 mint**。
  - 不可修改既有 depin 的业务语义（仍是 1e8 MC 生态起点），仅补「记账 + cap 约束」。

### R2 · 分配占比 + 释放曲线（透明、可查）

- **需求描述**：链上固化团队/社区/生态三大池的分配占比与释放曲线，生态池以已拨付 InitialPool（1e8 MC）为起点。团队池（<20%）采用「1 年 cliff + 3 年线性 = 总 4 年」的长锁仓；社区、生态按既定计划拨付（暂按模块账户持有，不单独 vesting）。各池余额与占比可链上查询。
- **验收标准**：
  - 链上固化三大池分配参数（推荐默认：团队 15% / 社区 35% / 生态 50%），`mcchaind q tokenomics allocations` 返回各池占比与余额。
  - 团队池释放：genesis 用 `x/auth/vesting` 连续锁仓账户承载「1 年 cliff + 3 年线性 = 总 4 年」；`mcchaind q tokenomics release` 可观测「已释放 / 待释放 / 释放进度%」与释放曲线起止时间。**仅团队池使用 vesting**；社区池、生态池暂按模块账户持有、不单独 vesting（后续可扩展）。
  - 占比之和 = 100%，且各池初始拨付（含生态的 InitialPool 起点）之和 ≤ `total_supply_cap`。
  - 释放曲线「文档化 + 链上元数据」双重表达：链上记录起止时间 / cliff / 线性参数，社区文档给出可读曲线。
- **优先级**：**P1（Should have）**
- **关键约束**：
  - 占比采用推荐默认（团队 15 / 社区 35 / 生态 50），但**以用户拍板为准**（见 §5）。
  - 锁仓年限、cliff 等以用户拍板为准；团队占比须 <20%（路线图书面硬约束）。
  - 生态池起点 = 已拨付 InitialPool 1e8 MC，须与 R1 的 cap 记账一致。

### R3 · 透明查询（总量 / 分配 / 释放可链上观测）

- **需求描述**：提供统一查询面，使任意验证者/用户/外部工具在链上可观测总量上限、已发行、各池分配与占比、已释放进度，无需信任文档。
- **验收标准**：
  - `mcchaind q tokenomics`（及子命令 `supply` / `allocations` / `release`）覆盖 R1、R2 全部可观测项，返回结构化字段。
  - 提供 gRPC 查询端点（供区块浏览器 / SDK 调用），字段与 CLI 一致。
  - 可配合 Invariant 校验：在任何高度查询到的 `minted_supply <= total_supply_cap`（总量不超发）；分配一致性采用**会计口径**不变量「分配记账和 == 已发行（`minted_supply`）」—— 因运行期拨付后各池实时余额和会 < `minted_supply`，故以分配记账（ledger）之和而非实时余额之和做一致性校验。
- **优先级**：**P1（Should have**；但 R1/R2 落地后，本项随查询端点一并交付，视为 P0 体验闭环）
- **关键约束**：
  - 仅只读查询，不改变状态；不引入额外 mint/burn 逻辑。
  - 命名与现有 `mcchaind q depin` / `mcchaind q mcchain` 风格一致。

---

## 4. 关键设计决策建议（给架构师）

以下为 PM 视角的推荐，最终由架构师在设计阶段定稿。

1. **新增 `x/tokenomics` 模块 vs 扩展 `depin` params**
   - **推荐：新增 `x/tokenomics` 模块**。理由：经济模型（cap/分配/释放）是跨池、跨账户的横切关注点，独立模块职责清晰、便于查询与 Invariant 校验；扩展 `depin` params 会把经济模型耦合进 DePIN 业务语义，且 `depin` 已承载拨付，不适合再塞总量治理。tokenomics 作为「发行与分配总账」，depin 作为「生态池的使用方」更合理。
   - 衔接：depin 的 `InitialPool` 仍作为生态池起点参数，但拨付动作改为由 tokenomics 记账/拨付（见第 3 点），或在 tokenomics `InitGenesis` 中显式登记该笔 mint。

2. **genesis 多签 / 锁仓账户方案（拨付由 `x/tokenomics` 的 `InitGenesis` 程序化完成，`config.yml` 不额外加三大池币）**
   - 三大池拨付统一在 `x/tokenomics` 的 `InitGenesis` 内程序化完成（读取 params 的分配占比，一次性 `MintCoins` 后转入对应账户），`config.yml` **不额外新增三大池账户币**。
   - 团队池：genesis 创建 `x/auth/vesting` 连续锁仓账户（ContinuousVesting），由团队多签（或治理多签）持有，**1 年 cliff + 3 年线性 = 总 4 年**。**仅团队池使用 vesting**。
   - 社区池：拨付至社区模块账户（如独立的 `community` 模块账户，或复用 gov community pool）；若复用 gov 社区池，tokenomics 仅做只读跟踪。**暂按模块账户持有、不单独 vesting（后续可扩展）**。
   - 生态池：含 InitialPool 1e8 MC 起点 + 其余生态额度；可由 `tokenomics` 模块账户或独立 `ecosystem` 模块账户持有，按需拨付给 depin 等模块。**暂按模块账户持有、不单独 vesting（后续可扩展）**。
   - 具体地址、多签阈值、cliff 由架构师在设计中定，PRD 不锁死。

3. **cap 与 InitialPool 的衔接（核心约束）**
   - **单一发行入口（推荐）**：由 `x/tokenomics` 在 `InitGenesis` 内统一完成全部拨付（团队/社区/生态），并一次性 `MintCoins` 铸入，受 `total_supply_cap` 约束；生态池再将其中的 InitialPool 切片转给 depin 模块账户（depin 不再自铸）。这样「只有 tokenomics 能 mint，且受 cap 约束」，完全满足「不要另起一套冲突的 mint」。
   - **兼容方案（若避免改动 depin 自铸）**：depin 仍自铸 InitialPool，但 tokenomics `InitGenesis` 必须**登记**该笔 mint（读取 depin params 的 `InitialPool`，将等值的 `minted_supply` 计入生态池），并对剩余池做受 cap 约束的拨付；此时须调整 genesis 顺序，确保 tokenomics 的 cap 记账覆盖 depin 自铸（建议 tokenomics `InitGenesis` 在 depin 之前跑、预留 cap，或 depin 自铸后 tokenomics 校验累计不超 cap）。
   - 两种方案以「累计 mint 永不超 cap、InitialPool 计入生态」为硬验收。
   - **主理人裁决（采用架构师默认口径）**：拨付统一由 `x/tokenomics` 的 `InitGenesis` 程序化完成，`config.yml` 不额外新增三大池账户币 —— 即采用上方「单一发行入口」方案。

4. **查询端点设计**
   - gRPC Query service `Query` 于 `x/tokenomics`：`Supply`（cap + minted）、`Allocations`（各池占比 + 余额）、`Release`（各池释放进度 + 曲线元数据）。
   - CLI：`mcchaind q tokenomics supply|allocations|release`。
   - 字段均以 `umc` 整数表达总量/minted/余额；占比以整数百分比或 basis point 表达；释放进度可由 vesting 账户 start/duration 与当前 block time 推算，**建议缓存于 tokenomics state**（避免每次查询跨模块读 vesting）。

5. **Invariant 与治理衔接**
   - 新增 `x/tokenomics` Invariant：`minted_supply <= total_supply_cap`（总量不超发）；分配一致性采用会计口径「分配记账和 == 已发行（`minted_supply`）」（可由 crisis 模块在每轮校验触发）。注意运行期拨付后各池实时余额和会 < `minted_supply`，故以分配记账之和校验。
   - `total_supply_cap` 为常量参数，本批次**不**纳入治理可改（防超发由不可变 cap 保证）；未来若需调整，留待 B2 治理框架。

---

## 5. 待确认问题清单（需用户拍板，标注推荐默认）

| # | 问题 | 推荐默认 | 影响 |
|---|------|----------|------|
| Q1 | 是否新增 `x/tokenomics` 模块（vs 扩展 depin params）？ | **是，新增 `x/tokenomics`** | 架构与查询端点 |
| Q2 | 三大池分配占比？ | **团队 15% / 社区 35% / 生态 50%** | R2 验收基准 |
| Q3 | 团队池锁仓年限与 cliff？ | **1 年 cliff + 3 年线性 = 总 4 年**（已裁决，仅团队池用 vesting） | R2 释放曲线 |
| Q4 | 生态池是否包含已拨付 InitialPool 1e8 MC 作为起点？ | **是**，计入生态且受 cap 约束 | R1/R2 衔接 |
| Q5 | 社区池落账方式：gov community pool vs 独立社区模块账户？ | **优先独立 `community` 模块账户**（避免与既有 distribution 语义耦合，便于 tokenomics 跟踪）；若复用 gov 社区池须只读跟踪 | R2 账户方案 |
| Q6 | 团队池托管：多签阈值 / 是否由治理多签持有？ | 由团队多签持有，阈值与地址由架构师定稿 | R2 账户方案 |
| Q7 | InitialPool 拨付方式：tokenomics 统一 mint（depin 不自铸）vs depin 自铸 + tokenomics 登记？ | **推荐 tokenomics 统一 mint**（单一发行入口，最契合「不冲突的 mint」）；兼容方案允许 depin 自铸但必须登记 | R1 核心约束 |
| Q8 | `total_supply_cap` 是否纳入治理可改？ | **否**，本批次锁定为常量（防超发） | 安全属性 |
| Q9 | 释放进度查询是否缓存于 tokenomics state，还是实时读 vesting 账户？ | **缓存于 tokenomics state**（查询稳定、跨模块少） | R3 查询设计 |

---

**给架构师的下一步（已闭环）**：Q1（新增 `x/tokenomics`）、Q2（团队 15% / 社区 35% / 生态 50%）、Q7（tokenomics 统一 mint、`config.yml` 不额外加三大池币）已按主理人裁决采用架构师默认口径；工程师已基于 `docs/system_design_b1_tokenomics.md` 开工。本 PRD 后续随 B2 安全红线再扩展治理可改 cap 等议题。
