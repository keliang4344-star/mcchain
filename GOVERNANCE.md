# MC 公链治理与协作框架（GOVERNANCE）

> 本文定义 MC 公链在「开源 + 外部工程师共同开发」模式下的治理边界与决策流程。
> 参与贡献前请先读 [CONTRIBUTING.md](./CONTRIBUTING.md) 与本文。

## 1. 原则

- **开源可审计 · 参数写代码**：所有链上逻辑公开，经济与共识参数固化在代码与创世中，不靠链下约定。
- **核心可控、外围开放**：共识与安全相关代码由核心团队控 merge 权；工具 / 前端 / 文档对外充分开放。
- **争议靠 RFC，不靠嗓门**：任何非平凡改动先提案，核心团队裁定。

## 2. 角色

| 角色 | 权限 | 晋升 |
|------|------|------|
| Contributor（贡献者） | 提 Issue / PR | 持续高质量贡献 |
| Reviewer（评审者） | 评审开放区 PR、approve | 核心团队提名 |
| Core Team（核心团队） | 评审 + merge 所有 PR、裁定争议、发版 | 创始团队 |

核心团队名单由创始团队维护；如需细粒度路径审批，可在 `.github/CODEOWNERS` 配置。

## 3. 核心区 vs 开放区

### 核心区（Core / 受限）
改动必须 ≥2 名 Core 同意，且通过测试网验证与安全自查。

- `x/tokenomics` — 铸币总账、总量与分配（经济安全）
- `x/phonenode` — 硬件 attestation、心跳、离线 slash（共识安全）
- `x/mcchain` — 链级参数、系统配置
- `app/` 的 ante 装饰器、创世逻辑、共识参数
- `docs/TOKEN_ALLOCATION.md`、`docs/MAINNET_*` 与主网参数相关文档

### 开放区（Open）
外部工程师可深度参与，≥1 名 Core / Reviewer approve 即可 merge。

- `x/dex` — 原生 AMM（目前在测，欢迎完善与审计）
- `x/edgeai` — 边缘 AI 任务市场（非共识关键路径）
- `web/` — 钱包 / 浏览器前端（RPC 可配置化等）
- `tools/`、`monitoring/`、`mc-miner/`、`cosmjs-bundle/`
- `docs/`（除核心区标注的主网参数文档）、`BEGINNER_GUIDE.md`
- 测试与覆盖率提升（全模块目标 ≥70%）

## 4. 合并权限矩阵

| 改动类型 | 评审要求 | merge 权 |
|----------|----------|----------|
| 核心区 | ≥2 Core approve + 测试网跑通 + 安全自查 | 仅 Core |
| 开放区 | ≥1 Core / Reviewer approve | Core / Reviewer |
| 文档 / CI | ≥1 Core approve | Core |
| 依赖升级（go.mod） | 升级方负责 `go build` / `go test` 全绿后，≥1 Core approve | 仅 Core |

> 外部贡献者一律通过 PR 合入，**不直接拥有主仓库写权限**。

## 5. 共识层改动流程（强制）

1. 提 RFC（在 `docs/` 或 Issue 加 `rfc` 标签），说明动机、设计、影响。
2. Core 评审通过。
3. 实现 + 单测（覆盖率目标见上）。
4. **先在测试网（`mcchain-1` 或专用 testnet）验证出块与状态迁移**，再提 PR。
5. PR 经 ≥2 Core approve + CI 绿 + 安全自查 → merge 到 `main`。
6. 主网升级走 `docs/MAINNET_*` 发布流程。

## 6. 争议解决

- 一般分歧：PR / Issue 讨论，Reviewer 协调。
- 无法达成一致：升级为 RFC，Core Team 在 5 个工作日内裁定。
- 安全相关：按 [SECURITY.md](./SECURITY.md) 私下处理，不在公开渠道辩论。

## 7. 求援模块

急需外部顶级工程师的领域见 [ROADMAP.md](./ROADMAP.md) 的「Help Wanted」一节；按标签 `help wanted` 筛选 Issue 也可快速定位。
