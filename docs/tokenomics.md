# MobileChain 经济模型（B1 · 链上基础）

> 配套设计：`docs/system_design_b1_tokenomics.md`、类图/时序图 `docs/b1_tokenomics_*.mermaid`。
> 本文件描述链上已落地的 tokenomics 模块（发行与分配总账），供社区与 SDK 引用。

## 1. 总量上限（R1 · 总量固化）

| 项 | 值 |
|----|----|
| 总供应量上限 `TotalSupplyCap` | **1e15 umc = 1e9 MC（10 亿 MC）** |
| 单位 | `1 MC = 1e6 umc`，链上金额一律以 `umc` 整数表达 |
| denom | `umc`（链强制 `BondDenom=umc`） |
| 是否可治理修改 | **否**（Q8：锁定为链上 Go 常量 + Genesis 双保险校验，不进 params subspace） |

- tokenomics 是**唯一持有 `Minter` 权限**的模块（Q7）。
- 任何铸造（目前仅 genesis 一次性）都必须使累计 `minted_supply ≤ cap`，否则 panic。
- `minted_supply` 自 genesis 起恒等于 `cap`（1e15 umc），只增不减，持久化于 tokenomics KVStore。

## 2. 分配占比与拨付（R2）

| 池 | 占比（基点 bps） | 拨付额（umc） | 地址 | 释放模型 |
|----|----------------|--------------|------|----------|
| 团队 team | 1500（15%） | **1.5e14** | 3-of-5 多签派生的 vesting 账户 | 1 年 cliff + 3 年线性（总 4 年） |
| 社区 community | 3500（35%） | **3.5e14** | `community` 模块账户 | 立即可用（无 vesting） |
| 生态 ecosystem | 5000（50%） | **5e14** | `ecosystem` 模块账户 | 立即可用；其中 1e14 umc 在 genesis 转给 depin |

- 三池拨付额之和 = `cap`（1e15 umc），会计口径恒等。
- **团队池**：由 3-of-5 多签持有（占位 secp256k1 测试公钥，**主网 genesis export 前必须替换为真实团队多签公钥**，常量位于 `x/tokenomics/types/keys.go`）。
  - 释放曲线：`start_time = cliff_time = genesis + 1yr`，`end_time = genesis + 4yr`；第 1 年 cliff（0 释放），第 2–4 年线性释放。
- **社区池**：独立 `community` 模块账户（Q5），立即可用。
- **生态池**：`ecosystem` 模块账户，持有 5e14 umc；其中 **1e14 umc（InitialPool）在 genesis 由生态池转给 `depin` 模块账户**（Q4/Q7）。
  - 因此 `depin` 不再自铸（其 `maccPerms` 已移除 `Minter`，仅 `{Burner, Staking}`）。

### Genesis 顺序铁律

`tokenomics.InitGenesis` **必须排在 `depin.InitGenesis` 之前**（在 `app.go` 的 `SetOrderInitGenesis` 中位于 `mcchain` 之后、`depin` 之前），以确保生态切片拨付与 cap 记账覆盖 depin。

## 3. 查询（R3 · 透明只读）

tokenomics **无 Msg service**，运行期不增发/不销毁。提供 gRPC + CLI 三类查询：

```bash
mcchaind q tokenomics supply        # 总量上限与已发行量
mcchaind q tokenomics allocations   # 三大池占比、拨付额、当前余额
mcchaind q tokenomics release        # 团队池释放进度（已释放/未释放/进度）
```

- `release` 释放进度：曲线元数据（start/cliff/end/total_locked）缓存于 tokenomics state，
  进度按 `ctx.BlockTime()` **实时计算**，**查询不改状态**（Q9）。
- `allocations` 的 `current_balance` 为各池当前 bank 余额（运行期拨付会减少）。

## 4. 不变量（crisis 每轮校验）

| 不变量 | 含义 | 路由 |
|--------|------|------|
| `minted-supply` | `minted_supply ≤ total_supply_cap`（R1，恒真） | `tokenomics/minted-supply` |
| `pool-sum` | 各池 `allocated_amount` 之和 == `minted_supply`（会计口径，恒真） | `tokenomics/pool-sum` |

> 说明（共享知识 #2）：运行期社区/生态会从各自池对外拨付，导致实时 bank 余额之和 < `minted_supply`；
> 故不变量以「链上记录的分配记账和 == 已发行」为准，而非实时余额之和。

## 5. 验收（本机）

```bash
cd $HOME/mcchain
ignite chain build
go test ./x/tokenomics/... ./x/depin/... ./app/...

ignite chain init
ignite chain start

mcchaind q tokenomics supply       # total_supply_cap=1e15, minted_supply=1e15
mcchaind q tokenomics allocations  # 三池占比 1500/3500/5000，余额与分配一致
mcchaind q tokenomics release      # 团队池 vested/remaining/progress 随区块时间推进增长
mcchaind q bank total              # depin 模块账户含 1e14 umc（来自生态切片）
```
