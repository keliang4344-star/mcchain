package keeper

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"mcchain/internal/daywindow"
	"mcchain/internal/safemath"
	"mcchain/x/referral/types"
)

// ============================================================================
// 推荐返佣预算的配速释放
// ============================================================================
//
// 背景：
//
//  1. 返佣预算 ReferralEcosystemBudget = 82.5M MC，是从 55% 设备激励池里一次性
//     切出来的固定额度（x/tokenomics/types/keys.go）。它与 DePIN 挖矿池
//     467.5M MC 共用同一个池子——返佣不是额外预算，是设备激励的切出。
//
//  2. 网络日上限被校准成「11 年匀速花完」：82,500,000 / 20,600 ≈ 4,005 天，
//     刻意与 depin 金库的 4015 天释放窗口对齐。作为「防抽干」设计它是合理的，
//     但作为「促增长」设计方向相反：冷启动需要前置投放，而日上限是常数。
//
//  3. 十代费率合计 23.5%（10/5/3/2/1/0.5×5），而预算承载比只有
//     82.5M / 467.5M = 17.65%。23.5% > 17.65% 意味着在「大多数用户都有
//     10 代链」的稳态下，返佣消耗速率必然快于预算承载能力：
//     预算在累计 depin 发放 82.5M/0.235 ≈ 351M MC 时耗尽，此时挖矿池还剩 25%。
//     即推荐体系会比挖矿体系提前四分之一停摆。
//
// 本文件用「余额驱动的配速上限」同时解决上述三点，且**不改变任何总量口径**：
//
//	有效日上限 = min( 治理参数日上限 × 阶段倍数, 剩余预算 / 剩余天数 )
//
//   - 剩余预算直接读 ecosystem 模块账户的实时余额，不另设账本，因此不可能与
//     实际资金漂移；
//   - 剩余天数按 ReleaseSchedule.TotalDays 递减，最小值取 1；
//   - 配速项保证预算**必然撑满整个释放窗口**（花得快 → 剩余预算变小 → 次日上限
//     自动收紧），因此任何前置投放都不会超发；
//   - 阶段倍数用于冷启动前置投放：前 Phase1Days 天允许更高的日上限，但因为
//     配速项的钳制，总支出仍然不可能超过预算总额，只是把「何时花」前移。
//
// 三条性质叠加后：
//   · 总支出 ≤ 82.5M（硬上限不变，白皮书口径不变）；
//   · 预算覆盖期 = TotalDays（消除了「提前 25% 停摆」）；
//   · 冷启动期可以前置投放（消除了「花不出去」，且不需要改白皮书）。

// ReleaseSchedule 推荐返佣预算的释放节奏配置（JSON 持久化于模块 KVStore）。
//
// 与 x/dex 的 SettlementConfig 采用同一模式：KVStore + 模块默认值，避免为
// 一个运营参数引入 proto 字段变更。
type ReleaseSchedule struct {
	// StartTimeUnix 释放窗口起点（区块时间 Unix 秒）。创世时写入首个区块时间；
	// 为 0 表示尚未初始化，此时配速不生效（退化为纯参数上限，与历史行为一致）。
	StartTimeUnix int64 `json:"start_time_unix"`

	// TotalDays 释放窗口总天数，默认 4015（≈11 年，与 depin 金库同口径）。
	TotalDays uint32 `json:"total_days"`

	// Phase1Days 冷启动前置投放天数，默认 365（首年）。0 表示不做阶段加速。
	Phase1Days uint32 `json:"phase1_days"`

	// Phase1MultiplierBps 冷启动期日上限倍数（bps），默认 200 = 2 倍。
	// 100 表示不加速（等于纯配速）。
	Phase1MultiplierBps uint32 `json:"phase1_multiplier_bps"`
}

