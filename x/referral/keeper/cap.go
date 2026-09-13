package keeper

import (
	"encoding/binary"
	"fmt"
	"math"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/internal/daywindow"
	"mcchain/internal/safemath"
	"mcchain/x/referral/types"
)

// dailyCapKey builds a per-user daily cap key suffixed with the current UTC day index.
//
// 口径统一：日序号来自 daywindow（区块时间 UTC 零点切日），
// 不再用「区块高度 / 除数」。旧口径的除数是按 6 秒出块算的 14400，而本链固化
// 4 秒出块，导致 referral 的「一天」只有 16 小时、全网日上限被等效放宽 1.5 倍、
// 返佣预算提前约 3.65 年耗尽。改为区块时间切日后，改出块速度再不会静默改变
// 「一天」的含义，也与 x/depin 的日界口径完全一致（END：不再存在两套窗口）。
func dailyCapKey(perUserKey string, ctx sdk.Context) []byte {
	day := daywindow.DayIndex(ctx)
	return append([]byte(perUserKey), uint64Bytes(day)...)
}

// BlockDayDivisor 返回「一天有多少个区块」。
//
// Deprecated: 日界已改为按区块时间切分（见 daywindow），本函数仅保留给需要
// 做容量估算（例如「N 天 ≈ 多少块」）的调用方，不再参与任何限额判定。
// 返回值按链上固化的 4 秒出块计算：86400 / 4 = 21600。
func BlockDayDivisor() int64 {
	return daywindow.BlocksPerDay
}

// ---- per-user daily cap ----

func (k Keeper) getDailyPerUser(ctx sdk.Context, user string) uint64 {
	store := ctx.KVStore(k.storeKey)
	key := dailyCapKey(types.DailyPerUserCapKeyPrefix+user, ctx)
	bz := store.Get(key)
	if bz == nil {
		return 0
	}
	return binary.BigEndian.Uint64(bz)
}

func (k Keeper) setDailyPerUser(ctx sdk.Context, user string, amount uint64) {
	store := ctx.KVStore(k.storeKey)
	key := dailyCapKey(types.DailyPerUserCapKeyPrefix+user, ctx)
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, amount)
	store.Set(key, b)
}

// ---- network-wide daily cap ----

func (k Keeper) getDailyNetwork(ctx sdk.Context) uint64 {
	store := ctx.KVStore(k.storeKey)
	key := dailyCapKey(types.DailyNetworkCapKey, ctx)
	bz := store.Get(key)
	if bz == nil {
		return 0
	}
	return binary.BigEndian.Uint64(bz)
}

func (k Keeper) setDailyNetwork(ctx sdk.Context, amount uint64) {
	store := ctx.KVStore(k.storeKey)
	key := dailyCapKey(types.DailyNetworkCapKey, ctx)
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, amount)
	store.Set(key, b)
}

// ---- cap check (called before TrackReward) ----

// CheckDailyCaps returns an error if the pending bonus would exceed either the
// per-user daily cap or the currently effective network-wide daily cap.
//
// bonus is denominated in umc (the smallest unit).
//
// 上限比较必须在任意精度上做。用 `used + bonus.Uint64` 比较有两个致命问题：
//   - bonus 超出 uint64 或为负时，Int.Uint64() 直接 panic —— DeliverTx 内 panic 会中止整个区块；
//   - `used + bonus` 溢出 uint64 时结果回绕成一个很小的数，上限检查反而通过，
//     日上限被完全绕过。二者都改为 sdkmath.Int 比较，不再有回绕与 panic 面。
//
// 全网上限不再直接用治理参数，而是取 EffectiveNetworkCap ——
//
//	有效上限 = min( 参数上限 × 阶段倍数 , 剩余预算 / 剩余天数 )
//
// 这样预算必然撑满整个释放窗口，且冷启动期可按阶段倍数前置投放而不可能超发。
func (k Keeper) CheckDailyCaps(ctx sdk.Context, inviter string, bonus sdkmath.Int) error {
	if bonus.IsNil() || !bonus.IsPositive() {
		return nil
	}
	params := k.GetParams(ctx)
	day := daywindow.DayIndex(ctx)

	// Per-user cap
	if params.DailyPerUserCap > 0 {
		used := k.getDailyPerUser(ctx, inviter)
		newUsed := sdkmath.NewIntFromUint64(used).Add(bonus)
		if newUsed.GT(sdkmath.NewIntFromUint64(params.DailyPerUserCap)) {
			return fmt.Errorf("referral reward exceeds daily-per-user cap: used=%d + bonus=%s > cap=%d (day=%d)",
				used, bonus.String(), params.DailyPerUserCap, day)
		}
	}

	// Network cap（配速后的有效上限）
	effCap, _ := k.EffectiveNetworkCap(ctx)
	if effCap.IsPositive() {
		used := k.getDailyNetwork(ctx)
		newUsed := sdkmath.NewIntFromUint64(used).Add(bonus)
		if newUsed.GT(effCap) {
			return fmt.Errorf("referral reward exceeds daily-network cap: used=%d + bonus=%s > effective_cap=%s (param_cap=%d, day=%d)",
				used, bonus.String(), effCap.String(), params.DailyNetworkCap, day)
		}
	} else {
		// 有效上限为 0：预算已耗尽。给出可区分的取巧错误语义（调用方据此决定
		// 是「延后」还是「放弃」），而不是让下游撞到一个含义模糊的 bank 错误。
		return fmt.Errorf("referral reward exceeds daily-network cap: effective_cap=0 (budget exhausted, param_cap=%d, day=%d)",
			params.DailyNetworkCap, day)
	}

	return nil
}

