# MobileChain DAO 演进路线（B6-R1）

> 本文档规划治理从最小链上 gov 演进到 DAO 的分阶段路径。`total_supply_cap` 主网期锁定，仅规划未来治理化，不立即开放。

## 阶段 0 · 现状（已具备）
- Cosmos SDK `gov v1` 已集成，支持文本/参数变更/社区池支出提案。
- 社区池：B1 独立 community 模块账户，余额由 genesis 分配。

## 阶段 1 · 信号治理（上线即可）
- 社区通过 gov 文本提案表达方向（非约束性）。
- 工具：`mcchaind tx gov submit-proposal --title ... --type text`。

## 阶段 2 · 参数治理（P0，本批次落地参数）
- 以下参数开放 gov 变更（需提案 + 投票通过）：
  - `depin`：RewardRate 各任务类型、MaxRewardPerTask、ContributionThreshold
  - `phonenode`：OfflineGraceBlocks、OfflineSlashBps、AttestationValidity
  - `tokenomics`：各池分配比例（team/community/ecosystem）
- 治理门槛默认值建议（待拍板）：
  - 最低提案押金：`100000000000 umc`（=100k MC）
  - 投票期：`2 weeks`
  - 法定投票率（quorum）：`33.4%`
  - 通过阈值（yes）：`50%`（重要参数升级建议 `66.7%`）

## 阶段 3 · 资金治理（社区池支配）
- 社区池支出提案（Community Pool Spend）用于生态资助、漏洞赏金、审计费。
- 金库多签（T1）与社区池并行：团队金库按 vesting 释放，社区池由治理支配。

## 阶段 4 · cap 治理化（未来，谨慎）
- 仅当生态成熟、经长期信号+参数治理验证后，才提案将 `total_supply_cap` 由代码锁定改为 gov 可调整。
- 安全护栏：任何 cap 上调提案需超级多数（≥90%）+ 长投票期（≥4 weeks）+ timelock（≥2 weeks）后方可执行，防超发。

## timelock（可选）
- 新增 `x/timelock` 或合约层，对高影响提案加执行延迟，留出社区退出/审计窗口。

## 追溯与文档
- 所有治理参数变更记录于链上提案；本路线文档化并随版本更新，确保可追溯。