// DefaultReleaseSchedule 返回推荐预算释放节奏的默认配置。
//
// 默认值的选择依据：
//   - TotalDays = 4015：与 x/depin 的 DefaultVaultDays 完全一致，两个同源预算
//     用同一个窗口，避免再次出现「各模块各写一个周期」的口径分裂；
//   - Phase1Days = 365 / Multiplier = 200：冷启动首年允许 2 倍日上限。以
//     82.5M/4015 ≈ 20,548 MC/日 为基线，首年上限约 41,096 MC/日，首年最多投出
//     约 15M（占预算 18%，基线为 9%）——前置投放一倍，总额与窗口不变。
func DefaultReleaseSchedule() ReleaseSchedule {
	return ReleaseSchedule{
		StartTimeUnix:       0,
		TotalDays:           types.DefaultReleaseTotalDays,
		Phase1Days:          types.DefaultReleasePhase1Days,
		Phase1MultiplierBps: types.DefaultReleasePhase1MultiplierBps,
	}
}

// normalize 把零值/越界字段回填为默认值，保证即使 KVStore 里存了残缺 JSON
// （升级、人工修链、旧版本写入）也不会让配速逻辑退化成「无上限」。
func (s ReleaseSchedule) normalize() ReleaseSchedule {
	d := DefaultReleaseSchedule()
	if s.TotalDays == 0 {
		s.TotalDays = d.TotalDays
	}
	if s.Phase1MultiplierBps == 0 {
		s.Phase1MultiplierBps = d.Phase1MultiplierBps
	}
	if s.Phase1MultiplierBps < 100 {
		// 低于 1 倍没有经济含义（会把上限压到治理参数之下），按不加速处理。
		s.Phase1MultiplierBps = 100
	}
	if s.Phase1Days > s.TotalDays {
		s.Phase1Days = s.TotalDays
	}
	return s
}

// GetReleaseSchedule 读取释放节奏配置；未设置或解析失败一律返回确定性默认值。
//
// 失败方向必须是「保守」而非「放开」：解析失败时返回默认值，其配速项仍然生效，
// 不会退化成无上限。
func (k Keeper) GetReleaseSchedule(ctx sdk.Context) ReleaseSchedule {
	bz := ctx.KVStore(k.storeKey).Get([]byte(types.ReleaseScheduleKey))
	if bz == nil {
		return DefaultReleaseSchedule()
	}
	var s ReleaseSchedule
	if err := json.Unmarshal(bz, &s); err != nil {
		return DefaultReleaseSchedule()
	}
	return s.normalize()
}

// SetReleaseSchedule 持久化释放节奏配置。
func (k Keeper) SetReleaseSchedule(ctx sdk.Context, s ReleaseSchedule) {
	s = s.normalize()
	bz, err := json.Marshal(s)
	if err != nil {
		panic(fmt.Sprintf("referral: marshal release schedule: %v", err))
	}
	ctx.KVStore(k.storeKey).Set([]byte(types.ReleaseScheduleKey), bz)
}

// EnsureReleaseScheduleStart 在创世（或首次 BeginBlock）把释放窗口起点钉在链上
// 首个区块时间。幂等：已设置过就不再改写，避免重启后窗口被重置。
func (k Keeper) EnsureReleaseScheduleStart(ctx sdk.Context) {
	s := k.GetReleaseSchedule(ctx)
	if s.StartTimeUnix > 0 {
		return
	}
	if ctx.BlockTime().Unix() <= 0 {
		return // 区块时间尚未就绪，等下一个高度
	}
	d := DefaultReleaseSchedule()
	d.StartTimeUnix = ctx.BlockTime().Unix()
	// 保留运维在创世写入过的天数 / 阶段参数，只补起点。
	d.TotalDays = s.TotalDays
	d.Phase1Days = s.Phase1Days
	d.Phase1MultiplierBps = s.Phase1MultiplierBps
	k.SetReleaseSchedule(ctx, d)
}

// ---------------------------------------------------------------------------
// 预算余额
// ---------------------------------------------------------------------------

