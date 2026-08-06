# MC 公链 · 上线前全面审核报告（生产级）

- **日期**：2026-08-06
- **版本基准**：`E:\mc-build\mcchain` @ commit `bd174c8`（main 分支）
- **审核视角**：高级产品经理 / 顶级区块链工程师 / 高级技术负责人（三重叠加）
- **审核范围**：链上全部代码（共识、出块、交易、账户资产、节点通信、创世）、运维/后端/密钥/部署、白皮书↔代码一致性
- **方法**：四路并行只读审计（链核心、模块逻辑、运维密钥、一致性）+ 关键 BLOCKER 逐项源码复核
- **总体结论**：**❌ 不可上线（NO-GO）**。存在 11 项上线阻断（BLOCKER），任一都会在出块首日导致**资金被盗、链分叉停摆或经济模型崩溃**。其中两项已直接源码核实为"私钥/助记词泄漏"。

> 说明：本报告所有 BLOCKER 级问题均经主代理逐行源码复核确认；HIGH/MEDIUM 级标注了精确文件:行号，引用自分路审计。修复为后续动作，本报告仅给出结论与建议。

---

## 审核更新 · 二轮（2026-08-06 修复后复核）

> 版本基准：本仓库当前 HEAD（修复集已提交，见 git log）。本轮按"致命缺陷地毯式修复 + 回归测试"执行，
> 覆盖 **DEX 结算/AMM、referral 日上限与多级费率、tokenomics gas 回流与 vesting、全模块 Int64/Uint64 溢出（OVF-1）**。
> 共 **185 项测试全绿**（dex 40 / edgeai 75 / phonenode 23 / liquidstaking 17 / tokenomics 12 / referral 12 / depin 6），`go build ./...` 与 `go vet ./x/... ./internal/...` 通过。

### 本轮已修复并加回归测试（原报告条目 → 现状）

| 原编号 | 修复内容 | 证据（文件 / 测试） |
|---|---|---|
| **M-13** | referral 日上限 `used + bonus.Uint64()` 裸加法可 uint64 回绕绕过上限；bonus 超界时 `Int.Uint64()` panic → 改 `sdkmath.Int` 任意精度比较 + 计数器饱和累加（`saturatingAdd`），负/零/nil bonus 忽略 | `x/referral/keeper/cap.go`；`cap_test.go`（`TestCheckDailyCaps_NoOverflowBypass` / `_HugeBonusDoesNotPanic` / `TestRecordDailyCapUsage_Saturates`） |
| **M-4** | referral 多级费率 `rate==0` 时 `continue` 不推进链 → 下一级费率错配到同一祖先（如 Level2=0 时三代 2% 误发二代祖先）→ 修复为 rate=0 时仍沿链上移一级 | `x/referral/keeper/referral.go`；`m4_regression_test.go`（`TestTrackReward_M4_ZeroMiddleRateDoesNotMisroute`） |
| **L-5** | edgeai `task.Reward * 8500` 裸 uint64 乘法及 `int64(task.Reward)` 回绕 → 全部改 `sdkmath.NewIntFromUint64`（validate/clawback/verifier/create_task 四处），消除对 CreateTask 上界校验的脆弱依赖 | `x/edgeai/keeper/{validate,clawback,verifier,msg_server_create_task}.go` |
| **M-5** | depin resonance 浮点/钳制失效（更早会话已修 sdk.Dec）→ 本轮补 `int(adjusted.Int64())` 溢出饱和（`safemath.ClampToInt`，32 位平台安全） | `x/depin/keeper/resonance.go` |
| **—（新）** | **OVF-1 系列**：全仓 `.Int64()/.Uint64()` 停机级 panic 面清零。tokenomics gas 回流（先 `Uint64()` 再乘 bps）、vesting `NewInt(int64(totalLocked))` 大额回绕负数、liquidstaking 复利累加回绕、depin 金库余额/拨付、phonenode/referral/tokenomics telemetry `float32(Int64())` → 全部改 `sdk.Int` 大数运算或 `internal/safemath` 饱和转换 | `internal/safemath/safemath.go`（新增 `ClampToInt`）；`x/tokenomics/keeper/{gas_rebate,vesting,keeper,grpc_query,genesis}.go`；`x/liquidstaking/keeper/{liquidstaking,hooks}.go`；`x/depin/keeper/{release,resonance,reward,payout}.go`；`x/phonenode/keeper/slash.go`；`x/referral/keeper/{cap,referral}.go`；测试见 `safemath_compat_test.go` / `vesting_test.go`（`TestComputeVested_TotalAboveMaxInt64`） |
| **—（新）** | params 校验器拒绝日上限=0（`daily cap must be > 0`）→ 链上"日上限"恒为硬约束，不存在"0=不限额"配置面 | `cap_test.go`（`TestParamsRejectZeroCap`） |
| **DEPIN-1 关联** | dex 测试 mock bank 升级为严格复式记账 + 模块地址用 `authtypes.NewModuleAddress`（与生产一致），40 项 DEX 测试覆盖结算原子性/幂等/防重放/储备挪用防护/创世 LP 溢出/费率钳制/越界赎回 | `x/dex/keeper/*_test.go`（上会话修复，本会话测试钉死） |

### 尚未修复（保持原报告状态）

- 原 M-13 的另一半：`ResetDailyCaps` BeginBlock 全键扫描随用户数线性增长（P2 性能项，未动）。
- 其余 BLOCKER/HIGH（KEY-1~4、FORK-1、CONS-1、MINT-1、UPG-1、WASM-1、GEN-1、DEPIN-1 主体、ORACLE-1、H-1/H-2/H-3/H-4/H-5/H-6 等）见下方正文，均未在本轮触及。

