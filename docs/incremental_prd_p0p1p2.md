# MobileChain 增量 PRD（P0 / P1 / P2）

> 文档类型：**增量 PRD**（基于已有代码，只描述变更，不写实现代码）
> 代码库路径：`$HOME/mcchain`
> 技术栈：Cosmos SDK v0.47.3 + cometbft v0.37.1，Ignite 脚手架；自定义模块 `x/depin`、`x/phonenode`
> 语言：简体中文
> 环境约束：本沙箱**无法运行 go**（Windows PE 二进制 Exec format error，GOMODCACHE 为空）。因此所有【验收标准】一律以「用户本机 `ignite chain build` 通过 + `go test ./...` 通过 + 链上行为正确」为准，**不得**写「在本环境运行测试」。

---

## 0. 现状核对（已读代码确认，供架构师采信）

| 事实 | 落点 | 结论 |
|---|---|---|
| depin 模块账户权限 | `app/app.go` maccPerms：`depinmoduletypes.ModuleName: {Minter, Burner, Staking}` | ✅ 权限足够（可发币/铸/烧） |
| 奖励发放实现 | `x/depin/keeper/msg_server_submit_contribution.go`：`bankKeeper.SendCoinsFromModuleToAccount(depin模块→设备)` | ✅ 路径正确，但依赖模块账户有钱 |
| RewardDenom 当前值 | 同文件 `const RewardDenom = "stake"`（dev 占位） | ⚠️ 需锁 `"umc"` |
| depin 依赖的 keeper | `x/depin/types/expected_keepers.go`：仅 `AccountKeeper`、`BankKeeper` | ⚠️ 缺 `PhonenodeKeeper`（P2 需加） |
| phonenode 节点主键 | `x/phonenode/keeper/store.go`：`GetNode(ctx, addr)`（按节点 Address 存） | P2 关联用 |
| 移动端端点注册 | `x/depin/module.go`、`x/phonenode/module.go` 均已 `RegisterInterfaces` / `RegisterGRPCGatewayRoutes` / `RegisterServices` | ✅ tx/query 已注册到 codec/gRPC/REST |
| 出块时间控制 | cometbft `config/config.toml` 的 `timeout_commit`（ignite `config.yml` 不支持直接设块时间） | P0 落点 |
| 代币换算 | 1 MC = 1e6 umc → **100k MC = 100000000000umc（1e11 umc）** | 全文统一口径 |
| 当前 genesis 资金 | `config.yml`：validator alice `bonded: 100000000umc`（=100 MC）；alice 余额 ≈200 MC | ⚠️ **远低于 100k MC 下限**，P0 须同步抬高 |

---

## 1. 产品目标（一句话）

让 mcchain 测试网具备「可跑起来的验证人经济 + 端到端可发币的 DePIN 贡献闭环 + 移动节点前置关联与可对接的移动端入口」，三步到位逼近可对外联调的测试网。

---

## 2. 增量范围说明（受影响模块 / 文件类别）

**P0（质押与链参数）**
- `config.yml`（仓库根）：抬高 genesis 验证人自质押与账户余额至 ≥100k MC；保持 denom = `umc`。
- `app/app.go`：确保 staking genesis `BondDenom = "umc"`（与 config.yml 一致）；插入 `MinSelfDelegation` 全局下限校验的 ante decorator。
- `config/config.toml`（链初始化后生成）：`timeout_commit = "4s"`。
- 新增文件（建议）：`x/depin/ante/` 或 `app/ante.go` 中的 `MinSelfDelegationDecorator`（若选全局强制方案，见 §4/§5）。

**P1（DePIN 池拨付，让 submit 发币）**
- `x/depin/types/params.proto` + `params.pb.go` + `params.go`：新增 `InitialPool`（uint64，umc）参数；将 `RewardDenom` 由 const 改为 param（默认 `"umc"`）。
- `x/depin/keeper/msg_server_submit_contribution.go`：改用 `params.RewardDenom`（去掉 const）。
- `x/depin/genesis.go`：`InitGenesis` 中向 depin 模块账户拨付 `InitialPool` umc（来源见 §4）。
- 关联 genesis 文件（`config/genesis.json` 或 InitChainer 预设）。

