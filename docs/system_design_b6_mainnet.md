# MobileChain B6 批次 · 主网就绪（DAO 路线 / 治理模块 / 审计清单 / 主网 genesis·配置·部署）· 增量架构设计 + 任务分解

**文档类型**：增量架构设计（基于 B1–B5 已落地模块，仅描述变更；docker-compose/runbook/脚本为运维文本可落盘）
**批次**：B6（主网就绪）— 路线图收尾与主网启动
**作者**：高见远（Gao），Architect
**语言**：简体中文
**技术栈**：Cosmos SDK v0.47.3 + cometbft v0.37.1 + Ignite。**链上仅最小治理增强**；验收以本机 `ignite chain build` + `mcchaind init`/启链 + genesis↔B1 cap 一致性校验 + 审计/部署文档评审为准。
**配套图**：`docs/b6_mainnet_class-diagram.mermaid`、`docs/b6_mainnet_sequence-diagram.mermaid`。

---

# Part A · 系统设计

## 1. 实现方案与框架选型

| 难点 | 方案 |
|------|------|
| DAO 治理（社区池支配） | 复用 cosmos `gov v1`；新增**极简 `x/dao` 模块**注册 `CommunitySpendProposal` 处理者（gov 通过后调用 `bankKeeper.SendCoinsFromModuleToAccount(community, recipient, amt)`）。参数治理沿用 B2 `paramproposal` 路由。 |
| 治理参数 | 在 genesis `gov` 模块固化 voting period / min deposit / quorum / threshold / veto，生产默认值。 |
| 主网 genesis/配置 | 基于测试网 genesis 固化 B1 分配 + 团队 vesting + B2–B4 参数 + 种子账户；`config.toml`/`app.toml` 生产默认值（含 B4 pruning/snapshot）。 |
| 部署/runbook | `deploy/docker-compose.yml`（validator/fullnode/snapshot）+ `docs/runbook_mainnet.md` + `scripts/`（init/start/upgrade）。运维文本，可落盘。 |
| 审计清单 | `docs/audit_checklist.md`：经济/安全/代码/渗透四维度 + 通过标准（必由第三方）。 |
| cap 锁定 | `total_supply_cap` 维持 B1 链上常量 + genesis 校验双保险，**本批次仍锁定**，仅文档规划未来治理化路径。 |

- **框架**：延续 cosmos 标准模块；`x/dao` 极简（无状态、仅提案处理）。**无新第三方依赖**。
- **关键决策（推荐默认，不抛用户）**：(1) community 池经 `x/dao` 的 `CommunitySpendProposal` 由治理支配，v1 不做 timelock（列为路线图可选）；(2) `total_supply_cap` 锁定不变；(3) gov 参数与社区池支配即「资金」阶段 DAO，参数治理（B2）即「参数」阶段；(4) 部署/genesis/审计均为文本可落盘，不在沙箱执行。

## 2. 文件列表及相对路径

### 2.1 新增 `x/dao`（极简治理支配模块）
| 文件 | 关键变更 |
|------|----------|
| `x/dao/types/keys.go` | `ModuleName="dao"`、`RouterKey="dao"`（提案路由键） |
| `x/dao/types/proposal.go` + `proposal.proto`/`proposal.pb.go`(生成) | `CommunitySpendProposal{recipient string, amount sdk.Coins, description string}` |
| `x/dao/keeper/keeper.go` | 注入 `BankKeeper` + `communityModuleName`；`SpendFromCommunity(ctx, recipient, amt) error` |
| `x/dao/keeper/proposal_handler.go` | `NewCommunitySpendProposalHandler(k) govv1beta1.Handler`：解析提案 → `k.SpendFromCommunity` |
| `x/dao/module.go` | `AppModuleBasic`（`RegisterInterfaces` 注册提案类型）+ `AppModule`（`RegisterProposalHandlers` 在 app.go govRouter 注册）；无状态，InitGenesis 空；`ConsensusVersion=1` |