// EcosystemBalance 返回推荐返佣生态账户的实时 umc 余额（= 剩余预算）。
//
// 直接读实时余额而不是维护一个「已释放」累加器：模块账户余额本身就是资金的
// 唯一真相，任何自建账本都可能与它漂移，而漂移的代价是超发。
func (k Keeper) EcosystemBalance(ctx sdk.Context) sdkmath.Int {
	addr := authtypes.NewModuleAddress(types.EcosystemModuleAccount)
	coin := k.bankKeeper.GetBalance(ctx, addr, types.BaseDenom)
	if coin.IsNil() || !coin.Amount.IsPositive() {
		return sdkmath.ZeroInt()
	}
	return coin.Amount
}

// ---------------------------------------------------------------------------
// 有效日上限
// ---------------------------------------------------------------------------

// CommittedLiabilities 返回「已经承诺但尚未从生态账户支出」的返佣总额（umc）。
//
// 这类钱仍在生态账户里（余额没变），但去向已经定死 —— 要么是
// 待领取的 pending rewards，要么是延后账本里排队的欠款。配速基数必须把它们
// 排除，否则会把「已承诺的钱」当成「还能再花的钱」。
func (k Keeper) CommittedLiabilities(ctx sdk.Context) sdkmath.Int {
	return k.getPendingRewardTotal(ctx).Add(k.getDeferredTotal(ctx))
}

// AvailableBudget 返回真正可以继续承诺的返佣预算（umc）。
//
//	可用预算 = max( 生态账户余额 − 已承诺负债 , 0 )
func (k Keeper) AvailableBudget(ctx sdk.Context) sdkmath.Int {
	avail := k.EcosystemBalance(ctx).Sub(k.CommittedLiabilities(ctx))
	if !avail.IsPositive() {
		return sdkmath.ZeroInt()
	}
	return avail
}

// EffectiveNetworkCap 返回当日_实际生效_的全网返佣上限（umc）。
//
//	配速项     = 可用预算（余额 − 已承诺负债）/ 剩余天数
//	基准额度   = min( 治理参数上限 , 配速项 )
//	有效上限   = 基准额度 × 阶段倍数 / 100
//
// 返回的第二个值为阶段倍数（百分数，100 = 1 倍），仅用于可观测性（查询/事件）。
//
// ⚠️ 单位口径（曾出错，务必对齐）：Phase1MultiplierBps 的取值语义是**百分数**，
// 100 = 不加速（1 倍）、200 = 2 倍 —— 这一点由 DefaultReleaseSchedule 的注释与
// normalize() 的下界（<100 归一到 100）共同锁定。此处换算必须除以 100。
// 早期实现按「万分比」除以 10000，等于把治理参数上限压到 1%（100 → 0.01 倍），
// 日上限被静默削掉 100 倍：预算看起来「还有」，实际几乎发不出去。
// cap_test 的日上限用例正是通过 effective_cap=100 ≠ param_cap=10000 抓到了它。
//
// 倍数只乘在「治理参数上限」上是不够的，
// 而配速项通常比它更低，于是 min() 总取配速项，倍数对结果没有任何影响 ——
// 文档承诺的「首年日上限约 41,096 MC、前置投放约 15M」从未发生。
// 现在倍数乘在**基准额度**（= min(参数上限, 配速项)）上，语义回到设计本意：
// 冷启动期整体放大日投放量，同时仍被「可用预算 / 剩余天数」这一配速项约束，
// 因此总支出不可能超过预算，只是把「何时花」前移。
func (k Keeper) EffectiveNetworkCap(ctx sdk.Context) (cap sdkmath.Int, phaseMultiplierPct uint32) {
	params := k.GetParams(ctx)
	sched := k.GetReleaseSchedule(ctx)

	// 治理参数上限（绝对值，作为基准额度的天花板）
	base := sdkmath.NewIntFromUint64(params.DailyNetworkCap)

	// 阶段倍数：仅当窗口已初始化且处于冷启动期内才 > 100
	multPct := uint32(100)
	if sched.StartTimeUnix > 0 && sched.Phase1Days > 0 && sched.Phase1MultiplierBps > 100 {
		elapsed := elapsedDays(ctx, sched)
		if elapsed < uint64(sched.Phase1Days) {
			multPct = sched.Phase1MultiplierBps
		}
	}

	// 配速项：可用预算 / 剩余天数
	//
	// 基数是「可用预算」而不是「账户余额」—— 已承诺的 pending 与
	// 延后账本欠款必须先从余额里扣掉，否则用户长期不领取时，配速项会高估
	// 可承诺额度，累计承诺额可能超过预算总额。
	available := k.AvailableBudget(ctx)
	var daysLeft uint64 = 1
	if sched.StartTimeUnix > 0 {
		elapsed := elapsedDays(ctx, sched)
		total := uint64(sched.TotalDays)
		if elapsed < total {
			daysLeft = total - elapsed
		} else {
			// 窗口已到期末：允许把剩余预算在当日释放完，不锁死资金。
			daysLeft = 1
		}
	}
	paced := available.Quo(sdkmath.NewIntFromUint64(daysLeft))
	if paced.IsNegative() {
		paced = sdkmath.ZeroInt()
	}

	// 基准额度 = min(参数上限, 配速项)
	allowance := base
	if paced.LT(allowance) {
		allowance = paced
	}

	// 阶段倍数乘在基准额度上（百分数换算：100 → 1 倍）。
	effective := allowance.Mul(sdkmath.NewIntFromUint64(uint64(multPct))).Quo(sdkmath.NewInt(100))

	// 硬边界：无论倍数多大，当日上限都不得超过可用预算（配速项本身已隐含
	// 这一点，这里显式钳一次，避免未来调整公式时把不变量弄丢）。
	if effective.GT(available) {
		effective = available
	}
	if effective.IsNegative() {
		effective = sdkmath.ZeroInt()
	}
	return effective, multPct
}