**P2（关联 + 移动端对接）**
- `x/depin/types/expected_keepers.go`：新增 `PhonenodeKeeper` 接口（`HasNode(ctx, addr) bool`）。
- `x/depin/keeper/keeper.go` + `NewKeeper` 签名：注入 `PhonenodeKeeper`。
- `x/depin/keeper/msg_server_submit_contribution.go`（或 `register_device.go`）：提交贡献前校验 phonenode 已注册。
- `x/depin/types/errors.go`：新增 `ErrPhonenodeNotRegistered`（建议 code 1107）。
- `app/app.go`：把 phonenode keeper 接线进 depin keeper。
- 产出文档：`docs/mobile_sdk_integration.md`（移动端 SDK 对接文档，轻量交付，不新建 SDK 工程）。

**不受影响**：KVStore 迁移（已落地）、模块核心 KV 结构、proto 服务定义（tx/query 已注册）。

---

## 3. 需求池

### P0 — 离「可跑测试网」最近

#### P0-1 最低自质押 100k MC
- **需求描述**：链上对任意验证人强制「最低自质押 = 100k MC（1e11 umc）」。
- **验收标准**：
  1. 用户本机 `ignite chain build` + `go test ./...` 通过。
  2. 链起后 `mcchaind q staking validator <addr>` 显示 `min_self_delegation: "100000000000"`。
  3. 以低于 1e11 umc 的 `MinSelfDelegation` 发起 `MsgCreateValidator` / `MsgEditValidator` 被拒绝（若选全局强制方案）。
  4. genesis 验证人自质押 ≥ 1e11 umc，链可正常 Init/出块。
- **优先级**：P0（必须）
- **关键约束**：cosmos-sdk v0.47 的 x/staking **无全局最低自质押参数**，`MinSelfDelegation` 仅是每个验证人字段。方案选型见 §4/§5（推荐全局强制）。⚠️ 当前 `config.yml` 验证人 `bonded` 仅 100 MC、alice 余额 ≈200 MC，**必须同步抬高到 ≥100k MC**，否则无法满足下限或链无法初始化。

#### P0-2 出块间隔 3–5s
- **需求描述**：测试网实际出块间隔落在 3–5 秒。
- **验收标准**：
  1. 链起后连续观察 ≥20 个区块，`mcchaind q block <n>` 时间戳差值中位数 ∈ [3s, 5s]（建议目标 4s）。
  2. `config/config.toml` 中 `timeout_commit = "4s"`（链初始化后设置，建议用脚本/文档固化，ignite `config.yml` 无法原生设置块时间）。
- **优先级**：P0（必须）
- **关键约束**：块时间由 cometbft 的 `timeout_commit` 控制；属运行期配置，不进 `config.yml`。验收在用户本机测试网观察。

#### P0-3 denom 一致性（umc）
- **需求描述**：staking 的 `BondDenom` 必须为 `umc`，与 `config.yml` 账户 denom 一致。
- **验收标准**：
  1. `mcchaind q staking params` 显示 `bond_denom: umc`。
  2. 账户、验证人绑定、DePIN 奖励均使用 `umc`，无 `stake` 残留。
- **优先级**：P0（必须）
- **关键约束**：cosmos staking 默认 `BondDenom = "stake"`，需显式在 staking genesis（genesis 文件或 InitChainer 预设）置为 `umc`；architect 核对 `config.yml` 与 staking params 一致性。

### P1 — 让 submit 真的能发币（端到端跑通）

#### P1-1 DePIN 池 genesis 拨付
- **需求描述**：在 genesis 给 depin 模块账户拨付一笔初始 `umc`（来源见 §4），使 `SubmitContribution` 发得出奖励。
- **验收标准**：
  1. 链起后 `mcchaind q bank total` / 模块账户余额显示 depin 模块账户持有 `InitialPool` umc。
  2. 注册并 attest 设备后，提交一次有效贡献：`mcchaind q bank balances <设备地址>` 中 `umc` 余额增加，且 denom 为 `umc`。
  3. `go test ./x/depin/...` 通过（建议补充：genesis 拨付 + submit 后余额变化的集成测试）。