### 2.2 修改 `app/app.go`（**串行共享，务必精确**）
- **imports**：新增 `daomodule "mcchain/x/dao"`、`daomodulekeeper`、`daomoduletypes`。
- **ModuleBasics**：追加 `daomodule.AppModuleBasic{}`。
- **New() 装配**：`app.DaoKeeper = *daomodulekeeper.NewKeeper(app.BankKeeper, tokenomicsmoduletypes.CommunityPoolName)`（无 store，创建顺序无约束，建议放在 gov 之前）。
- **govRouter**：在既有 `paramproposal` 路由后追加 `AddRoute(daomoduletypes.RouterKey, daomodulekeeper.NewCommunitySpendProposalHandler(app.DaoKeeper))`。
- **mm 模块列表**：追加 `daoModule`（空 InitGenesis，置于末尾附近）。
- **SetOrderBeginBlockers/EndBlockers/InitGenesis**：追加 `daomoduletypes.ModuleName`（空 InitGenesis）。
- **initParamsKeeper**：dao 无 params，**不新增 subspace**。
- **maccPerms**：**不变**（复用 B1 既有 `community` 模块账户）。
- **config.yml**：**不变**（沿用 B1，不新增币）。

### 2.3 主网 genesis / 节点配置 / 部署（文本可落盘）
| 文件 | 关键变更 |
|------|----------|
| `config/genesis.production.json`（生成模板） | 固化 B1 分配（team 3-of-5 vesting + community/ecosystem 模块账户 + `depin` InitialPool 1e14 切片）、B2/B3/B4 参数、gov 生产参数、种子 gentx |
| `scripts/build_genesis.sh` | 组装生产 genesis（init + add-genesis-account + gentx + collect-gentxs + 校验 cap） |
| `config/config.toml.production` / `config/app.toml.production` | 生产默认值：seeds/persistent_peers、B4 pruning(`custom` keep-recent=100)/snapshot(interval=1000)、p2p/mempool 调优 |
| `deploy/docker-compose.yml` | validator / fullnode / snapshot 三类服务 + 卷/网络 |
| `scripts/{init,start,upgrade}.sh` | 启链/升级脚本 |
| `docs/runbook_mainnet.md` | init → collect-gentxs → start → upgrade(plan/height) 全流程 |
| `docs/dao_roadmap.md` | DAO 四阶段（信号→参数→资金→cap 治理化）+ 未来 timelock/cap 治理化路径 |
| `docs/audit_checklist.md` | 经济/安全/代码/渗透四维度审计项 + 通过标准 |
| `docs/monitoring_baseline.md` | 基础监控指标 + 升级治理流程 |

## 3. 数据结构与接口（类图，见 `docs/b6_mainnet_class-diagram.mermaid`）

**`x/dao` 提案结构（proto）**
```proto
// x/dao/types/proposal.proto
message CommunitySpendProposal {
  string title = 1;
  string description = 2;
  string recipient = 3;   // 收款地址
  repeated cosmos.base.v1beta1.Coin amount = 4;
}
```
**核心方法**：`dao.Keeper.SpendFromCommunity(ctx, recipient, amt)`；`NewCommunitySpendProposalHandler(k)`（gov 通过后执行）。

## 4. 程序调用流程（时序图，见 `docs/b6_mainnet_sequence-diagram.mermaid`）

- **① 社区池治理支配**：提案人 `SubmitProposal(CommunitySpendProposal)` → gov 投票（voting period/quorum/threshold 由 genesis 固化）→ 通过后 gov 调 `dao` 处理者 → `SpendFromCommunity` → `bankKeeper.SendCoinsFromModuleToAccount(community, recipient, amt)`。**绝不 MintCoins**，仅转移 B1 已铸社区池余额。
- **② 参数治理（B2 既有）**：`SubmitProposal(ParameterChangeProposal)` → 改 phonenode/depin/slashing  subspace 参数（不含 `total_supply_cap`）。
- **③ 主网启链**：`scripts/build_genesis.sh` 组装 genesis（与 B1 cap 一致）→ `mcchaind init` + `collect-gentxs` + `start` → docker-compose 拉起 validator/fullnode/snapshot。
- **④ cap 锁定校验**：genesis 加载即触发 B1 `TotalSupplyCap` 常量 + `InitGenesis` 校验；本批次不开放 cap 治理化（仅路线图）。