// elapsedDays 返回释放窗口已过去的天数（按区块时间，UTC 日界）。
func elapsedDays(ctx sdk.Context, sched ReleaseSchedule) uint64 {
	if sched.StartTimeUnix <= 0 {
		return 0
	}
	now := ctx.BlockTime().Unix()
	if now <= sched.StartTimeUnix {
		return 0
	}
	return uint64((now - sched.StartTimeUnix) / daywindow.SecondsPerDay)
}

// DaysRemaining 返回释放窗口剩余天数（窗口未初始化时返回 TotalDays）。
func (k Keeper) DaysRemaining(ctx sdk.Context) uint64 {
	sched := k.GetReleaseSchedule(ctx)
	if sched.StartTimeUnix <= 0 {
		return uint64(sched.TotalDays)
	}
	elapsed := elapsedDays(ctx, sched)
	total := uint64(sched.TotalDays)
	if elapsed >= total {
		return 0
	}
	return total - elapsed
}

// ---------------------------------------------------------------------------
// 延后账本
// ---------------------------------------------------------------------------

// deferredKey 构造延后账本键：前缀 + 8 字节大端 dayIndex + inviter。
//
// 把 dayIndex 放在 inviter 之前，是为了让迭代天然按「日」升序（大端编码的
// 整数序即字节序），从而：
//   - 补发严格 FIFO，先欠的先还；
//   - BeginBlock 只用扫描前缀开头即可，扫到 day >= today 就能停，无需全表遍历。
func deferredKey(dayIdx uint64, inviter string) []byte {
	k := make([]byte, 0, len(types.DeferredRewardPrefix)+8+len(inviter))
	k = append(k, []byte(types.DeferredRewardPrefix)...)
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], dayIdx)
	k = append(k, buf[:]...)
	k = append(k, []byte(inviter)...)
	return k
}