- **优先级**：P1（应该）
- **关键约束**：模块账户权限已含 `Minter`，可在 `InitGenesis` 铸入；但产品口径为「总量恒定 10 亿 MC，不额外增发」，故池资金应计入 1B 上限（见 §4 两种来源方案）。`RewardDenom` 须由 `const "stake"` 改为锁定 `"umc"`（param 或常量，推荐 param 以便可配）。

#### P1-2 RewardDenom 锁定 umc
- **需求描述**：将 `x/depin/keeper/msg_server_submit_contribution.go` 的 `const RewardDenom = "stake"` 改为生产值 `umc`。
- **验收标准**：
  1. 奖励发放 denom 为 `umc`（P1-1 验收第 2 条已覆盖）。
  2. 不再出现 `stake` 作为奖励 denom。
- **优先级**：P1
- **关键约束**：建议并入 P1-1 的 param 改造，避免散落常量。

### P2 — 关联 + 移动端对接

#### P2-1 depin↔phonenode 关联（节点注册为贡献前置）
- **需求描述**：提交贡献（creator）前，必须已在 `x/phonenode` 注册过节点，否则拒绝。
- **验收标准**：
  1. **未注册** phonenode 的设备调用 `SubmitContribution` → 返回 `ErrPhonenodeNotRegistered`（建议 code 1107），不发币。
  2. **已注册** phonenode 的设备调用 `SubmitContribution` → 正常发币（denom=umc）。
  3. `go test ./x/depin/...` 含上述正反用例通过。
- **优先级**：P2（锦上添花）
- **关键约束**：depin 当前 `expected_keepers.go` 无 phonenode keeper；架构师需新增 `PhonenodeKeeper` 接口（`HasNode(ctx, addr) bool`）并接线。关联键建议：depin 设备地址（= `SubmitContribution.Creator`）等于 phonenode 已注册的节点 `Address`（phonenode 节点以 `Address` 为主键，见现状核对）。关联键口径见 §5。

#### P2-2 移动端 SDK 对接文档（轻量交付）
- **需求描述**：确认 depin/phonenode 的 tx 与 query 已注册到 codec/gRPC/REST（代码已确认注册），并产出《移动端 SDK 对接文档》。
- **验收标准**：
  1. 文档列清：gRPC/REST 端点清单、`umc` denom 约定、关键 tx 示例（`MsgRegisterNode`、`MsgRegisterDevice`、`MsgAttestDevice`、`MsgSubmitContribution`）、query 示例。
  2. 不新建完整 SDK 工程（范围界定见 §5）。
- **优先级**：P2
- **关键约束**：端点注册已由 ignite scaffold 完成（已核对 `module.go`），本项核心是**文档**而非接线；如架构师发现某端点未暴露，再补注册。

---

## 4. 关键设计决策建议（给架构师参考）

### D1. 最低自质押下限实现方案（P0-1）
- **推荐：全局强制（ante decorator）**。理由：仅 genesis 写死 `MinSelfDelegation` 无法阻止后续验证人把下限设为 0，形同虚设。
- 实现要点：
  - 新增 `MinSelfDelegationDecorator`（`sdk.AnteDecorator`），在 `AnteHandle` 中遍历 `tx.GetMsgs()`，对 `stakingtypes.MsgCreateValidator` / `MsgEditValidator` 校验 `MinSelfDelegation >= 100000000000umc`，否则返回错误。
  - 在 `app.go` 的 ante 装饰链（ignite 的 `app/ante.go` 或 `SetAnteHandler`）中插入该 decorator。
  - genesis 验证人同时把 `min_self_delegation` 设为 1e11 umc，并把 `bonded` 与账户余额抬高到 ≥1e11 umc（见 D4）。
- 备选（仅 genesis 校验者设死）：改动小，但无运行期约束，不推荐作为唯一手段。

### D2. DePIN 初始池金额与来源（P1-1）
- **推荐默认值**：`InitialPool` param 默认 **1e8 MC = 1e14 umc**（约 10 亿总量的 10%），做成可配置 param。
- **来源（两种，推荐 A）**：
  - **方案 A（genesis 铸入，计入 1B 上限）**：`depin.InitGenesis` 调用 `bankKeeper.MintCoins(ctx, types.ModuleName, poolCoins)` 一次性铸入模块账户；因运行期奖励只从池内划拨、不再增发，符合「总量恒定 1B」。需在文档中声明 1B = genesis 账户余额之和 + 模块池。
  - **方案 B（从预留 genesis 账户转账）**：设一个预留账户持有池资金，`InitGenesis` 用 `SendCoinsFromAccountToModule` 转入模块账户（严格不增发）。更贴合「不铸造」，但需多维护一个 genesis 账户。
