# x/mcchain 模块职责说明

> 目的：澄清 `x/mcchain` 在 MC 公链中的定位与职责，消除「空壳模块 / 系统模块」概念混淆。
> 对应白皮书 §2.4 待办与 §4.1 改进建议（P2：明确 `x/mcchain` 职责）。

## 1. 定位：基础/系统锚点模块

`x/mcchain` 是 Ignite 脚手架生成的第一条模块（习惯称 launch / base module），
是 **Cosmos SDK 链的标准「锚点」**，并非业务模块，也**不是死代码**。
它在依赖图中是一个**独立叶子节点**：业务模块（tokenomics → depin → phonenode → edgeai）
自成一条依赖链，**不 import `x/mcchain`**，彼此也不被 `x/mcchain` 依赖。

```
tokenomics → depin → phonenode → edgeai   （业务闭环，单向依赖）
mcchain                                      （基础锚点，独立存在）
```

保留它的原因：
- 它是链上 `mc` 地址前缀命名空间的根模块名（`types.ModuleName = "mcchain"`）。
- 它持有 params 模块的 `Subspace`（参数子空间基础设施），是未来链级系统参数的天然归属地。
- 删除它需要重新脚手架，且会破坏 `app.go` 中已固化的模块装配与 genesis 顺序，收益为零、风险很高。

## 2. 当前实际职责（逐文件证据）

| 文件 | 实际内容 | 职责 |
|------|----------|------|
| `x/mcchain/keeper/keeper.go` | `Keeper{cdc, storeKey, memKey, paramstore}`，`NewKeeper` 注入 params 子空间；`Logger` | 提供参数子空间与日志 |
| `x/mcchain/types/params.go` | `Params` 当前为空（`ParamSetPairs()=∅`，`Validate()=nil`）；`DefaultParams()`、`String()` | 预留链级系统参数结构 |
| `x/mcchain/genesis.go` | `InitGenesis` 写 params；`ExportGenesis` 读 params | genesis 入/出参 |
| `x/mcchain/module.go` | 完整 `AppModule`：`RegisterServices`、`RegisterGRPCGatewayRoutes`、`GetTxCmd`/`GetQueryCmd`、`InitGenesis`/`ExportGenesis`、`ConsensusVersion()=1`、空 `BeginBlock`/`EndBlock`、`RegisterInvariants` 空 | 标准模块装配，可编译可运行 |
| `x/mcchain/client/cli/tx.go` | `GetTxCmd` 当前无子命令（`RunE=ValidateCmd`） | 预留交易命令根 |
| `x/mcchain/client/cli/query.go` | `GetQueryCmd` 含 `params` 查询 | 链级参数查询入口 |

结论：**模块本身功能完整、可编译、参与 genesis 与查询**；只是目前没有业务字段，
这是设计使然——它属于「系统层」，而非「业务层」。

## 3. 与「系统模块」概念的区分

- **业务模块**（tokenomics/depin/phonenode/edgeai）：承载 DePIN、移动节点认证、边缘 AI 经济闭环。
- **基础模块**（mcchain）：承载链级、跨业务的系统参数与协调面，不绑定任何具体业务。
- 二者不冲突：`mcchain` 不应被合并进 `app` 层（那会丢失模块边界与可升级性），也不应包含
  任何 DePIN/AI 业务逻辑（那会污染系统层）。

## 4. 建议的未来职责（落地方向）

当链需要「链级 / 跨模块」能力时，优先落在 `x/mcchain`，而非塞进业务模块：

1. **链级系统参数**：在 `Params` 中定义全局开关、跨模块常量（如最低自抵押、全局费率上限）。
   配套在 `params.go` 增加 `ParamSetPairs` 与校验，并在 `genesis` 中初始化。
2. **治理/升级协调**：作为 `x/gov` 提案的执行落点之一，承载链级配置变更的提案 handler。
3. **跨模块系统配置查询**：通过已有 `GetQueryCmd`/`RegisterGRPCGatewayRoutes` 暴露统一系统参数面，
   供前端（web「模块实时查询」）与运维读取。
4. **特性开关（feature flags）**：以 params 形式灰度启用/停用某些链上行为，无需硬分叉。

> 实施上述任一项时，需同步 bump `module.go` 中的 `ConsensusVersion()`（状态破坏性变更）。

## 5. 一句话总结

`x/mcchain` 是 MC 公链的**系统锚点模块**：当前为空参数结构 + 完整模块骨架，
预留承载链级系统参数、治理协调与跨模块配置；它独立于业务模块依赖链，
是标准 Cosmos SDK 基础模块，应保留并逐步填充系统层职责，而非删除或并入 app 层。