// DeferReward 把一笔因触顶、预算耗尽或任何「暂时发不出」的原因未能入账的返佣
// 记入延后账本。对外暴露的理由：顺延是模块的公共语义（推广者该拿的钱不会消失），
// 其他模块与上层集成方需要能复用同一条入账路径，而不是各写一套账本。
func (k Keeper) DeferReward(ctx sdk.Context, inviter string, amount sdkmath.Int) {
	k.deferReward(ctx, inviter, amount)
}

// deferReward 把一笔因触顶而未能入账的返佣记入延后账本（累加到当日桶）。
//
// 与「静默丢弃」的区别是这条设计的全部意义所在：推广者该拿的钱不会消失，
// 只是顺延到有额度的日子发放；同时链上会发出可观测事件，推广者能自己查到
// 「为什么今天没到账」。
func (k Keeper) deferReward(ctx sdk.Context, inviter string, amount sdkmath.Int) {
	if amount.IsNil() || !amount.IsPositive() {
		return
	}
	store := ctx.KVStore(k.storeKey)
	key := deferredKey(daywindow.DayIndex(ctx), inviter)

	total := sdkmath.ZeroInt()
	if bz := store.Get(key); bz != nil {
		var entry deferredEntry
		if err := json.Unmarshal(bz, &entry); err == nil {
			if v, ok := sdkmath.NewIntFromString(entry.Amount); ok {
				total = v
			}
		}
	}
	total = total.Add(amount)

	bz, err := json.Marshal(deferredEntry{Amount: total.String()})
	if err != nil {
		// 序列化失败只会出现在内存耗尽等极端情况；此处不 panic（共识路径），
		// 但也不能静默——记日志并放弃本次延后（等同旧行为），由事件暴露。
		k.Logger(ctx).Error("referral: marshal deferred entry failed", "err", err, "inviter", inviter)
		return
	}
	store.Set(key, bz)

	// 全局延后余额（仅用于查询与告警，不参与共识判定）
	k.addDeferredTotal(ctx, amount)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeReferralRewardDeferred,
		sdk.NewAttribute(types.AttrInviter, inviter),
		sdk.NewAttribute(types.AttrAmount, amount.String()),
		sdk.NewAttribute(types.AttrReason, types.DeferReasonDailyCap),
	))
}

// deferredEntry 延后账本的单条记录。
type deferredEntry struct {
	Amount string `json:"amount"`
}

// getDeferredTotal / addDeferredTotal 维护延后余额的全局计数（可观测用）。
func (k Keeper) getDeferredTotal(ctx sdk.Context) sdkmath.Int {
	bz := ctx.KVStore(k.storeKey).Get([]byte(types.DeferredRewardTotalKey))
	if bz == nil {
		return sdkmath.ZeroInt()
	}
	if v, ok := sdkmath.NewIntFromString(string(bz)); ok {
		return v
	}
	return sdkmath.ZeroInt()
}

func (k Keeper) addDeferredTotal(ctx sdk.Context, delta sdkmath.Int) {
	k.setDeferredTotal(ctx, k.getDeferredTotal(ctx).Add(delta))
}

func (k Keeper) setDeferredTotal(ctx sdk.Context, v sdkmath.Int) {
	if v.IsNil() || v.IsNegative() {
		v = sdkmath.ZeroInt()
	}
	ctx.KVStore(k.storeKey).Set([]byte(types.DeferredRewardTotalKey), []byte(v.String()))
}

// DeferredTotal 暴露延后余额（供查询与告警）。
func (k Keeper) DeferredTotal(ctx sdk.Context) sdkmath.Int {
	return k.getDeferredTotal(ctx)
}

