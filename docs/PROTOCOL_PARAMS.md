# MC 公链 · 协议参数总表与调整路径

> 适用范围：`mcchain-mainnet-1` 创世参数。本文是**唯一权威口径**，任何与代码或创世文件不一致的表述以本文推导过程为准并须同步修正。
> 出块间隔固定 **4 秒**（`timeout_commit="4s"`），所有「块数 ↔ 时长」换算均以此为准：`1 天 = 21600 块`。
> 最后更新：2026-09-11

---

## 0. 为什么要"显式钉住"

Cosmos SDK 与 CometBFT 的默认参数是按**长块时**（10 秒级以上）的直觉给出的。本链出块 4 秒，两者时间尺度相差一个数量级以上，直接沿用默认值会产生**互不自洽**的参数组合。

一个真实例子：

```
SDK 默认 slashing.params.signed_blocks_window = 100 块
本链 4s 出块 → 100 × 4 = 400 秒

SDK 默认 slashing.params.downtime_jail_duration = 600 秒
```

漏签统计窗口只有 400 秒，**比 jail 时长本身还短**——验证人一次例行重启（超过 400 秒无签名）就会被判 downtime 并 jailed。这不是"参数偏严"，而是参数之间已经不自洽。

还有第二类风险：**默认值漂移**。链上参数在创世即固化，事后只能靠治理提案修正；而 SDK 升级可能静默改动默认值。把隐式默认变成显式声明，是让"链的实际行为"与"文档写的行为"保持一致的最低成本手段。

因此本项目采用如下策略：

| 做法 | 说明 |
|---|---|
| **值取等价默认** | 除非有明确论据，参数值保持与当前 SDK/CometBFT 默认一致，不引入未经论证的经济改动 |
| **显式写入创世** | 由 `scripts/make_genesis.py` 强制写入，缺失即用等价默认补齐 |
| **按出块节奏校准** | 仅对与"块数"强相关的参数重新校准（本文标注 ★） |
| **生成期 fail-fast** | 静态校验 + **跨参数交叉校验**，绝不产出一个启动后才暴露问题的创世 |

---

## 1. staking 参数

| 参数 | 取值 | 说明 |
|---|---|---|
| `bond_denom` | `umc` | 1 MC = 1e6 umc |
| `max_validators` | `100` | 验证人集合上限。SDK 默认等价，**显式声明以明确治理意图** |
| `max_entries` | `7` | 单个验证人的解绑/转委托条目上限，SDK 默认 |
| `historical_entries` | `10000` | 保留的历史条目数，SDK 默认 |
| `unbonding_time` | `1814400s`（21 天） | SDK 默认。解绑期是**安全阀**：罚没窗口内资金不得离场，过短会显著降低作恶成本 |
| `min_commission_rate` | `0` | SDK 默认。佣金上限由 SDK 常量 `MaxCommissionRate = 20%` 与 `MaxCommissionChangeRate = 1%/日` 另行约束 |

### 关于 `unbonding_time` 的取舍

绑定期不影响普通设备节点的日常体验——**设备节点的离线罚没走 `x/phonenode` 的自定义路径，不占质押、不进入 unbonding**（见第 4 节）。只有验证人级别的 30,000 MC 自质押受此约束。

治理可调整区间建议 **14–21 天**：低于 14 天会削弱"作恶成本 > 作恶收益"的论证强度。

---

## 2. slashing 参数（★ 按 4s 出块校准）

| 参数 | 取值 | 与 SDK 默认的关系 |
|---|---|---|
| `signed_blocks_window` | `21600` 块 = **1 天** @4s | ★ **已校准**（SDK 默认 100 块 = 400 秒，不足以覆盖一次例行维护） |
| `min_signed_per_window` | `0.5` | SDK 默认。窗口内须签满 **50%** 才不被判 downtime（注意：是 50%，不是 5%） |
| `downtime_jail_duration` | `600s`（10 分钟） | SDK 默认 |
| `slash_fraction_double_sign` | `0.05`（5%） | SDK 默认 |
| `slash_fraction_downtime` | `0.01`（1%） | SDK 默认 |

### 校准推导

```
准入判据：窗口时长 >= 10 × jail 时长

SDK 默认：  100 块 × 4s = 400s   vs  600s × 10 = 6000s   → 不满足（差 15 倍）
校准后：21600 块 × 4s = 86400s vs  600s × 10 = 6000s   → 满足（14.4 倍余量）
```

窗口取"1 天"的额外好处：运维口径与人类作息对齐——**一次日间维护不会触发惩罚**，而长时间失联（超过 12 小时漏签一半以上）仍会被准确捕获。

该判据已写入 `scripts/make_genesis.py` 的 `check_node_params()`，生成期强制校验。

---

## 3. consensus / evidence 参数

