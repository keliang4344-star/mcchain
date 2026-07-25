# Changelog

All notable changes to MobileChain (MC) will be documented in this file.

---

## [v4.0] — 2026-07-18

### Added
- `x/referral` 推荐裂变激励模块
- Verifier 抽检机制（`x/phonenode` 验证人随机抽检）
- 销毁机制：50% burn / 30% treasury / 20% LP 分配模型
- DEX 交易手续费分配（0.3% swap fee 自动分账）
- 25 个集成测试覆盖核心模块
- Web Dashboard 四面板（`web/index.html`）：概览 / 链参数 / 验证人 / 交易
- CosmJS 类型定义 (`cosmjs-bundle/`)

### Changed
- `x/tokenomics`：固化销毁入口与 burn 事件
- `x/dex`：swap 手续费自动按比例分账
- `x/phonenode`：新增 Verifier 角色与抽检逻辑
- 白皮书更新至 v4.0

---

## [v3] — 2026-06

### Added
- 白皮书 v3.0 (`docs/WHITEPAPER.md` 重写)
- 商业计划书
- 主网 Runbook (`docs/MAINNET_RUNBOOK.md`)
- 主网部署方案 (`docs/MAINNET_DEPLOY_PLAN.md`)
- Gas 与费用策略文档 (`docs/GAS_AND_FEES.md`)
- DAO 路线图 (`docs/dao_roadmap.md`)
- 审计清单 (`docs/audit_checklist.md`)
- 模块白皮书 (`docs/MODULE_WHITEPAPER.md`)

### Changed
- genesis 生成器脚本完善 (`scripts/make_genesis.py`)
- 部署脚本标准化 (`deploy/init.sh`, `deploy/start.sh`)

---

## [v2] — 2026-04

### Added
- `x/dex` 原生 AMM 去中心化交易所（常量积 x×y=k）
- IBC 跨链通信集成 (ibc-go v7.1.0)
- `mc-miner/` Android 挖矿 App（WebView + CosmJS）
- `cosmjs-bundle/` 前端 CosmJS UMD Bundle
- 后端 API 文档 (`docs/BACKEND_API.md`)
- 协作指南 (`docs/COLLABORATION.md`)

---

## [v1] — 2026-02

### Added
- `x/tokenomics` 代币发行与分配总账模块（唯一 Minter，总量 10 亿 MC 固化）
- `x/depin` 设备贡献激励引擎
- `x/phonenode` 移动全节点管理模块
- `x/edgeai` 边缘 AI 任务市场模块
- `x/mcchain` 链级参数管理模块
- 模块单元测试（depin 14 / phonenode 7 / tokenomics 7 / edgeai 17 / mcchain 5）
- CI 流水线 (`.github/workflows/ci.yml`)

---

## [v0] — 2026-01

### Added
- 项目初始化，基于 Cosmos SDK 脚手架
- `mcchaind` 二进制构建 (`cmd/mcchaind`)
- 基础链配置与创世生成
- 许可证 (Apache 2.0)