---

## 一、公链代码审核（区块链工程师视角）

### 1.1 共识 / 出块 / 创世

| 编号 | 级别 | 位置 | 问题 | 影响 | 修复建议 |
|---|---|---|---|---|---|
| **FORK-1** | 🔴阻断 | `x/depin/types/attestation.go:23`、`x/depin/keeper/release.go:195,223` | 共识状态写入使用 `time.Now()`（挂钟时间）而非 `ctx.BlockTime()`。 attestation 时间戳、每日释放键 `ReleaseDaily:YYYY-MM-DD` 均依赖本地时钟。 | 各验证人同一区块执行时刻不同 → 写入不同状态 → AppHash 不一致 → **立即分叉/停链**。跨 UTC 零点边界必触发。 | 全部改为 `ctx.BlockTime()`（日期键用 `ctx.BlockTime().UTC().Format(...)`）。CI 加 `time.Now` 禁用 lint。 |
| **CONS-1** | 🔴阻断 | 全仓无 `consensus_params` 设置；`scripts/make_genesis.py` 无处理；`deploy/init.sh:29` 对 app.toml 执行 `sed s/^max_gas=`（app.toml 无此字段，静默空操作） | `block.max_gas` 沿用 CometBFT 默认 `-1`（不限）。 | 单笔 tx 可占满区块无限 gas → 出块失控/全网停摆；与 GAS-1 叠加为零成本 DoS 完整链路。 | 创世 `consensus_params.block.max_gas` 显式设值（如 `100_000_000`）；删除 init.sh 错误 sed。 |
| **UPG-1** | 🔴阻断 | `app/app.go`（无 `SetStoreLoader` / `ReadUpgradeInfoFromDisk`） | 已注册 `SetUpgradeHandler`（`app.go:1140`），但**缺少升级存储加载器**。 | 未来任一次"新增/删除模块 store"的升级，全网节点在目标高度崩溃且**不可恢复**。 | 补 `ReadUpgradeInfoFromDisk()` + `SetStoreLoader(UpgradeStoreLoader(...))`；多注册升级名路由。 |
| **GEN-1** | 🔴阻断 | `server_setup.sh:22`（`MIN_SELF_DELEGATION="30000000"`） | 自抵押下限设为 30 MC（3e7 umc），而链级 `app/ante.go:17` 要求 `3e10`（30k MC），相差 1000 倍。 | gentx 的 `MsgCreateValidator` 被 `MinSelfDelegationDecorator` 拒绝 → `DeliverGenTxs` 失败 → **InitChain 永不完成，出不了第 1 块**。 | 改为 `"30000000000"`；补一条 InitChain+gentx 端到端集成测试（当前该路径零覆盖）。 |
| **MINT-1** | 🔴阻断 | `app/app.go:217`（`minttypes.ModuleName:{authtypes.Minter}`）；`InitChainer`（`app.go:1103-1111`） | x/mint 仍持 Minter 权限；通胀仅在**创世**清零一次，注释称"每次启动兜底"不成立。 | 治理 `MsgUpdateParams` 把 `InflationMax` 调回 0.13 即可经 bank 铸币，**击穿 10 亿 MC 硬顶**——直接违背白皮书"稀缺性无人能背弃"。 | 从 `maccPerms` 移除 mint 的 Minter，或在 `BeginBlocker` 每块强制归零，或自定义 authority 锁死 mint 参数。 |
| **WASM-1** | 🔴阻断 | `app/wasm_setup_cgo.go:29` | `wasmkeeper.NewKeeper` **未传任何 Option** → wasmd 默认 `CodeUploadAccess=AllowEverybody` + `InstantiateDefaultPermission=Everybody`；创世脚本亦无 wasm params。 | 主网首日任意人可上传并实例化任意合约，攻击面全开（含 `stargate` capability）。 | 起步用 `AllowNobody` 或治理白名单；明确 capability 白名单。 |
| **CONS-2** | 🟠高 | `app/app.go:410` | `consensusparamkeeper.NewKeeper(..., keys[upgradetypes.StoreKey], ...)` 用了 **upgrade 的 store key**；已挂载的 `consensusparamtypes.StoreKey` 空置。 | 与 x/upgrade 共享存储前缀，未来任一方新增前缀即状态污染；改正 key 后旧共识参数全丢（需迁移）。 | 改用正确的 `consensusparamtypes.StoreKey`。 |
| **M-8** | 🟡中 | `edit_configs.py:7`（仅此处设 `timeout_commit="4s"`）；`config.yml`/genesis/`make_genesis.py` 均无 | 4 秒出块未在创世固化，文档自相矛盾（`docs/runbook.md:40` 示例 5s）。 | 运维漏跑脚本 → CometBFT 默认 5s → 所有块计时参数（`BlocksPerDay=21600`、LP 锁仓、争议期、罚没冷却）漂移 25%。 | `make_genesis.py` 写入并校验；白皮书 §A.1 补出块时间一行。 |
| **L-3** | 🔵低 | `app/app.go:1116` 注释 | 注释称下限 `100000000000`（100k MC），实际常量 `3e10`（30k MC）。 | 文档误导。 | 修正注释。 |

### 1.2 交易 / Ante / 账户资产

