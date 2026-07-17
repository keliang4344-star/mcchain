# 附录 A　完整链上参数表（可逐条对照源码审计）

本附录汇总白皮书正文引用的全部关键参数，并标注其源码位置。审计者可据此逐条核对"白皮书数字 = 代码数字"。**凡与代码不符，以代码为准。**

## A.1 链级基础参数

| 参数 | 值 | 源码位置 |
|---|---|---|
| 共识引擎 | CometBFT v0.37.6 | `go.mod` |
| 应用框架 | Cosmos SDK v0.47.14 | `go.mod` |
| 语言 | Go 1.21 | `go.mod` |
| 主网链标识 | `mcchain-mainnet-1` | 创世配置 |
| 测试网链标识 | `mcchain-testnet-1` | 创世配置 |
| 账户地址前缀 | `mc` | `app/app.go`（`AccountAddressPrefix`） |
| 验证人地址前缀 | `mcvaloper` | `app/app.go` |
| 共识地址前缀 | `mcvalcons` | `app/app.go` |
| 主币最小单位 | `umc` | `x/tokenomics/types/keys.go` |
| 精度 | 6（1 MC = 1,000,000 umc） | 链配置 |
| SLIP-44 CoinType | 118 | `docs/keplr-chain-registry.json` |
| 增发通胀 | 0（`x/mint` 强制为零） | 创世/mint 配置 |

## A.2 代币经济（tokenomics）

| 参数 | 值（umc） | 值（MC） | 源码常量 |
|---|---|---|---|
| 总量上限 | 1,000,000,000,000,000（1e15） | 10 亿 | `TotalSupplyCap` |
| 设备激励 55% | 550,000,000,000,000 | 5.5 亿 | `DeviceIncentivePercentBps=5500` |
| 质押安全 15% | 150,000,000,000,000 | 1.5 亿 | `StakingSecurityPercentBps=1500` |
| 团队 12% | 120,000,000,000,000 | 1.2 亿 | `TeamPercentBps=1200` |
| 基金会 13% | 130,000,000,000,000 | 1.3 亿 | `FoundationPercentBps=1300` |
| 早期开发 5% | 50,000,000,000,000 | 0.5 亿 | `EarlyDevPercentBps=500` |
| 设备池注入 depin | 550,000,000,000,000 | 5.5 亿 | `DepinInitialPoolSlice` |
| 团队多签阈值 | 3-of-5 | — | `TeamMultisigThreshold=3` |
| 团队释放 | 1 年 cliff + 3 年线性 | — | `x/tokenomics/keeper/genesis.go` |
| 分配笔数校验 | 恰好 5，基点之和恰好 10000 | — | `x/tokenomics/types/genesis.go`（`Validate`） |

## A.3 设备激励（depin）

| 参数 | 值 | 源码常量 |
|---|---|---|
| 初始金库 | 550,000,000,000,000 umc（5.5 亿 MC，55%） | `DefaultInitialPool` |
| 奖励 denom | `umc` | `DefaultRewardDenom` |
| 铸币权限 | 无（只发放，不铸造） | maccPerms（无 Minter） |
| 与 tokenomics 一致性 | `DefaultInitialPool == DepinInitialPoolSlice`（创世断言） | `x/tokenomics/keeper/genesis.go` |

## A.4 手机节点安全（phonenode）

| 参数 | 值 | 含义 | 源码 |
|---|---|---|---|
| `AttestationRequired` | true | 必须认证才能参与 | `x/phonenode/types/params.go` |
| `AttestationValidity` | 2,592,000 秒（30 天） | 认证有效期 | 同上 |
| `SybilDeviceBinding` | true | 女巫设备绑定 | 同上 |
| `OfflineGraceBlocks` | 100 区块 | 离线宽限 | 同上 |
| `OfflineSlashBps` | 500（5%） | 离线罚没 | 同上 |
| `ContribSlashBps` | 1000（10%） | 作弊贡献罚没 | 同上 |
| `AttestSlashBps` | 2000（20%） | 伪造认证罚没 | 同上 |
| `SlashCooldownBlocks` | 43200 区块（约 12 小时 @4s） | 罚没后再认证冷却 | `DefaultSlashCooldownBlocks` |
| 罚没基点硬约束 | 所有 slash bps ≤ 10000 | 防罚没超本金 | 参数校验 |