// DrainDeferred 按固定预算把过期的延后条目补发入 pending。
//
// 每块处理至多 types.DeferredDrainBudget 条，成本恒定。设计要点：
//   - 只处理 dayIndex < today 的条目（严格 FIFO），遇到 day >= today 立即停止；
//   - 补发时重新走一遍日上限检查——额度是按当天的，不能沿用被拒那天的结论；
//   - 仍被拒则改记到今天的桶里（重新排队），而不是丢弃；
//   - 预算耗尽（effective cap 为 0）时直接返回，条目原样保留，等预算恢复。
func (k Keeper) DrainDeferred(ctx sdk.Context) {
	store := ctx.KVStore(k.storeKey)
	prefix := []byte(types.DeferredRewardPrefix)

	today := daywindow.DayIndex(ctx)

	// 预算耗尽：不做任何事，保留用户权益原样（不删除、不清零）。
	// 注意这里用「是否还有余额」判断，而不是当天的上限是否为 0——上限可能因为
	// 当日额度用尽而为 0，那是正常的顺延，不该停止补发。
	if !k.EcosystemBalance(ctx).IsPositive() {
		return
	}

	it := sdk.KVStorePrefixIterator(store, prefix)
	type pendingMove struct {
		key     []byte
		inviter string
		amount  sdkmath.Int
	}
	var done []pendingMove
	var invalidKeys [][]byte

	processed := 0
	for ; it.Valid(); it.Next() {
		if processed >= types.DeferredDrainBudget {
			break
		}
		key := it.Key()
		// 非法键一次性删除并告警，不再「跳过但永久滞留」
		// —— 滞留的毒键每轮都白耗 Drain 预算（对比 collectStaleKeys 同步修复）。
		if len(key) < len(prefix)+8 {
			invalidKeys = append(invalidKeys, append([]byte(nil), key...))
			processed++
			continue
		}
		dayIdx := binary.BigEndian.Uint64(key[len(prefix) : len(prefix)+8])
		if dayIdx >= today {
			break // 前缀按 day 升序，后面的都是今天或未来的，无需再看
		}
		inviter := string(key[len(prefix)+8:])

		var entry deferredEntry
		if err := json.Unmarshal(it.Value(), &entry); err != nil {
			invalidKeys = append(invalidKeys, append([]byte(nil), key...))
			processed++
			continue
		}
		amount, ok := sdkmath.NewIntFromString(entry.Amount)
		if !ok || !amount.IsPositive() {
			invalidKeys = append(invalidKeys, append([]byte(nil), key...))
			processed++
			continue
		}

		processed++

		if err := k.CheckDailyCaps(ctx, inviter, amount); err != nil {
			// 今天的额度也满了 → 重新排队到今天的桶，下一轮继续等。
			k.deferReward(ctx, inviter, amount)
			done = append(done, pendingMove{key: key, inviter: inviter, amount: sdkmath.ZeroInt()})
			continue
		}

		// 补发入账
		k.setPendingRewards(ctx, inviter, k.getPendingRewards(ctx, inviter).Add(amount))
		k.RecordDailyCapUsage(ctx, inviter, amount)
		done = append(done, pendingMove{key: key, inviter: inviter, amount: amount})

		ctx.EventManager().EmitEvent(sdk.NewEvent(
			types.EventTypeReferralRewardReleased,
			sdk.NewAttribute(types.AttrInviter, inviter),
			sdk.NewAttribute(types.AttrAmount, amount.String()),
			sdk.NewAttribute(types.AttrDayIndex, fmt.Sprintf("%d", dayIdx)),
		))
	}
	it.Close()

	// 迭代结束后统一处理（迭代期间改 store 会使迭代器行为未定义）
	for _, m := range done {
		store.Delete(m.key)
		if m.amount.IsPositive() {
			k.addDeferredTotal(ctx, m.amount.Neg())
		}
	}
	// 毒键清理：一次性删除并告警（不做账务调整 —— 无法解析的条目本就未计入
	// deferred total，删除不产生账务偏差）。
	for _, key := range invalidKeys {
		store.Delete(key)
	}
	if len(invalidKeys) > 0 {
		ctx.EventManager().EmitEvent(sdk.NewEvent(
			"referral.deferred_invalid_keys_removed",
			sdk.NewAttribute("count", fmt.Sprintf("%d", len(invalidKeys))),
		))
	}
}