| 编号 | 级别 | 位置 | 问题 | 影响 | 修复建议 |
|---|---|---|---|---|---|
| **GAS-1** | 🔴阻断 | `cmd/mcchaind/cmd/root.go:364`（`srvCfg.MinGasPrices = "0stake"`）；`edit_configs.py:11`、`deploy/init.sh:22`（默认 `0umc`） | 默认零手续费，且 denom 为遗留 `stake`（非 `umc`）。 | 默认部署下 `fee_collector` 恒空 → gas 销毁/回流现金流为 0（白皮书 7% 销毁/10% 回流成"常量存在、现金流为零"）；免费 spam 与 CONS-1 组成 DoS 链路。 | 改为 `"0.0025umc"`；`init.sh` 在 chain-id 含 `mainnet` 时拒绝 0 费。 |
| **M-2** | 🟡中 | `x/depin/keeper/defense.go:269-292,397-400` | 连续计数器只在**失败**时重置：诚实设备累计 50 次成功后反被永久封禁；攻击者故意触发一次拒绝即可清零绕过。 | 反刷防线可被绕过，且误伤诚实设备。 | 改为基于时间窗的滑动计数与正确重置语义。 |
| **M-5** | 🟡中 | `x/depin/keeper/resonance.go:79,82-108` | `qualityFactor` 在 clamp **之前**计算（clamp 形同虚设）；`multiplier` 用 float64 参与金额计算。 | 共识层使用浮点 → 跨节点浮点不确定性风险；clamp 失效。 | 全部改用 `sdk.Dec`/`sdkmath.Int` 定点运算，clamp 后再算因子。 |
| **M-2b** | 🟡中 | `x/depin/keeper/defense.go:304-339` | `MaxDevicesPerBlock=10` 是**全局**每块阈值，第 11 个设备直接被拒。 | 10 个女巫设备抢先占位即可让全网诚实设备在该块无法获得奖励。 | 改为按设备/来源分组限流或软告警。 |
| **M-13** | 🟡中 | `x/referral/keeper/cap.go:81,91,117-157` | `used + bonus.Uint64()` 裸 uint64 加法（可回绕绕过上限）；`ResetDailyCaps` BeginBlock 对全部用户键无界迭代+删除。 | 上限可被绕过；随用户数线性增长的停链风险。 | 全程 `sdkmath.Int`；改为按日前缀批量前缀删除，避免全键扫描。 |
| **M-4** | 🟡中 | `x/referral/keeper/referral.go:163-167` | `rate==0` 时 `continue` 未上移 `currentInvitee`，导致下一级费率错配到同一推荐人。 | 多级返佣费率错配。 | 修正循环变量推进逻辑。 |
| **L-2** | 🔵低 | `cmd/mcchaind/cmd/root.go:88` | 默认 chain-id `"mcchain"` ≠ 主网 `"mcchain-mainnet-1"`。 | 误部署到错误网络。 | 生产构建固定主网 chain-id。 |

### 1.3 自定义模块逻辑