## A.5 边缘 AI（edgeai）

| 参数 | 值 | 含义 | 源码 |
|---|---|---|---|
| `MaxTaskReward` | 1,000,000,000 umc（1000 MC） | 单任务赏金上限 | `x/edgeai/types/params.go` |
| `DisputePeriodBlocks` | 100 区块 | 争议窗口 | 同上 |
| `AntiCheatThresholdBps` | 5000（50%） | 反作弊阈值 | 同上 |
| `Arbitrator` | 团队多签地址（部署设定，可治理化） | 争议裁决者 | 同上 |
| 付费模型 | 需求方托管（escrow）+ 乐观结算 | — | `x/edgeai` keeper |

## A.6 共识安全门槛（ante）

| 参数 | 值 | 含义 | 源码 |
|---|---|---|---|
| `MinSelfDelegationLowerBound` | 100,000,000,000 umc（10 万 MC） | 验证人最低自抵押 | `app/ante.go` |
| 强制范围 | 全链统一，任何建/改验证人交易 | — | `MinSelfDelegationDecorator` |

---

# 附录 B　模块与代码结构清单

| 模块 / 组件 | 路径 | 职责 |
|---|---|---|
| mcchain | `x/mcchain` | 系统锚点模块 |
| tokenomics | `x/tokenomics` | 总量上限、五池分配、团队 vesting、分配查询 |
| depin | `x/depin` | 设备激励金库与按任务发放 |
| phonenode | `x/phonenode` | 设备认证、女巫绑定、分级罚没 |
| edgeai | `x/edgeai` | AI 任务、托管付费、乐观结算、争议仲裁 |
| ante 装饰器 | `app/ante.go` | 验证人最低自抵押强制 |
| 应用装配 | `app/app.go` | 模块账户权限（maccPerms）、地址前缀、模块装配 |
| 链下预言机 | `internal/oraclesvc` | 受控地把链下事实提交上链 |
| 事件订阅器 | `cmd/event-subscriber` | 链上事件指标导出与持久化 |
| Web 仪表盘 | `web/` | 钱包 + 区块浏览器 + 交易助手（协作方维护） |

主要文档（`docs/`）：模块白皮书 `MODULE_WHITEPAPER.md`、代币分配设计 `TOKEN_ALLOCATION.md`、开发文档 `DEVELOPMENT.md`、主网 runbook `MAINNET_RUNBOOK.md`、Keplr 链注册 `keplr-chain-registry.json`、mcchain 模块职责 `module_mcchain.md`、预言机框架 `ORACLE_FRAMEWORK.md`、移动端集成 `mobile_sdk_integration.md`。

---

# 附录 C　术语表