// RecordDailyCapUsage records the bonus against the daily counters.
//
// 计数器是 uint64，累加必须做饱和处理：若溢出回绕，当日已用额度会被清零，
// 同一天内可以反复领取而永远撞不到上限。饱和到 MaxUint64 会让后续检查一律失败，
// 这是比「静默解除限额」安全得多的失败方向。
func (k Keeper) RecordDailyCapUsage(ctx sdk.Context, inviter string, bonus sdkmath.Int) {
	if bonus.IsNil() || !bonus.IsPositive() {
		return
	}
	b := safemath.ClampUint64(bonus)
	if b == 0 {
		return
	}
	k.setDailyPerUser(ctx, inviter, saturatingAdd(k.getDailyPerUser(ctx, inviter), b))
	k.setDailyNetwork(ctx, saturatingAdd(k.getDailyNetwork(ctx), b))
}

// saturatingAdd 返回 a+b，溢出时饱和到 MaxUint64（绝不回绕）。
func saturatingAdd(a, b uint64) uint64 {
	if sum, ok := safemath.AddUint64(a, b); ok {
		return sum
	}
	return math.MaxUint64
}

// 每块清理预算：扫描 / 删除的键数上限。
//
// 朴素实现在「日边界那一个区块」里对全部 per-user 键做无界迭代 + 删除。
// 用户数一旦上量，那一个区块的 BeginBlock 就会超时——全网在同一高度卡死，
// 且每天复现一次，属停链级隐患。改为每块只处理固定预算，成本恒定。
const (
	dailyCapScanBudget    = 256 // per-user 前缀每块最多扫描的键数
	dailyCapNetScanBudget = 8   // network 前缀每块最多扫描的键数
)

// ResetDailyCaps 增量清理已过期（day < 今日）的日计数器。
//
// 计数器键本身带 day 后缀，跨日自然失效——**清理只是垃圾回收，不影响限额正确性**。
// 因此可以安全地分片执行：每块按预算扫描一段，用持久化游标跨块续扫，扫到尾部就
// 回到起点。即使某段时间没扫完，过期键也只是暂留，不会让任何人多领一分钱。
//
// 跨口径兼容：日序号从「高度/14400」切换为「Unix 日序号」后，历史小序号键会在
// 若干日内被本函数按「早于 today」全部回收，无需任何迁移脚本。
func (k Keeper) ResetDailyCaps(ctx sdk.Context) {
	store := ctx.KVStore(k.storeKey)
	today := daywindow.DayIndex(ctx)

	// ---- per-user：游标分片扫描 ----
	userStore := prefix.NewStore(store, []byte(types.DailyPerUserCapKeyPrefix))
	cursor := store.Get([]byte(types.DailyCapPruneCursorKey))
	stale, next := collectStaleKeys(userStore, cursor, today, dailyCapScanBudget)
	for _, key := range stale {
		userStore.Delete(key)
	}
	if next == nil {
		store.Delete([]byte(types.DailyCapPruneCursorKey)) // 扫到尾，下轮从头开始
	} else {
		store.Set([]byte(types.DailyCapPruneCursorKey), next)
	}

	// ---- network：每天仅 1 个键，过期键按 day 升序排在最前，从头扫小预算即可 ----
	netStore := prefix.NewStore(store, []byte(types.DailyNetworkCapKey))
	netStale, _ := collectStaleKeys(netStore, nil, today, dailyCapNetScanBudget)
	for _, key := range netStale {
		netStore.Delete(key)
	}
}

// collectStaleKeys 从 start 起最多扫描 budget 个键，返回其中 day 后缀早于 today 的键，
// 以及下一轮的续扫位置（nil 表示已扫到尾部）。
//
// 迭代期间不做删除（会使迭代器行为未定义），统一收集后再删。
func collectStaleKeys(store prefix.Store, start []byte, today uint64, budget int) (stale [][]byte, next []byte) {
	it := store.Iterator(start, nil)
	defer it.Close()

	scanned := 0
	for ; it.Valid(); it.Next() {
		if scanned >= budget {
			next = append([]byte(nil), it.Key()...)
			return stale, next
		}
		scanned++
		key := it.Key()
		if len(key) < 8 {
			// 非法键直接删除并计入 stale —— 若永久
			// 滞留会在游标轮转中反复被扫，白耗每块扫描预算。计数器键是纯 GC
			// 对象（不影响限额正确性），删除无账务风险。
			stale = append(stale, append([]byte(nil), key...))
			continue
		}
		if binary.BigEndian.Uint64(key[len(key)-8:]) < today {
			stale = append(stale, append([]byte(nil), key...))
		}
	}
	return stale, nil
}