| 编号 | 级别 | 位置 | 问题 | 影响 | 修复建议 |
|---|---|---|---|---|---|
| **DEPIN-1** | 🔴阻断 | `x/depin/keeper/release.go:246-248`（`getModuleAddress()` 返回 `sdk.AccAddress([]byte("depin"))`） | 模块账户地址取的是裸字节 `"depin"`，**不是** `authtypes.NewModuleAddress("depin")`（正确写法见 `tokenomics/types/keys.go:305`）。 | 读余额得到 **0** → `InitialBalance=0` → `dailyCap=0` → `CheckDailyReleaseCap` 对任何 `amount>0` 拒绝 → **4.675 亿设备池（DePIN 挖矿）永久无法发奖**；金库首次写入即固化为 0，需硬分叉修复。 | 改用 `authtypes.NewModuleAddress(types.ModuleName)`；`GetOrInitReleaseVault` 增加 `balance==0` 时不落盘保护。 |
| **H-1** | 🟠高 | `x/referral/keeper/msg_server.go:35`、`keeper/referral.go:22-89` | `MsgCreateReferral` 签名者是推荐人，`Invitee` 为任意入参，`InviteCode` 全程只存储、从不校验（`ErrInvalidInviteCode` 已定义却未使用）。`ErrInviteeAlreadyReferred` 使关系**永久不可更改**。 | 攻击者可扫描已注册设备地址批量建立"我是你推荐人"，截取全网 10%/5%/2% 三级返佣流（预算 8250 万 MC，独立池、不侵蚀被推荐人收益）。 | `InviteCode` 改为被推荐人签名的凭证，或增加 `MsgAcceptReferral` 双向确认。 |
| **H-2** | 🟠高 | `x/edgeai/keeper/validate.go:172`（BeginBlock 调 `AllResults`）；`:130` panic | 每块把**全部历史 Result** 反序列化进内存；`MaxTasksPerBlock` 只限"结算数"不限"遍历数"；损坏条目直接 `panic`。 | 出块时间随历史线性增长 → 最终停链；可被低价任务加速膨胀。 | 建 `pending` 二级索引只迭代未结算项；结算后删除/归档；BeginBlock 内以事件替代 panic。 |
| **H-3** | 🟠高 | `x/liquidstaking/keeper/liquidstaking.go:25-44` | `ExchangeRate()` 与 `sharesForStake()` 在 `TotalBondedUmc==0` 时返回 `OneDec()`（1:1 虚报）。 | 100% 罚没后 `TotalBonded=0` 而 `TotalShares>0`，新入者存 X 得 X 份额，但其本金被历史无价值份额按比例瓜分；对外汇率接口失真。 | `TotalBonded==0 && TotalShares>0` 进入"池已清零"态，禁止新增质押或强制份额归零重置。（注：`hooks.go:85` 罚没向上取整方向正确，无问题。） |
| **H-4** | 🟠高 | `x/dex/keeper/swap.go:46`（`CalcSwapOutput(..., 0)` 第 4 参 LP 费率传 0） | 配置的 LP 费率（FeeLPBps）在 swap 输出计算中硬编码为 0；储备仍按 `amountIn-nonLPFee` 增长。 | 配置费率不生效，x*y=k 不变量漂移，LP 经济与白皮书 §24 不符。 | 按 `CalcSwapOutput(reserveIn, reserveOut, effIn, FeeLPBps*feeRate/10000)` 折算；补 x*y=k 不变量单测。 |
| **H-5** | 🟠高 | `x/depin/keeper/payout.go:18-28`（`PayoutReward` 直发，不调 `CheckDailyReleaseCap`/`RecordDailyRelease`） | edgeai→depin 拨付绕过每日释放上限，与贡献路径双层约束不一致。 | 线性摊薄机制存在旁路（DEPIN-1 修复后即成为真实的提前抽干路径）。 | 把额度检查下沉进 `PayoutReward`。 |
| **H-6** | 🟠高 | `x/depin/keeper/msg_server_register_device.go:20`、`msg_server_attest_device.go:18-31` | handler 只校验 `msg.Address` 格式，不校验等于签名者 `msg.Creator`；重复地址返回 `ErrDeviceExists` 且无更新/转移/注销路径。 | 攻击者可批量抢注他人地址（含伪造 Model/OS），使真实用户**永久无法入网**。 | `require msg.Creator == msg.Address`；或提供治理级重置。 |
| **M-1** | 🟡中 | `x/dex/keeper/amm.go:35-46` | `shareA`/`shareB` 同时截断为 0 时走 else 分支 `Quo(shareA)` → 除零 panic；初始铸造无 MINIMUM_LIQUIDITY 锁定。 | 小额存入大池即可触发可被脚本化的异常路径。 | 增加 MINIMUM_LIQUIDITY；空储备分支保护。 |
| **M-3** | 🟡中 | `x/depin/keeper/msg_server.go:23-33`、`types/attestation.go:41-52` | `MsgSubmitAttestation` 不校验 `OracleAddress` 是否白名单/等于签名者；历史无上限追加，每次整表读写。 | 存储膨胀 + gas 放大。 | 白名单校验 + 历史按上限滚动。 |
| **M-6** | 🟡中 | `x/edgeai/keeper/validate.go:251-252` | 15% 验证者预留 `SetVerifierReserve` 后无过期回收路径。 | 未被抽检领取的托管款永久沉淀。 | 增加回收/过期结算逻辑。 |
| **M-7** | 🟡中 | `x/phonenode/keeper/node_allowance.go:56`（`SetNodeAllowanceConfig` 仅单测调用） | 节点资本津贴（白皮书承诺"治理可调旋钮"）链上**无 Msg / 无 proposal / 无 CLI / 不在 params**。 | 默认 30 MC/节点/日不可调；10 万节点日耗 300 万 MC，约 155 天见底，远早于宣称的十二年底线。白皮书给出的唯一缓解手段无入口。 | 新增 `MsgUpdateNodeAllowance`（authority=gov）或纳入 phonenode Params，上线前补齐。 |
| **L-4** | 🔵低 | `x/depin/keeper/defense.go:83`、`release.go:54` | `DefenseBlockSub:`/`ReleaseDaily:` 键永不清理，状态单调膨胀。 | 长期存储增长。 | 增加归档/清理。 |
| **L-5** | 🔵低 | `x/edgeai/keeper/clawback.go:34`、`validate.go:226` | `task.Reward * 8500` 裸 uint64 乘法（当前总量下不可达，但应改 `sdkmath.Int`）。 | 潜在风险。 | 改为 `sdkmath.Int`。 |

### 1.4 已复核通过（置信项）

- 创世不变量 `DepinInitialPoolSlice − ReferralEcosystemBudget == DefaultInitialPool` 数学成立，不会误停链。
- 铸币路径收敛：全仓 `MintCoins` 仅 tokenomics（MC）、dex（LP）、liquidstaking（ulmc）；depin/referral/edgeai/phonenode 的 `expected_keepers` 已剔除 MintCoins，无旁路铸 MC。
- 重放/签名防护：标准 `ante.NewAnteHandler`，序列号递增、签名校验、feegrant 完整。
- 团队 1 年 cliff + 3 年线性 vesting 正确；`ComputeVested` 用 `sdk.Int` 防溢出。
- liquidstaking `BeforeValidatorSlashed` 向上取整方向正确（不会高估本金）。
- Evidence keeper 已接线；无 `--unsafe`/CORS 通配/禁用 TLS 类配置。

---

## 二、云端代码库 / 后端 / 运维 / 密钥审核（技术负责人视角）

### 2.1 密钥与机密（最高优先级）