// DeferredEntryCount 统计延后账本条数（受 types.DeferredCountScanBudget 限制，
// 仅用于查询与告警，不参与共识判定）。返回 (count, truncated)。
func (k Keeper) DeferredEntryCount(ctx sdk.Context) (int, bool) {
	store := ctx.KVStore(k.storeKey)
	it := sdk.KVStorePrefixIterator(store, []byte(types.DeferredRewardPrefix))
	defer it.Close()
	n := 0
	for ; it.Valid(); it.Next() {
		n++
		if n >= types.DeferredCountScanBudget {
			return n, true
		}
	}
	return n, false
}

// BudgetStatus 返佣预算的可观测快照。
type BudgetStatus struct {
	Remaining       sdkmath.Int
	DailyCapParam   uint64
	EffectiveCap    sdkmath.Int
	PhaseMultiplier uint32
	DaysRemaining   uint64
	DeferredTotal   sdkmath.Int
	BudgetExhausted bool

	// 把「已承诺但未领取」与「真正可用」分开暴露。
	// 只报 Remaining（账户余额）会让运维误以为还有一大笔可发，
	// 而实际上其中很大一部分已经承诺给用户了。
	PendingTotal sdkmath.Int
	Committed    sdkmath.Int
	Available    sdkmath.Int
}

// GetBudgetStatus 汇总返佣预算状态，供 CLI 查询与运维告警使用。
func (k Keeper) GetBudgetStatus(ctx sdk.Context) BudgetStatus {
	params := k.GetParams(ctx)
	eff, mult := k.EffectiveNetworkCap(ctx)
	rem := k.EcosystemBalance(ctx)
	pending := k.getPendingRewardTotal(ctx)
	committed := k.CommittedLiabilities(ctx)
	available := k.AvailableBudget(ctx)
	return BudgetStatus{
		Remaining:       rem,
		DailyCapParam:   params.DailyNetworkCap,
		EffectiveCap:    eff,
		PhaseMultiplier: mult,
		DaysRemaining:   k.DaysRemaining(ctx),
		DeferredTotal:   k.getDeferredTotal(ctx),
		BudgetExhausted: !rem.IsPositive(),
		PendingTotal:    pending,
		Committed:       committed,
		Available:       available,
	}
}

// ClampToUint64 供需要把 sdkmath.Int 落到 uint64 的场景使用（带饱和语义）。
func ClampToUint64(v sdkmath.Int) uint64 {
	return safemath.ClampUint64(v)
}

// MaybeEmitBudgetLowAlert 在预算跌破低水位时发出告警事件，按日去重（每天至多一次）。
//
// 为什么需要它：推荐预算若没有任何可观测性——`monitoring/` 下没有相关规则，
// 链上也没有余额查询。预算耗尽时唯一的信号是用户 ClaimRewards 报错，也就是说
// 运维只能在用户投诉之后才知道钱花完了。低水位事件把这件事变成一个可被
// Prometheus/Grafana 直接接住的链上信号。
func (k Keeper) MaybeEmitBudgetLowAlert(ctx sdk.Context) {
	initial := sdkmath.NewIntFromUint64(types.InitialEcosystemBudget)
	if !initial.IsPositive() {
		return
	}
	threshold := initial.Mul(sdkmath.NewIntFromUint64(uint64(types.BudgetLowThresholdBps))).Quo(sdkmath.NewInt(10000))

	remaining := k.EcosystemBalance(ctx)
	if remaining.GTE(threshold) {
		return
	}

	// 按日去重：事件每 4 秒一个区块，不做去重会污染整条事件流。
	store := ctx.KVStore(k.storeKey)
	alertKey := fmt.Sprintf("%s%d", types.BudgetLowAlertKeyPrefix, daywindow.DayIndex(ctx))
	if store.Has([]byte(alertKey)) {
		return
	}
	store.Set([]byte(alertKey), []byte{1})

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeReferralBudgetLow,
		sdk.NewAttribute(types.AttrRemaining, remaining.String()),
		sdk.NewAttribute(types.AttrThreshold, threshold.String()),
	))
}
