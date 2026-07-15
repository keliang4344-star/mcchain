# MobileChain 运行手册（Runbook）

本文件记录在本机启动一条 `mcchain` 测试网的最小步骤，以及 P0 相关运行期配置。

> 前提：本机已安装 `go`（>=1.21）、`ignite`（>=0.27）、`protoc`（如需重新生成 pb 文件，本增量已手改，可跳过）。
> 本增量代码在沙箱无法运行 go，验收一律以**本机**为准。

## 1. 编译

```bash
cd $HOME/mcchain
ignite chain build
```

编译产物为 `mcchaind`（链守护进程）。若出现 `go: ...` 依赖问题，先 `go mod tidy`。

## 2. 初始化链

```bash
# 首次初始化（生成 ~/.mcchain 配置与 genesis.json）
ignite chain init
```

初始化会按 `config.yml` 创建创世账户（alice / bob）与创世验证人（alice）。

### 2.1 设置出块间隔（P0-2）

`ignite` 的 `config.yml` 不支持直接设置块时间，需手动编辑已生成的 cometbft 配置：

```bash
# 路径示例（Windows）
notepad %USERPROFILE%\.mcchain\config\config.toml
# 或 Linux/macOS
vim ~/.mcchain/config/config.toml
```

将

```toml
timeout_commit = "5s"
```

改为

```toml
timeout_commit = "4s"
```

目标出块间隔 4s（实测 3–5s 验收区间）。

## 3. 启动链

```bash
ignite chain start
```

或后台启动：

```bash
ignite chain start --force-reset
```

## 4. 关于 genesis 验证人 min_self_delegation（重要）

P0-1 要求所有验证人（含 genesis 验证人 alice）的 `min_self_delegation == 30000000000`（=30k MC = 3e10 umc）。

- **创世验证人**由 `InitGenesis` 直接创建，**不经过 ante decorator**，因此单纯靠 ante 装饰器无法约束它。
- 本增量已在 `app/app.go` 的 `InitChainer` 中、`app.mm.InitGenesis(...)` **之后**增加兜底逻辑：遍历全部验证人，将 `MinSelfDelegation < 3e10` 的强制抬到 `3e10` 并 `SetValidator`。

**因此你无需手动修改 `genesis.json` 的 `min_self_delegation` 字段**——代码启动时会自动纠正。同样，`BondDenom` 也会在 InitChainer 被强制覆盖为 `umc`。

如需核验：

```bash
mcchaind q staking params          # bond_denom: umc
mcchaind q staking validators       # 任意验证人 min_self_delegation: "30000000000"
```

## 5. DePIN 初始池（P1-1）

`x/depin/types/params.proto` 的 `Params.InitialPool` 默认 `1e14 umc`（=1e8 MC），在 `depin` 模块 `InitGenesis` 经 `MintCoins` 一次性铸入 `depin` 模块账户（该账户已含 `Minter` 权限）。运行期奖励只从池中拨付，**不再 mint**，符合总量恒定 10 亿 MC。

```bash
mcchaind q bank total                       # 观察 depin 模块账户持有 InitialPool
mcchaind q depin params                     # initial_pool / reward_denom
```

## 6. 常见问题

| 现象 | 排查 |
|---|---|
| `min self delegation ... < lower bound` | 提交 `MsgCreateValidator`/`MsgEditValidator` 时 `min_self_delegation` 低于 3e10 umc，请抬高 |
| 出块间隔不在 3–5s | 确认已设置 `timeout_commit = "4s"` 并重启 |
| 奖励发放为 0 / 报错 `creator not registered in phonenode` | 提交贡献前需先用同一设备地址（=节点 Address）调用 `MsgRegisterNode` 注册 phonenode 节点 |

## 7. 经济模型模块 tokenomics（B1 新增）

`x/tokenomics` 是「发行与分配总账」，唯一持 `Minter` 权限；`depin` 不再自铸（其 `maccPerms` 已移除 `Minter`）。`tokenomics.InitGenesis` 须在 `depin.InitGenesis` **之前**执行（已在 `app.go` 的 `SetOrderInitGenesis` 固定）。

初始化与启动后，可验收经济模型：

```bash
# 总量上限与已发行量（均应为 1e15 umc = 1e9 MC）
mcchaind q tokenomics supply

# 三大池分配占比、拨付额与当前余额
# 期望占比 team=1500 / community=3500 / ecosystem=5000
mcchaind q tokenomics allocations

# 团队池释放进度（随区块时间推进，vested 增长、remaining 减少、progress_bps 上升）
mcchaind q tokenomics release

# 生态池在 genesis 向 depin 模块账户拨付 1e14 umc（InitialPool）
mcchaind q bank total
```

不变量（crisis 模块每轮校验）：`minted_supply ≤ total_supply_cap`、各池 `allocated_amount` 之和 == `minted_supply`。链启动若干高度后若未因这两项 halt，即说明经济模型不变量成立。

> 团队多签为占位公钥（位于 `x/tokenomics/types/keys.go`），**主网 genesis export 前必须替换为真实 3-of-5 团队多签公钥**。