| 编号 | 级别 | 位置 | 问题 | 影响 | 修复建议 |
|---|---|---|---|---|---|
| **KEY-1** | 🔴阻断 | `scripts/seed_cmp/main.go:14`、`scripts/derive_paths/main.go:16` | **明文硬编码 24 词 BIP39 助记词**（"mom merry morning skull ..."）。已用 BIP39+BIP32 独立复算：该助记词在 `m/44'/118'/0'/0/0` 派生公钥 = `team_pubkeys_gen.go:7` 的 **team1**（3-of-5 多签之一）。 | 团队多签 1 把私钥公开 → 门槛从"攻破 3 把"降为"攻破 2 把"；仓库具开源意图，一旦公开即等同私钥公示；历史提交已固化。 | 视 team1 为**永久泄露**：离线机重生成全部 5 把密钥 → 重编 `team_pubkeys_gen.go` → 5 份助记词分 5 人硬件钱包/离线介质分散托管；删除 `derive_paths`/`seed_cmp`/`derive_check` 三个调试程序；对历史做 filter-repo 清洗。 |
| **KEY-2** | 🔴阻断 | `x/tokenomics/types/foundation_addrs_gen.go:13-15`（三项覆写均为 `""`）；派生 `x/tokenomics/types/keys.go:294-301` | 三个覆写变量为空 → `resolveFoundationKey` 回退 `derivedPlaceholder(sha256(单字节种子))`，私钥任何人 5 秒可还原。 | 早期开发 5% + 基金会 13%（合计 **1.8 亿 MC / 18% 总量**）在创世即拨付到公开可取地址；链一出块即被瞬时提走。 | 创世前填入真实冷钱包/多签 bech32 `mcpub` 公钥，使 `FoundationOverridesConfigured()==true`。 |
| **KEY-3** | 🔴阻断 | `x/tokenomics/keeper/genesis.go:43-47` | 检测到占位密钥仅 `ctx.Logger().Error(...)`，**不中断创世**（注释明写"不硬失败"）。 | KEY-2 无任何机器强制拦截，全靠人肉。 | 主网 chain-id（`mcchain-mainnet`）下占位密钥直接 `panic`/返回 error（可用类似 `MC_ORACLE_ALLOW_SOFT` 的环境变量豁免 devnet）。 |
| **KEY-4** | 🟠高 | `cmd/oracle/main.go:238`（`keyring.BackendTest` 明文）+ 助记词走环境变量 | Oracle 私钥明文落盘；助记词经 env 注入，出现在 `docker inspect`/`/proc/<pid>/environ`/systemd 单元/shell history。 | 预言机签名账户私钥泄漏。 | 改 `file`/`os` 后端或 KMS/HSM/Vault，通过挂载 secret 文件注入。 |
| **L-6** | 🔵低 | `.github/` 无 gitleaks/trufflehog | 无密钥扫描 CI → KEY-1 存活至今。 | 同类泄漏将持续漏出。 | 加 gitleaks 到 CI，提交前阻断。 |

### 2.2 预言机 / API / 服务暴露

| 编号 | 级别 | 位置 | 问题 | 影响 | 修复建议 |
|---|---|---|---|---|---|
| **ORACLE-1** | 🔴阻断 | `cmd/oracle/main.go:289-291`（路由）、`:380-432`（handler）、`:436-445`（校验） | `/attest` 无 auth、无限流、无 IP ACL；"证明"仅为 `SHA256(device_id) == attestation_proof`（任何人知道 device_id 即可算出）；`req.Signature` 收下后从不验签，直接透传上链。 | 任意公网用户可为任意设备批量刷 attestation 上链 → 女巫攻击直接击穿"手机挖矿"经济模型；oracle 是有余额的签名账户，无限流 = 资金耗尽 + DoS。 | 上线前加 Bearer/mTLS + 每 IP/每设备限流；`verifyAttestationProof` 换成真实 TEE 出证验签（Android Key Attestation / iOS DeviceCheck）；oracle 只监听内网，前置 Nginx TLS。 |
| **H-11** | 🟠高 | `docs/MAINNET_DEPLOY_RUNBOOK.md:190` | 声称 oracle「`/sign` TLS+ACL ✅ 已实现（Bearer+限流+HTTPS 可选）」。代码（`cmd/oracle/main.go`）**仅 `/health`+`/attest`**，无 `/sign`、无 Bearer、无限流。 | 上线检查表据文档打勾放行 ORACLE-1。 | 修正文档或补齐实现。 |
| **H-2ops** | 🟠高 | `docker-compose.yml:56-61` vs `cmd/oracle/main.go:160,173` | oracle 服务环境变量名不匹配（`ORACLE_LISTEN`/`ORACLE_KEY` vs 实际 `ORACLE_LISTEN_PORT`/`ORACLE_SIGNER_MNEMONIC` 必填）；`:20` 与 `:73` 双方都绑 `9090:9090` → 端口冲突。 | `docker compose up` 直接失败。 | 对齐变量名；prometheus 改 `9091:9090`。 |
| **M-12** | 🟡中 | `cmd/event-subscriber/main.go:109,129`（`:2112` 全网卡）、`event_metrics.json` 权限 `0644` | `/metrics` 无 auth 暴露业务事件量画像；聚合指标单文件全局可读，进程崩溃最多丢 30s/1000 事件。 | 信息泄漏 + 无持久化/备份。 | 加 auth/绑定 localhost；输出到带备份的存储。 |
| **M-14** | 🟡中 | 仓库内无任何 nginx/TLS/防火墙配置；`docker-compose.yml:17-22` 把 26657/1317/9090/26660/26661 全部 `0.0.0.0` 暴露 | RPC/REST 无 CORS、无限流、无反代 TLS。 | 公网直暴露节点管理端口。 | 提供 nginx 反代 + TLS + 限流模板；仅暴露必要端口。 |
| **L-4w** | 🔵低 | `web/index.html:8` 支持"助记词解锁"、`web/config.json:2` 默认 `http://localhost:26657` | 明文助记词进浏览器内存；走明文 HTTP。 | 客户端密钥风险。 | 禁止助记词解锁；强制 HTTPS + Keplr/Leap 集成。 |

