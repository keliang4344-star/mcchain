# MobileChain 经济模型

> 配套口径见 `docs/PROTOCOL_PARAMS.md`（参数）与 `docs/TOKEN_ALLOCATION.md`（分配与释放）。
> 本文件描述链上已落地的 tokenomics 模块（发行与分配总账），供社区与 SDK 引用。
> 分配口径：**五池模型**（2026-07 定稿并落链），常量单一来源 `x/tokenomics/types/keys.go`。

## 1. 总量上限

| 项 | 值 |
|----|----|
| 总供应量上限 `TotalSupplyCap` | **1e15 umc = 1e9 MC（10 亿 MC）** |
| 单位 | `1 MC = 1e6 umc`，链上金额一律以 `umc` 整数表达 |
| denom | `umc`（链强制 `BondDenom=umc`） |
| 是否可治理修改 | **否**（锁定为链上 Go 常量 + Genesis 双保险校验，不进 params subspace） |

- tokenomics 是**唯一持有 `Minter` 权限**的模块。
- 任何铸造（目前仅 genesis 一次性）都必须使累计 `minted_supply ≤ cap`，否则 panic。
- `minted_supply` 自 genesis 起恒等于 `cap`（1e15 umc），只增不减，持久化于 tokenomics KVStore。

## 2. 五池分配与拨付

| 池 | 占比（基点 bps） | 拨付额（umc） | 地址 | 释放模型 |
|----|----------------|--------------|------|----------|
| 设备激励 device_incentive | 5500（55%） | **5.5e14**（切出推荐预算 8.25e13 后，4.675e14 注入 depin） | `depin` 模块账户 | 按真实任务逐笔拨付，约 11 年（`DefaultVaultDays=4015`）线性摊薄 + 6 月 cliff |
| 质押安全 staking_security | 1500（15%） | **1.5e14** | `staking_security` 模块账户 | 约 12 年（`DripFloorYears=12`）滴灌验证人；手续费净收入与罚没 60% 回流此池 |
| 团队 team | 1200（12%） | **1.2e14** | 3-of-5 多签派生的 vesting 账户 | 1 年 cliff + 3 年线性（总 4 年） |
| 基金会 foundation | 1300（13%） | **1.3e14** | `foundation` 模块账户 + 运营流动地址 | T0 解锁 5e13 → 运营流动地址（其中 5e12 划 DEX 初始流动性）；余 8e13 2 年 cliff + 5 年线性 |
| 早期开发 early_dev | 500（5%） | **5e13** | `early_dev` 模块账户 | 1 年 cliff + 2 年线性 |

- 五池拨付额之和 = `cap`（1e15 umc），会计口径恒等；占比之和恒为 10000 bps（Genesis `Validate` 强校验）。
- **设备激励池**：资金托管于 `depin` 模块账户，创世全额注入；其中推荐返佣预算 `ReferralEcosystemBudget = 8.25e13 umc`（= 55% 的 15%，即 8,250 万 MC）在创世切出并拨付到 `referral` 模块账户，供十代推荐返佣领取；`depin` 实际接收额 = `DepinInitialPoolSlice - ReferralEcosystemBudget`，必须等于 `x/depin/types.DefaultInitialPool`。
- **质押安全池**：独立模块账户，代码不可支出，由滴灌分发逻辑按 `DripRatioBps=500`（年化 5%）向验证人发放；`protocol_treasury` 为第 6 个独立链上地址（创世零余额，仅由企业结算费 60% 与滴灌池耗尽续期注资）。
- **团队池**：3-of-5 多签持有（真实公钥见 `scripts/gen_team_keys` 生成的 `team_pubkeys_gen.go`，与 `team_multisig_vault.txt` 对应）。
  - 释放曲线：`start_time = cliff_time = genesis + 1yr`，`end_time = genesis + 4yr`；第 1 年 cliff（0 释放），第 2–4 年线性释放。
- **基金会池**：`FoundationT0Unlock = 5e13 umc`（5,000 万 MC）创世即时解锁到运营流动地址（满足前期市场流通 1–2 亿体量），其中 `DexInitialLiquidityMC = 5e12 umc`（500 万 MC）划拨 `dex` 模块账户作初始流动性；剩余 8e13 进入 2 年 cliff + 5 年线性 vesting。
- **黑洞地址** `black_hole`：全链唯一销毁去向（模块名经 sha256 确定性派生，无私钥）。销毁来源：Gas 费 7%、DEX 手续费 50%（即成交额 0.15%）、作恶罚没 40%（`SlashBurnRatioBps`）；其余 60% 罚没回流质押安全池。创世即存在，余额即「累计已销毁量」权威口径。

### Genesis 顺序铁律

`tokenomics.InitGenesis` **必须排在 `depin.InitGenesis` 之前**（在 `app.go` 的 `SetOrderInitGenesis` 中位于 `mcchain` 之后、`depin` 之前），以确保设备激励切片拨付、推荐预算切出与 cap 记账覆盖 depin / referral。

## 3. 查询

tokenomics **无 Msg service**，运行期不增发/不销毁。提供 gRPC + CLI 三类查询：

```bash
mcchaind q tokenomics supply        # 总量上限与已发行量
mcchaind q tokenomics allocations   # 五池占比、拨付额、当前余额
mcchaind q tokenomics release        # 团队池释放进度（已释放/未释放/进度）
```

- `release` 释放进度：曲线元数据（start/cliff/end/total_locked）缓存于 tokenomics state，
  进度按 `ctx.BlockTime()` **实时计算**，**查询不改状态**。
- `allocations` 的 `current_balance` 为各池当前 bank 余额（运行期拨付会减少）。

## 4. 不变量（crisis 每轮校验）

| 不变量 | 含义 | 路由 |
|--------|------|------|
| `minted-supply` | `minted_supply ≤ total_supply_cap`（恒真） | `tokenomics/minted-supply` |
| `pool-sum` | 各池 `allocated_amount` 之和 == `minted_supply`（会计口径，恒真） | `tokenomics/pool-sum` |

> 说明：运行期各池会对外拨付，导致实时 bank 余额之和 < `minted_supply`；
> 故不变量以「链上记录的分配记账和 == 已发行」为准，而非实时余额之和。

## 5. 验收

```bash
# 本地编译
go build ./...

# 模块测试
go test ./x/tokenomics/... ./x/depin/... ./x/referral/... ./app/...

# 本地 devnet 一键启动（见 scripts/devnet.sh）
bash scripts/devnet.sh

mcchaind q tokenomics supply       # total_supply_cap=1e15, minted_supply=1e15
mcchaind q tokenomics allocations  # 五池占比 5500/1500/1200/1300/500，余额与分配一致
mcchaind q tokenomics release      # 团队池 vested/remaining/progress 随区块时间推进增长
mcchaind q bank total              # depin 模块账户含 4.675e14 umc（设备激励切片）
mcchaind q bank balances <referral 模块账户>  # 含 8.25e13 umc（推荐预算）
```

> 版本漂移防护：五池比例、释放曲线等常量全部单一来源（`x/tokenomics/types/keys.go`），
> 与 `docs/TOKEN_ALLOCATION.md` / `docs/PROTOCOL_PARAMS.md` 由 `scripts/check_docs_consistency.py` 门禁对齐。
