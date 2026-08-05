# Changelog

All notable changes to MobileChain (MC) will be documented in this file.

---

## [v4.2] — 2026-08-05

### Changed — 销毁政策定稿（《优化定稿版》口径）

通缩只向「协议使用费」与「作恶罚没」两处取材，**参与者的劳动所得一律不销毁**。

- **保留销毁**：gas 手续费 7%（`GasBurnRatioBps = 700`）、DEX swap 手续费的 50%（`FeeBurnBps = 5000`，即成交额 0.15%）。
- **撤销销毁（改为参与者 100% 足额到账）**：
  - `x/depin` 设备任务赏金 5% 销毁 → 撤销，节点全额到手（删除 `DePINBurnRatioBps`）；
  - `x/referral` 推荐奖励 1% 销毁 → 撤销，推荐人全额到手；
  - `x/edgeai` 结算 5% 销毁 → 撤销，分账由 80/15/5 改为 **85/15**（`EdgeAISubmitterRatioBps = 8500`，删除 `EdgeAIBurnRatioBps`）。
- **罚没分流**：`x/phonenode` 自定义 slash 由「100% 回流安全池」改为 **40% 销毁 / 60% 回流质押安全池**（`SlashBurnRatioBps = 4000`、`SlashSecurityRatioBps = 6000`）。销毁部分由 bonded pool 转入黑洞地址，回流部分转入 `staking_security`，两腿之和恒等于被罚总额。
- **企业结算费**：1.50% 拆为 40% 给节点（`EnterpriseFeeNodeRatioBps`）/ 60% 进国库，**此路径不销毁、不铸币**。
- **EdgeAI 作弊回收**：仲裁判定作弊时，托管的提交者份额转入质押安全池补贴诚实节点，不销毁。

### Security
- `x/depin`、`x/referral`、`x/edgeai` 的 `BankKeeper` 接口移除 `BurnCoins`，从类型层面物理禁止被撤销的销毁逻辑复用。
- 新增 `x/phonenode/keeper/slash_split_test.go`：验证 40/60 拆分金额守恒、黑洞与安全池双腿到账、Jail 与 distribution 钩子正常。
- `TestAppStateDeterminism`（含全量不变量）通过，自定义 slash 不破坏 staking 模块账户不变量。

### Docs
- `WHITEPAPER.md` / `docs/WHITEPAPER.md`、`WHITEPAPER_CN.md` / `mc-miner/WHITEPAPER_v3.md`、`docs/GAS_AND_FEES.md` 全量对齐上述口径，清理过期常量引用。

---

## [v4.2.1] — 2026-08-05

### Docs — 白皮书 ↔ 代码 双向一致性审计二轮修整（M#1–M#6 全载体同步）

- **timelock 收敛**：白皮书原将国库提现 timelock 写为硬约束（Delivered），代码仅注释、无实现；现统一为「治理多签 Live，timelock 规划中（v1 未强制执行）」，覆盖 WHITEPAPER.md、docs/WHITEPAPER.md、WHITEPAPER_CN.md、mc-miner/WHITEPAPER_v3.md、docs/whitepaper.html。
- **模块数统一**：「五大 / 六原生模块」→「八大原生模块」；架构表与路线图清单补全 liquidstaking / referral / 治理移交（mcchain）；附录 A.9 / A.7 代码地图补 liquidstaking、wasm、ibc。
- **DEX 锁仓时长自洽**：「7 天」→「100,800 区块（约 4.7 天，按 ~4 秒出块）」，与 `LpLockBlocksDefault = 100800` 及 §A.1 ~4s 一致；同步修正 `x/dex` 锁仓注释的 6s/7天 旧表述为 ~4s/~4.7天（不改任何常量值）。
- **节点日津贴归口**：明确由 `x/phonenode` 模块发放、取自设备池额度（30 MC / 认证节点 / 日）。
- **点名黑洞地址**：销毁章节补「确定性派生、无私钥的 black_hole 模块账户」说明。
- **路线图清单去重**：CN 与 HTML 版原将 liquidstaking / CosmWasm / IBC / 治理移交误列「规划中」，现移至「已完成」；规划中仅保留 EdgeAI 算力市场规模化。

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