| 术语 | 含义 |
|---|---|
| **MC / MobileChain** | 本项目与其主币的名称 |
| **umc** | MC 的最小单位，1 MC = 1,000,000 umc（精度 6） |
| **DePIN** | 去中心化物理基础设施网络，用代币激励协调现实设备提供基础设施 |
| **全节点（Full Node）** | 完整参与链的验证与数据的节点；MC 力求让手机也能承担 |
| **验证人（Validator）** | 参与共识出块的节点，MC 要求最低自抵押 10 万 MC |
| **设备节点 / 手机节点** | 参与 DePIN 设备激励的手机，门槛极低，无需 10 万 MC |
| **认证（Attestation）** | 证明设备真实可信的链上凭证，有效期 30 天 |
| **女巫攻击（Sybil Attack）** | 用大量伪造身份骗取激励；MC 用女巫设备绑定对抗 |
| **罚没（Slash）** | 对离线/作弊/伪造认证的惩罚，按基点分级，回流质押安全池 |
| **基点（bps）** | 万分之一，10000 bps = 100% |
| **托管（Escrow）** | EdgeAI 中需求方预先锁定赏金，保证算力方拿得到钱 |
| **乐观结算（Optimistic Settlement）** | 默认相信结果诚实，无争议自动放款，有争议才仲裁 |
| **五池（Five Pools）** | 设备激励 55 / 质押安全 15 / 团队 12 / 基金会 13 / 早期开发 5 |
| **零通胀（Zero Inflation）** | 创世一次性铸满，此后永不增发 |
| **质押安全池** | 15% 预铸储备，零通胀下养活验证人的核心资金来源 |
| **cliff / 线性释放** | cliff 为完全锁定期；之后按区块逐步线性解锁 |
| **maccPerms** | 模块账户权限映射，决定各模块账户能否铸币/销毁等 |
| **共振分发（Resonance Distribution）** | 激励节奏与网络真实贡献同频，而非与时间机械流逝同频 |

---

# 附录 D　引用与依据

本白皮书所述一切均以 MC 开源代码库为唯一权威依据。核心依据包括：

- 源码库：`x/tokenomics`、`x/depin`、`x/phonenode`、`x/edgeai`、`x/mcchain`、`app/ante.go`、`app/app.go`。
- 依赖：Cosmos SDK v0.47.14、CometBFT v0.37.6（见 `go.mod`）。
- 项目文档：`docs/` 目录下的模块白皮书、代币分配设计、开发与部署文档。
- 验证记录：五池模型经 `go build/vet/test` 全绿、`init`+`validate-genesis` 通过、真实启动出块并链上查询确认（`bank total = 1e15`、`depin.initial_pool = 5.5e14`、`minted_supply = cap`、五池余额与团队 vesting 符合设计）。

行业公开数据（如其它公链的分配结构）仅在内部设计研讨中作为参考，本正式白皮书不做任何竞品对比、不点名任何其它项目——**MC 只讲自己要走的路。**

---



## A.7 DEX 原生交易所（x/dex）

| 参数 | 值 | 说明 |
|---|---|---|
| 做市模型 | 常量积（x×y=k） | AMM |
| 初始流动性池 MC/USDT | 500 万 MC + 10 万 USDT | 创世注入 |
| 初始价格 | 0.02 USDT/MC | 对应初始 FDV $2,000 万 |
| 交易手续费 | 0.3% | 其中 0.05% 销毁 |
| LP 最低锁仓 | 7 天 | — |
| 流动性激励（前 6 个月） | 5,000 MC/天 | 从设备激励池划拨，可治理 |

## A.8 推荐系统（x/referral · 规划中）

| 参数 | 值 | 说明 |
|---|---|---|
| 一代 / 二代 / 三代 | 10% / 5% / 2% | 被推荐人 DePIN 收益加成 |
| 单人日上限 | 500 MC（5e8 umc） | — |
| 全网日熔断 | 20,600 MC（2.06e10 umc） | — |
| 推荐预算占比 | 设备激励池的 15%（可治理） | — |
| 最多代数 | 3 | — |
| 首阶段 | 仅开放一代 | 3 个月后治理决定 |

## A.9 销毁机制

| 来源 | 比例 | 说明 |
|---|---|---|
| DePIN 任务赏金 | 5% | 每笔结算时销毁 |
| DEX 交易手续费 | 0.05%（从 0.3% 中抽） | — |
| 推荐加成 | 1% | 推荐奖励发放时销毁 |
| 治理提案押金 | 10% | 通过后不退还部分 |

## A.10 验证节点（Verifier）

| 参数 | 值 | 说明 |
|---|---|---|
| 最低质押 | 50,000 MC（5e10 umc） | — |
| 验证奖励 | 任务赏金的 15% | 多验证节点均分 |
| 验证脚本规范 | Python 3.10+、≤ 30s、禁止联网、JSON 输出 | 脚本哈希存链 |

