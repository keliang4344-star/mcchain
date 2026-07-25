# MC 公链路线图（贡献者视角）

> 面向外部工程师与社区。治理与决策见 [GOVERNANCE.md](./GOVERNANCE.md)，完整去中心化计划见 [docs/dao_roadmap.md](./docs/dao_roadmap.md)。

## 当前阶段：主网启动前（Pre-Mainnet）

- 6 个自定义模块：`depin` / `edgeai` / `phonenode` / `tokenomics` / `mcchain` 已完成；**`dex`（原生 AMM）在测**。
- 主网就绪清单见 [docs/LAUNCH_READINESS.md](./docs/LAUNCH_READINESS.md)。
- 链 ID：主网 `mcchain-mainnet-1`，本地 / 测试网 `mcchain-1`。

## 近期里程碑

| 里程碑 | 状态 | 说明 |
|--------|------|------|
| 模块开发完成 | ✓ | 5/6 模块完成，`dex` 在测 |
| 测试覆盖率 ≥70% | 进行中 | 关键模块 CI 门禁 |
| 安全审计 | 待启动 | 第三方审计 + 修复 |
| dex 上线 | 待完成 | AMM 完善与审计 |
| 多验证人主网 | 待启动 | ≥4 独立验证人 + TMKMS |

## 🆘 Help Wanted（求援模块）

> 以下领域急需外部顶级工程师认领。认领前在对应 Issue 评论，避免重复劳动。带 `help wanted` 标签的 Issue 即为此类；也可直接用 `.github/ISSUE_TEMPLATE/help_wanted.md` 提任务。

| 领域 | 模块 | 复杂度 | 共识关键 | 期望交付物 | 对接 |
|------|------|--------|----------|-----------|------|
| 原生 AMM 完成与审计 | `x/dex` | hard | 否 | swap / pool / liquidity 单测 + 审计清单 | @core |
| 前端 RPC 可配置化 | `web/` | medium | 否 | 钱包 / 浏览器可连任意节点 | @core |
| 监控看板 | `monitoring/` | medium | 否 | Prometheus + Grafana 面板 | @core |
| IBC 通道与中继 | `cosmos/` IBC | hard | 是（需 Core） | 通道开放 + relayer 演练 | @core |
| 覆盖率提升 | 全模块 | medium | 否 | 关键逻辑补充单测至 ≥70% | @core |
| 安全审计配合 | 全链 | hard | 是 | 配合第三方审计 + 修复 | @core |

## 长期方向

- DAO 治理去中心化（[docs/dao_roadmap.md](./docs/dao_roadmap.md)）
- 多验证人生态与跨链互通
- 移动全节点规模化（[BEGINNER_GUIDE.md](./BEGINNER_GUIDE.md)）

> 想认领某领域？提 Issue 或 PR，并在 [GOVERNANCE.md](./GOVERNANCE.md) 框架内推进。