- 推荐 A（简单、可用），但无论哪种，都**不得**在运行期 mint 奖励。

### D3. block time 落点（P0-2）
- 在 `ignite chain init` 生成 `config/config.toml` 后，将 `timeout_commit = "4s"`（或写脚本/文档固化）。ignite `config.yml` 无法设块时间，勿在此纠缠。

### D4. genesis 资金一致性（P0-1 / P1 联动）
- `config.yml` 当前：validator `bonded: 100000000umc`（100 MC）、alice 余额 ≈200 MC —— **均远低于 100k MC**。
- 必须调整（推荐值）：
  - alice 账户：`coins` 含 `100000000000umc`（≥100k MC，留部分流动）。
  - `validators[0].bonded: 100000000000umc`（=100k MC，恰好等于下限）。
  - 其余 dev 账户（bob 等）按需保留少量 `umc` 用于测试。
- 同时确认 staking genesis `BondDenom = "umc"`（P0-3）。

### D5. P2 错误码与接口
- 新增：`x/depin/types/errors.go` → `ErrPhonenodeNotRegistered = sdkerrors.Register(ModuleName, 1107, "creator not registered in phonenode")`（1100–1106 已占用）。
- `PhonenodeKeeper` 接口建议方法：`HasNode(ctx sdk.Context, addr string) bool`（wrap `GetNode` 的 not-found）；或复用 `GetNode` + error 判断。架构师二选一。

### D6. P2 关联键口径
- **推荐**：depin 设备地址（`SubmitContribution.Creator`）== phonenode 已注册节点 `Address`。契合「手机即节点即设备」的移动端模型。
- 备选：按 phonenode 节点 `Creator`（owner）关联 —— 需先在 `NodeState` 持久化 owner 字段。当前 `NodeState` 未存 Creator，故默认不采用。

---

## 5. 待确认问题清单（需用户拍板，已附推荐默认）

| # | 问题 | 推荐默认 | 影响范围 |
|---|---|---|---|
| Q1 | 最低自质押用**全局强制（ante decorator）**还是仅**genesis 写死**？ | 全局强制（ante decorator），更可靠 | P0-1 |
| Q2 | genesis 验证人/账户余额按 100k MC 抬高的具体数值与账户分配？ | alice `100000000000umc`、validator `bonded: 100000000000umc` | P0-1 / D4 |
| Q3 | DePIN 初始池金额与来源方案（A 铸入计入 1B / B 预留账户转账）？ | `InitialPool` 默认 1e8 MC（1e14 umc），方案 A | P1-1 / D2 |
| Q4 | `RewardDenom` 改为 param 还是直接常量 `"umc"`？ | 改为 param（默认 `"umc"`），便于治理/可配 | P1-2 |
| Q5 | P2 关联键按**节点 Address** 还是**节点 owner（Creator）**？ | 按节点 `Address`（== depin 设备 Creator） | P2-1 / D6 |
| Q6 | 关联校验放在 `SubmitContribution` 还是同时 gate `RegisterDevice`？ | 仅 `SubmitContribution`（发币闸口） | P2-1 |
| Q7 | 移动端 SDK 对接是否界定为「仅产出对接文档 + 确认端点已注册」（不新建 SDK 工程）？ | 是，轻量交付，范围 OK | P2-2 |
| Q8 | `BondDenom = "umc"` 落点用 genesis 文件还是 InitChainer 预设？ | 写 genesis 文件（`config/genesis.json` 的 `staking.params.bond_denom`），或 InitChainer 强制覆盖 | P0-3 |

---

## 6. 验收总口径（通用）

- 所有编译/测试验收 = 用户本机 `ignite chain build` 成功 + `go test ./...` 通过。
- 所有链上行为验收 = 用户本机启动测试网后，用 `mcchaind` CLI / gRPC 观测（出块间隔、余额、denom、拒绝行为等）。
- 本沙箱不执行 go 命令；PRD 不依赖本环境测试结果。
