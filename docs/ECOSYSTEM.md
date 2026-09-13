# 生态文档索引（ECOSYSTEM）

> 入口型文档：把项目所有公开资料按角色串联起来，让「第一次接触 MC 公链的人」
> 能在 5 分钟内找到自己需要的那一份。

## 1. 按角色

### 1.1 投资人 / 战略读者
- `../WHITEPAPER_CN.md`（白皮书 · 中文，一切内容的来源）
- `MobileChain白皮书_完整典藏版.pdf`（中文印刷版：A4 排版、含目录书签）
- `whitepaper.html`（中文在线版，与正典同源）
- `../WHITEPAPER.md`（Whitepaper · English，与中文版逐章逐节对应）
- `MobileChain-Whitepaper-Collector-Edition.pdf`（英文印刷版：A4 排版、含目录书签）
- `whitepaper_en.html`（英文在线版）
- `TOKEN_ALLOCATION.md`（代币分配）
- `tokenomics.md` / `PROTOCOL_PARAMS.md`（模块职责与参数口径）

### 1.2 节点运营 / 验证人
- `DEPLOYMENT_GUIDE.md`（部署指南）
- `VALIDATOR_RUNBOOK.md`（运维 → 处置）
- `NODE_CONFIG.md`（节点配置模板）
- `ALERTS.md`（Prometheus 告警规则）
- `PROTOCOL_PARAMS.md`（出块参数口径）
- `INCIDENT_PLAYBOOK.md`（故障处置手册）

### 1.3 dApp 开发者
- `DEVELOPER_GUIDE.md`（工程速查）
- `BACKEND_API.md`（REST + gRPC 接口）
- `sdk_event_contract.md`（订阅事件）
- `mobile_sdk_integration.md`（移动端 SDK 集成）
- `typescript-sdk/README.md`
- `IBC.md`（跨链接入）
- `INDEXER.md`（自建 explorer）
- `COSMWASM_INTEGRATION.md`（智能合约集成）

### 1.4 治理参与者
- `GOVERNANCE.md`（提提案 / 投票）
- `dao_roadmap.md`（DAO 路线图）
- `docs/proposals/`（提案模板与归档）

### 1.5 审计 / 监管
- `tokenomics.md`（各模块的链上状态与职责）
- `PROTOCOL_PARAMS.md`（参数口径总表）
- `GAS_AND_FEES.md`（费用与销毁口径）
- `ORACLE_FRAMEWORK.md`（预言机框架）

### 1.6 研究员
- `PROTOCOL_PARAMS.md` / `CAPACITY_PLAN.md` / `PERF_BASELINES.md`
- `module_mcchain.md`（治理移交模块）

## 2. 按生命周期

| 阶段 | 文档 |
|---|---|
| 部署期 | `DEPLOYMENT_GUIDE.md`、`NODE_CONFIG.md` |
| 集成期 | `INDEXER.md`、`IBC.md`、`DEVELOPER_GUIDE.md` |
| 运营期 | `VALIDATOR_RUNBOOK.md`、`ALERTS.md`、`INCIDENT_PLAYBOOK.md` |
| 治理期 | `GOVERNANCE.md`、`dao_roadmap.md`、`docs/proposals/` |
| 升级期 | `VALIDATOR_RUNBOOK.md` §3「升级流程」 |

## 3. 不一致自查表

如果发现两个文档口径不一致，**以代码为准**，再按优先级更新文档：

| 优先级 | 来源 | 说明 |
|---|---|---|
| 0 | 编译产物 `mcchaind` | 真正在跑的代码 |
| 1 | `scripts/make_genesis.py` | 创世口径校准脚本 |
| 2 | `docs/PROTOCOL_PARAMS.md` | 参数口径总览 |
| 3 | `WHITEPAPER_CN.md`（`WHITEPAPER.md` 为其英译） | 商业承诺 |
| 4 | 其他运营 / 集成文档 | 引用 (2) 与 (3) |

## 4. 怎么贡献文档

- 文档变更需要 PR + 一位 reviewer。
- 涉及参数：同步更新 `PROTOCOL_PARAMS.md` 与 `make_genesis.py` 的注释。
- 涉及白皮书承诺：先发提案在治理通道走流程。
- 涉及安全相关（slash / oracle / 治理移交）：必须有一位核心 reviewer + 安全工程师双签。

## 5. 文档版本与变更记录

每次 commit 修改本目录时，请更新对应文档头的 `> 最近一次更新` 字段
（首次落地时统一加上）。