### 2.3 部署 / 监控 / 数据库

| 编号 | 级别 | 位置 | 问题 | 影响 | 修复建议 |
|---|---|---|---|---|---|
| **H-4** | 🟠高 | `Dockerfile:13-21` | `debian:bookworm-slim` 无 digest、`WORKDIR /root`、**无 `USER`** → 容器内 root 运行验证人；无 `HEALTHCHECK`。 | 容器逃逸风险 + 无健康探测。 | 加非 root 用户与目录属主；镜像按 sha256 固定；加 healthcheck + 资源上限。 |
| **H-6** | 🟠高 | `monitoring/alert.rules.yml:31-35` 依赖 `up{job="mc-oracle"}`，但 `monitoring/prometheus.yml:13-22` **无该 scrape 任务**（也无 event-subscriber `:2112`）；`alertmanager.yml:18,21` webhook 指向 `http://alertmanager:9093/` 自环占位 | OracleDown / 链停块告警**永不触发**，且 webhook 自环。 | 预言机宕机、链停块无人收到。 | 补 scrape job；配置真实 webhook。 |
| **H-5** | 🟠高 | `docker-compose.yml:89-90,97`（`GF_SECURITY_ADMIN_PASSWORD=mcchain` + 暴露 `3000:3000`） | Grafana 弱口令且公网暴露。 | 监控面板被接管。 | 改 `.env`/secret；监控栈只绑 `127.0.0.1` 走 SSH 隧道。 |
| **M-4ops** | 🟡中 | 全仓无 Postgres/MySQL/Redis 依赖；`docs/LAUNCH_READINESS.md:38` 承认"生产级区块浏览器/索引器未建" | 仅 Cosmos 内嵌 leveldb；无外部索引/备份/快照/迁移计划文档。 | 节点数据无备份，灾难不可恢复。 | 制定快照/备份/迁移 SOP；建索引器。 |
| **M-11** | 🟡中 | `scripts/mainnet-genesis-config.json:6`（`"assert_accounts": []`）、`:13`（`edgeai_arbitrator` 单地址） | 创世账户存在性护栏被完全绕过；仲裁者为单点地址（中心化）。 | 团队/基金会账户漏配也能通过；仲裁无多签。 | `assert_accounts` 填真实地址并 fail-closed；仲裁者改多签。 |
| **M-2d** | 🟡中 | `fix_genesis.py:3`、`edit_configs.py:3`（`r"$HOME/mcchain\testnet\..."` raw 字符串 `$HOME` 不展开 + Windows 反斜杠）；`fix_genesis.py:25` 断言 `initial_pool==1e14` 与 `mainnet-genesis-config.json:4` 的 `4.675e14` 矛盾；`server_setup.sh:83` 同样写死旧值 `1e14` | 脚本已损坏且与主网口径冲突；误用即产出错误创世。 | 创世配置漂移。 | 修复脚本路径与断言，统一为 `4.675e14`。 |
| **M-3ops** | 🟡中 | `server_setup.sh:103`（`--keyring-backend test`，助记词明文写 `validator_key.json`） | 脚本名易被误当生产入口，实为 demo。 | 误用于生产泄露验证人助记词。 | 明确标注"仅 demo"；生产用 `file` 后端 + 离线密钥。 |
| **L-1** | 🔵低 | `monitoring/` Grafana 口令等 | 已含 H-5。 | — | — |
| **L-7** | 🔵低 | `config.yml`（ignite 本地开发 alice/bob+faucet） | 不得带入主网。 | 误带入污染创世。 | 主网流程排除该文件。 |

---

## 三、代码 ↔ 白皮书一致性核对（产品经理视角）

> 基准：`WHITEPAPER_CN.md`（canonical）vs 代码常量与实现。

### 3.1 已核实一致（CONSISTENT，置信项）

| 主题 | 代码证据 |
|---|---|
| 总量 10 亿硬顶 + 双保险 | `tokenomics/types/keys.go:52`；`keeper/keeper.go:56`；`invariant.go:15` |
| 五池 55/15/12/13/5，bps 强校验 | `keys.go:60-64`；`genesis.go:88,108` |
| 4.675 亿 depin + 8250 万 referral 拆分不变量 | `keys.go:71,77`；`depin/types/params.go:23`；`genesis.go:97-99` |
| 唯一铸币入口（umc） | `genesis.go:67`；depin/referral/phonenode/edgeai `expected_keepers` 无 MintCoins；dex/liquidstaking 仅铸 LP/ulmc |
| Gas 7% 销毁 / 10% 回流，已接 BeginBlock | `gas_rebate.go:15,19,52`；`module.go:166` |
| DEX 0.30% 费、50% 入黑洞（成交额 0.15%） | `dex/keeper/fee.go:19-21`（`FeeBurnBps=5000`） |
| 罚没 40% 烧 / 60% 入安全池，零新印 | `keys.go:137-138`；`phonenode/keeper/slash.go` |
| EdgeAI 85/15，15% 可由验证者领取 | `edgeai/types/keys.go:55-56`；`validate.go`；`verifier.go` |
| 企业结算费 1.5%，40/60 拆分 | `keys.go:144-146`；`edgeai/keeper/msg_server_create_task.go:86`；`tokenomics/keeper/keeper.go:119` |
| 已撤销销毁项确未落码 | depin/edgeai types 仅存注释；全仓无 DePIN 5%/推荐 1%/EdgeAI 5%/治理押金 10% 销毁常量 |
| 推荐三级 10%/5%/2%，独立 8250 万池，不侵蚀被推荐人收益 | `referral/types/keys.go:22-24,28-29`；贡献拨付后独立触发推荐 |
| 验证人 3 万 MC 自抵押 + 创世兜底 | `ante.go:17,40`；`app.go:1117-1121` |
| 团队 1 年 cliff + 3 年线性 | `tokenomics/keeper/genesis.go:76-78` |
| 津贴 30 MC/节点/日、资金来自 depin 池 | `phonenode/keeper/node_allowance.go:28,115` |

