# MobileChain B6 批次 · 主网就绪（DAO 路线 / 治理模块 / 审计清单 / 主网 genesis·配置·部署）· 增量 PRD

**文档类型**：增量 PRD（基于现有 `mcchain` 代码，仅描述变更，不含实现代码；docker-compose/runbook 为运维文本可落盘）
**批次**：B6（主网就绪）— 归属路线图收尾与主网启动
**作者**：许清楚（Xu），Product Manager
**语言**：简体中文
**验收总原则**：沙箱无法运行 `go`/`protoc`，本 PRD 不写代码、不在沙箱执行脚本；验收一律以用户本机 `ignite chain build` + `mcchaind init`/启链 + genesis 与 B1 cap 一致性校验 + 审计清单评审为准。

**产品目标（一句话）**：将 B1–B5 的链上能力推进到主网就绪——明确 DAO 治理演进与治理模块、给出第三方审计清单、固化生产 genesis/配置、提供 docker-compose 部署与启链 runbook。

## 0. 背景现状（已侦察，采信）
- B1–B5 完成经济模型、安全、edgeai、phonenode 轻量同步、生态接口。
- B2 已有最小链上治理（gov）；本批次升级为 DAO 路线（社区池支配、提案门槛、timelock 可选、未来 cap 治理化路径）。
- 缺少：生产 genesis/配置、部署脚本、审计清单、DAO 演进文档。

## 1. 增量范围说明（基于现有 mcchain，仅列变更）
- **DAO 路线与治理模块**：在 B2 gov 基础上明确 DAO 演进——社区池由治理支配（B1 社区池为独立 community 模块账户）、提案/voting 门槛、timelock（可选新模块）、未来 `total_supply_cap` 治理化路径。本批次落地治理参数与社区池支配。
- **第三方审计清单**：经济模型、模块/合约、安全（attestation/slashing/anti-cheat）、渗透、文档——给出审计项与通过标准。
- **主网 genesis/配置**：生产 genesis 固化 B1–B5 全部参数/账户/拨付；config.toml/app.toml 生产默认值；种子节点列表。
- **部署脚本与 runbook**：docker-compose（验证者/全节点/快照）、启链 runbook（init/collect-gentxs/start/upgrade）。
- **不变**：B1–B5 业务逻辑；docker-compose/runbook 为运维文本，可落盘（非 go 代码）。

## 2. 需求池（P0–P2）

### R1 · 治理模块与 DAO 路线（P0 / Must have）
- **需求描述**：明确治理参数、社区池治理支配、提案/voting 门槛、timelock；DAO 演进路线文档化（含未来 cap 治理化路径）。
- **验收标准**：
  - gov 提案可变更治理参数、社区池由治理支配；DAO 路线文档化且可追溯。
- **关键约束**：本批次 `total_supply_cap` 仍锁定（防超发），仅规划未来治理化路径，不立即开放。

### R2 · 主网 genesis 与配置（P0 / Must have）
- **需求描述**：生产 genesis 固化 B1 分配 + B2 安全参数 + B3/B4 参数 + 多签/锁仓账户；config 生产默认值。
- **验收标准**：
  - genesis 经 `mcchaind init` 验证可启动，与 B1 cap 一致；参数完整。
- **关键约束**：沿用 B1 决定（config.yml 不额外加三大池币、拨付程序化）。

### R3 · 部署脚本与 runbook（P0 / Must have）
- **需求描述**：docker-compose（验证者/全节点/快照）+ 启链 runbook（init/collect-gentxs/start/upgrade）。
- **验收标准**：
  - 脚本/文档齐全，用户本机可照 runbook 启链（沙箱不跑，用户本机验证）。
- **关键约束**：运维文本，可落盘。

### R4 · 第三方审计清单（P1 / Should have）
- **需求描述**：经济/安全/代码/渗透审计项与通过标准。
- **验收标准**：
  - 清单可被审计方使用，含交付物与通过门槛。
- **关键约束**：必由第三方完成，PM 只列项与标准。

### R5 · 监控/运维基线（P2 / Nice to have）
- **需求描述**：基础监控指标、升级治理流程文档。
- **验收标准**：
  - 文档完整。

## 3. 关键设计建议（给架构师）
1. **治理**：复用 cosmos `gov v1` + 社区池（B1 独立 community 模块账户或 gov 社区池）；timelock 可选（新模块或合约）；DAO 分阶段（信号→参数→资金→cap 治理化）。
2. **genesis**：基于测试网 genesis 模板，固化 B1 分配 + B2 安全 + B3/B4 参数；多签/锁仓账户按 B1 设计（团队 vesting 1y+3y）。
3. **部署**：docker-compose 三件套（validator/fullnode/snapshot）；runbook 覆盖 collect-gentxs、start、upgrade（plan/height）。
4. **审计**：四维度清单（经济/安全/代码/渗透）+ 文档评审；明确「必由第三方」。

## 4. 文件清单（新增/修改）
- 修改/新增：`x/`（治理参数若需新模块如 `x/dao` 或 timelock，可选）、`config/`（生产 genesis、`config.toml`/`app.toml` 默认）、`deploy/docker-compose.yml`。
- 新增文档：`docs/dao_roadmap.md`、`docs/audit_checklist.md`、`docs/runbook_mainnet.md`、`scripts/`（init/start 脚本）。
- 不变：B1–B5 业务逻辑。

## 5. 验收总原则
- 沙箱无法运行 `go`/`protoc`；docker-compose/runbook 为文本可落盘，但不在沙箱执行。
- 验收一律以用户本机 `ignite chain build` + `mcchaind init`/启链 + genesis 与 B1 cap 一致性校验 + 审计清单评审为准。