## 5. 任务列表（有序、含依赖、优先级）
| Task | 名称 | 依赖 | 优先级 | 涉及文件 |
|------|------|------|--------|----------|
| **T1** | `x/dao` 极简治理支配模块（CommunitySpendProposal）+ app.go 装配 | 无（依赖 B1 community 账户 + B2 gov） | P0 | `x/dao/**`、`app/app.go`、`govRouter` 注册 |
| **T2** | 主网生产 genesis 模板 + 生成脚本（固化 B1–B5 分配/参数/gentx） | T1 | P0 | `config/genesis.production.json`、`scripts/build_genesis.sh`、`docs/genesis_mainnet.md` |
| **T3** | 节点生产配置默认值（config.toml/app.toml：seeds/pruning/snapshot） | 无 | P0 | `config/config.toml.production`、`config/app.toml.production`、`docs/node_config_mainnet.md` |
| **T4** | 部署（docker-compose + runbook + init/start/upgrade 脚本） | T2,T3 | P0 | `deploy/docker-compose.yml`、`docs/runbook_mainnet.md`、`scripts/{init,start,upgrade}.sh` |
| **T5** | DAO 路线 + 审计清单 + 监控基线文档 | T1 | P1 | `docs/dao_roadmap.md`、`docs/audit_checklist.md`、`docs/monitoring_baseline.md` |
**P0 验收锚点**：`CommunitySpendProposal` 经 gov 通过后从 community 账户成功拨付且 `minted_supply` 不变；genesis 与 B1 `TotalSupplyCap=1e15` 一致、可 `mcchaind init` 启链；docker-compose 三件套 + runbook 可照启链；cap 仍锁定（无治理改 cap 路径生效）。

## 6. 依赖包列表
- `github.com/cosmos/cosmos-sdk` v0.47.3（`gov v1`/`params`/`distribution` 提案机制均内置）；`x/tokenomics`（community 账户、cap）。**无需新增任何第三方依赖**。

## 7. 共享知识（跨文件约定，含与 B1–B5 交互边界）
- **denom/单位**：同 B1（`umc`）。
- **cap 锁定（铁律）**：`total_supply_cap` 仍是 B1 链上常量 + genesis 校验双保险；本批次**不开放**治理改 cap；`CommunitySpendProposal` 仅转移已铸 community 余额，**绝不 MintCoins**，`minted_supply` 不变。
- **社区池支配边界**：`x/dao` 仅能从 `community` 模块账户（B1 `CommunityPoolName`）对外拨付，且必须经 gov 投票通过（提案处理者由 gov 调用）；不得触碰 team/ecosystem 账户或 cap。
- **DAO 分阶段（路线图）**：① 信号（链下治理论坛/投票）→ ② 参数（B2 `paramproposal`，不含 cap）→ ③ 资金（本批 `CommunitySpendProposal`）→ ④ cap 治理化（未来，由 DAO 投票后改常量+genesis 校验，需硬分叉，本批仅文档规划）。
- **genesis 一致性**：生产 genesis 须使 `sum(各池 allocated_amount) == minted_supply <= TotalSupplyCap`，且生态池→depin 切片 `1e14` 已转；构建脚本内置 cap 校验。
- **与 B4 配置衔接**：生产 `app.toml` 复用 B4 移动端 pruning/snapshot 默认值，保证轻客户端快速同步。

## 8. 待明确事项（仅真正需后续定稿，占位并注明）
1. **真实团队多签/锁仓密钥**：B1 占位 3-of-5 主网前替换为真实团队多签；genesis 中 `team` vesting 账户地址须同步更新。
2. **timelock 模块**：v1 不做；DAO 演进到高价值资金阶段时再评估独立 timelock 模块或合约，列为路线图可选。
3. **审计机构**：`audit_checklist.md` 列项与标准，真实审计必由第三方执行（本批不指定机构）。
4. **种子节点/创世验证人集合**：生产 genesis 的最终 gentx 集合与种子节点列表由运营方在启链前定稿（本批给模板与脚本）。
5. **cap 治理化的硬分叉流程**：仅文档规划，实际启用需社区共识 + 软件升级提案，不在本批次落地。

> 其余均按推荐默认定稿，未向用户抛待确认。