### 3.2 不一致 / 缺失 / 未实现（MISMATCH / UNIMPLEMENTED）

| 编号 | 级别 | 白皮书主张 | 代码现实 | 差距与建议 |
|---|---|---|---|---|
| **C-B1** | 🔴阻断 | §31「每笔交易以 MC 支付 gas」为结构性需求通道之一；§31 表 `GasBurnRatioBps=700` | 默认 `MinGasPrices="0stake"`（GAS-1），现金流为 0 | 7% 销毁/10% 回流成"常量存在、现金流为零"。修复见 GAS-1。 |
| **C-B2** | 🔴阻断 | §A.1「增发通胀 0（x/mint 强制为零）」；§14「总量上限不进入可治理参数空间」 | x/mint 仍持 Minter，通胀仅创世清零（MINT-1） | 治理可复活通胀击穿 1B 硬顶。修复见 MINT-1。 |
| **C-B3** | 🔴阻断 | §32「津贴是治理参数，不是固定权利」 | 链上无治理入口（M-7） | 白皮书唯一缓解手段无入口。修复见 M-7。 |
| **C-H1** | 🟠高 | §9 表格：基金会 13% 托管于「foundation 模块账户」、释放「链上治理支配」；早期开发 5% 托管于「early_dev 模块账户」 | 两模块账户注册但不持资（`app.go:229-230`）；资金 T0 直打 EOA（`genesis.go:122,131-146`），默认占位可推导私钥（KEY-2） | §9 与代码事实不符（模块账户 vs EOA；"治理支配" vs 无任何门槛）。改写 §9 为真实托管与释放曲线。 |
| **C-H2** | 🟠高 | §14 红线表「安全参数（罚没基点、认证有效期、争议期）→ 可治理提案调整」 | `GasBurnRatioBps`/`FeeBurnBps`/`SlashBurn*`/`EdgeAISubmitterRatioBps`/`EnterpriseSettlementFeeBps` 均为 Go `const`，不在任何 params subspace | 调整任一费率需硬分叉，与"可变"承诺不符。建议迁入 params 或在 §14/附录 A 明确标注"需硬分叉"。 |
| **C-M1** | 🟡中 | §25「推荐奖励从 depin 金库划拨」 | §32/§A.2 称由独立 8250 万预算支付；代码为后者（`genesis.go:103-104` 转 8250 万至 referral 生态账户） | 白皮书内部自相矛盾，审计者按 §25 查 depin 余额会误判。修正 §25 第 589 行。 |
| **C-M2** | 🟡中 | 多处依赖 4 秒出块 | 出块时间未在创世固化（M-8） | 运维漂移 25%。修复见 M-8。 |
| **C-M3** | 🟡中 | 链级紧急暂停 / 提款时间锁（§33.4 已部分披露缺失） | 全仓仅 `dex/settlement_config.go:29` 的 `Halted` 覆盖一条结算路径；无 x/circuit | 建议接入 SDK `x/circuit` 或至少为 depin/edgeai/referral 拨付路径加治理开关。 |
| **C-M4** | 🟡中 | 验证人 Top-N 上限 + 基础设施认证 + 数据触发准入控制 | 全仓无 `max_validators` 显式固化（沿用 SDK 默认 100）；`phonenode` 的 attestation 仅作用于设备节点，**不作用于共识验证人** | 白皮书未明确承诺 Top-N，记 UNIMPLEMENTED/未承诺；但"验证人基础设施认证"完全不存在。建议创世显式钉 `max_validators` 并写入 §A.6。 |

### 3.3 一致性汇总

| 判定 | 条目 |
|---|---|
| ✅ CONSISTENT | 15 项（见 3.1） |
| ⚠️ MISMATCH | C-B1、C-B2、C-B3、C-H1、C-H2、C-M1、C-M2 |
| ⛔ UNIMPLEMENTED | C-B3（治理旋钮）、C-M3（链级熔断）、C-M4（验证人设施认证） |

---

## 四、上线前必办清单（TODO，按优先级）

### P0 — 上线阻断，必须在出块前清零（否则 NO-GO）
- [ ] **KEY-1** 作废当前 team 多签全部 5 把密钥，离线重生成，重编 `team_pubkeys_gen.go`，删除三个调试脚本，历史清洗。
- [ ] **KEY-2 + KEY-3** 填入真实基金会/早期开发 bech32 公钥；主网 chain-id 下占位密钥改硬失败。
- [ ] **DEPIN-1** 修正 `getModuleAddress()` 为 `authtypes.NewModuleAddress("depin")`，加 `balance==0` 保护。
- [ ] **FORK-1** 全仓 `time.Now()`（depin 至少 5 处）改为 `ctx.BlockTime()`。
- [ ] **CONS-1** 创世设 `block.max_gas`；删 `init.sh` 错误 sed。
- [ ] **GAS-1** `root.go:364` 改 `"0.0025umc"`；`init.sh` 主网拒绝 0 费。
- [ ] **MINT-1** 摘除 x/mint 的 Minter 权限（或每块强制归零 + 自定义 authority）。
- [ ] **UPG-1** 补 `SetStoreLoader` + `ReadUpgradeInfoFromDisk`。
- [ ] **WASM-1** CosmWasm 起步 `AllowNobody`/白名单。
- [ ] **ORACLE-1** `/attest` 加鉴权限流 + 真实 TEE 验签；oracle 仅内网。
- [ ] **GEN-1** `server_setup.sh` `MIN_SELF_DELEGATION` 改 `30000000000`；补 InitChain+gentx 集成测试。
- [ ] **C-B2/C-B3/M-7** 治理旋钮（津贴、必要费率）补链上入口或 §14 明确"需硬分叉"。