| 参数 | 取值 | 说明 |
|---|---|---|
| `block.max_gas` | `100000000`（1e8） | 与链上 `app.DefaultBlockMaxGas` 同值。**必须显式覆盖** |
| `block.max_bytes` | `22020096` | 21 MiB，CometBFT 默认 |
| `evidence.max_age_num_blocks` | `100000` | 约 4.6 天 @4s |
| `evidence.max_age_duration` | `172800000000000`（48h） | CometBFT 默认 |
| `evidence.max_bytes` | `1048576` | 1 MiB，CometBFT 默认 |

### `max_gas` 必须显式覆盖

CometBFT 生成的默认值是字符串 `"-1"`（不限）。若用真值判断：

```python
if not blk.get("max_gas"):      # "-1" 是非空字符串 → 真值 → 条件恒不成立
    blk["max_gas"] = "100000000"
```

用真值判断时，创世里就会始终留着"不限 gas"。虽然 app 层有 `DefaultBlockMaxGas` 兜底钳制，实际行为正确，但**创世文件与注释口径不一致本身就是隐患**——外部审计按创世读参数会得出"gas 无上限"的错误结论。现改为强制写入，并校验取值落在 `(0, 1e8]`。

---

## 4. 两类 slash 必须区分

链上存在两条互不相同的惩罚路径，参数与适用对象都不同：

| | 共识层 slashing | 应用层 phonenode slash |
|---|---|---|
| 模块 | `x/slashing` | `x/phonenode` |
| 触发 | 双签、漏签超窗口 | 设备超宽限无心跳、贡献作弊、伪造 attestation |
| 对象 | **验证人**（共识参与者） | **设备节点**（移动端挖矿） |
| 参数 | 本文第 2 节 | `x/phonenode` 模块参数（`OfflineSlashBps` 等） |
| 罚没去向 | SDK 路径 | **40% 黑洞销毁 / 60% 回流质押安全池**（白皮书 §24.4） |
| 是否受 unbonding 约束 | 是 | 否（非验证人节点不罚币，仅吊销 attestation + 记录 + 冷却） |

**注意**：`x/phonenode` 的 40/60 分流是以 `slashAndRoute` 自定义实现的，不使用 SDK `slashing.Slash`（v0.47 的 `Slash` 会把被罚金额 100% 烧毁、无法拆分去向，`BurnSlashTokens` 参数要到 v0.50+ 才有）。两条路径的惩罚金额不会重复计算。

---

## 5. 验证人分阶段扩容路径

`max_validators = 100` 是**上限**，实际席位由质押竞争决定（前 N 名按 power 排序）。因此扩容不需要改参数，只需要生态自然增长：

| 阶段 | 预期实际席位 | 说明 |
|---|---|---|
| 启动期 | 7–21 | 上限 100 给足空间，无需为启动刻意压低上限 |
| 成长期 | 21–50 | 质押竞争自然填充；`unbonding_time` 与 `min_commission_rate` 可经治理微调 |
| 成熟期 | 50–100 | 若需突破 100，提交 `max_validators` 参数变更提案 |

**监控要求**：扩容期间必须跟踪**投票权集中度**（top-N 占比）。验证人数量上升不等于去中心化上升——若 top-5 持有多数投票权，仍是实质集中。该指标已纳入告警规则（见 `docs/ALERTS.md`）。

---

## 6. 治理调整流程

所有参数均可通过链上治理修改。**注意：治理主体为链上治理模块账户，提案是唯一可达通道**（`x/mcchain` 的渐进治理移交已设为默认启用，48 小时时间锁）。

标准流程：

1. **提案**：使用 `docs/governance-proposals/` 下的模板（参数变更须填写"当前值 / 目标值 / 论据 / 影响面 / 回滚方案"）
2. **押金**：最低 `10 MC`
3. **投票**：48 小时；`quorum 33.4%` / `threshold 50%` / `veto 33.4%`
4. **生效**：`x/params` 通过 `ParamChangeProposal` 生效（或治理消息直调，视模块而定）
5. **同步本文档**：参数变更后**必须**同步更新本文与 `scripts/*-genesis-config.json`，否则"创世口径"与"运行口径"会分叉

---

## 7. 校验与回归测试

生成期校验（`scripts/make_genesis.py`）：

- `max_validators` ∈ [1, 1000]
- `unbonding_time` 可解析且 ≥ 1 天
- `signed_blocks_window` > 0，`min_signed_per_window` ∈ (0, 1]
- slash 比例 ∈ [0, 1]
- **交叉校验**：`signed_blocks_window × 4s ≥ 10 × downtime_jail_duration`
- `block.max_gas` ∈ (0, 1e8]

回归测试（可进 CI）：

```bash
python scripts/test_genesis_params.py
```

覆盖：参数被显式钉入、`max_gas` 强制覆盖、config 覆盖生效、不自洽的 slashing 参数被 fail-fast、非法 gas 上限被拒绝。