### P1 — 高危，建议上线前修复
- [ ] **H-1** 推荐关系双向确认 / InviteCode 校验。
- [ ] **H-2** edgeai BeginBlock 改 pending 索引，去全量扫描与 panic。
- [ ] **H-3** liquidstaking 池清零态处理。
- [ ] **H-4** DEX LP 费率落实 + x*y=k 单测。
- [ ] **H-5** edgeai→depin 拨付下沉日释放上限检查。
- [ ] **H-6** 设备注册/认证 `Creator==Address` 绑定 + 治理重置。
- [ ] **H-4ops/H-6/H-5docker** Dockerfile 非 root+HEALTHCHECK；监控补 scrape + 真实 webhook；Grafana 去弱口令。
- [ ] **H-11** 修正 oracle 文档与实现偏差。
- [ ] **KEY-4** oracle 私钥改 KMS/Vault，去 env 明文。
- [ ] **H-2ops** docker-compose oracle 变量名对齐 + 9090 端口冲突。

### P2 — 中危，上线窗口内或首版后跟进
- [ ] M-1/M-2/M-2b 反刷逻辑修正（除零、计数重置语义、分组限流）。
- [ ] M-3/M-5 历史整表读写、浮点金额改 `sdk.Dec`/`sdkmath.Int`。
- [ ] M-4 推荐费率循环推进 bug。
- [ ] M-8 4 秒出块固化进创世 + §A.1 补行。
- [ ] M-11 创世 `assert_accounts` 填真实并 fail-closed；仲裁者改多签。
- [ ] M-12/M-14 服务鉴权/限流/TLS 模板；备份/索引器 SOP。
- [ ] C-M1 白皮书 §25 表述修正。
- [ ] C-M3 评估接入 x/circuit。
- [ ] M-13 状态键归档/清理。

### P3 — 低危 / 卫生
- [ ] L-1~L-7：Grafana 口令、默认 chain-id、注释错误、web 助记词解锁、ignite 配置排除、CI 加 gitleaks、docker-compose 端口冲突。

---

## 五、整体上线就绪度评估

| 维度 | 就绪度 | 说明 |
|---|---|---|
| 代码可编译/可测试 | 🟢 通过 | `go build ./...` 与 `go vet ./x/... ./internal/...` 全绿；**185 项模块测试全 PASS**（含本轮 DEX 40 / edgeai 75 / phonenode 23 / liquidstaking 17 / tokenomics 12 / referral 12 / depin 6）。注：本机杀软拦截 `%TEMP%` 新建测试 exe，需 `go test -c -o <固定路径>.exe` 后直接执行。 |
| 共识安全性 | 🔴 不合格 | FORK-1（确定性破坏）、CONS-1（无 gas 上限）、UPG-1（升级不可恢复）。 |
| 资产安全性 | 🔴 不合格 | KEY-1（团队私钥泄漏）、KEY-2（1.8 亿 MC 占位可推导）、DEPIN-1（4.675 亿池发不出）。 |
| 经济模型一致性 | 🔴 不合格 | C-B1/C-B2/C-B3 使"固定总量/通缩/治理可调"三承诺在主网默认配置下不成立；H-1/H-3/H-4 经济逻辑缺陷。 |
| 预言机/抗女巫 | 🔴 不合格 | ORACLE-1 证明可伪造、无鉴权，DePIN 经济可被女巫击穿。 |
| 运维/监控/部署 | 🟠 弱 | Docker 以 root 运行、监控告警静默、密钥明文、无备份方案。 |
| 文档/治理闭环 | 🟠 弱 | 多处文档与代码漂移（H-11、C-M1、M-8）；治理旋钮缺入口。 |

**最终裁定：❌ NO-GO（不可上线）。**

当前仓库在"功能可编译"层面是健康的，但在**生产安全**层面存在 11 项独立阻断，覆盖"私钥泄漏 → 资金被盗""确定性破坏 → 链分叉""铸币权未锁死 → 通胀击穿""预言机可伪造 → 经济崩溃"四条致命链路。任一都足以在出块首日造成不可逆损失。

**到 GO 的估计路径**：
1. 先完成 P0 全部（密钥轮换 + 11 项代码修复 + 创世仪式 fail-closed 断言）——这是硬前提。
2. 补齐 P1 高危（尤其 H-1/H-2/H-3/H-4/H-6 经济逻辑）。
3. 执行**独立第三方安全审计**（仓库 `LAUNCH_READINESS.md` 已自承"第三方安全审计未做"）。
4.  genesis 仪式 + 多签冷存储 + 监控告警端到端验证。
5. 之后方可进入主网启动。

> 本报告为只读审核结论，未改动任何代码/配置。如需，我可在你确认后按 P0→P1 顺序逐条实施修复并补回归测试。
