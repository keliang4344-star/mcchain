package keeper

import (
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"

	depinmoduletypes "mcchain/x/depin/types"
	"mcchain/x/dex/types"
)

// DistributeLPIncentive is called once per UTC day (see x/dex/module.go BeginBlock)
// to distribute the daily LP incentive across all pools that contain umc.
//
// Whitepaper line 507: 5,000 MC per day LP incentive for the first 6 months.
// The incentive is deposited directly into the umc side of every pool that
// contains umc, proportionally to each pool's umc reserve. This automatically
// benefits LP holders without needing to enumerate them.
//
// The incentive is sourced from the device incentive pool (depin module account),
// per whitepaper §24. If the depin module account has insufficient umc, the
// distribution is skipped for that day.
func (k Keeper) DistributeLPIncentive(ctx sdk.Context) {
	params := k.GetParams(ctx)

	// Check whether the incentive period has ended.
	if params.LpIncentiveEndHeight > 0 && uint64(ctx.BlockHeight()) >= params.LpIncentiveEndHeight {
		return
	}

	incentiveAmt, ok := sdk.NewIntFromString(params.LpIncentivePerDay)
	if !ok || incentiveAmt.IsZero() {
		return
	}

	// Find all pools containing umc and compute their share of total umc reserve.
	type poolReserve struct {
		poolID     uint64
		umcReserve sdk.Int
	}
	var pools []poolReserve
	totalUMC := sdk.ZeroInt()

	allPools := k.GetAllPools(ctx)
	for _, pool := range allPools {
		var umc sdk.Int
		var ok bool
		if pool.DenomA == InitialPoolDenomMC {
			umc, ok = sdk.NewIntFromString(pool.ReserveA)
		} else if pool.DenomB == InitialPoolDenomMC {
			umc, ok = sdk.NewIntFromString(pool.ReserveB)
		}
		if !ok || umc.IsZero() {
			continue
		}
		pools = append(pools, poolReserve{poolID: pool.Id, umcReserve: umc})
		totalUMC = totalUMC.Add(umc)
	}

	if len(pools) == 0 || totalUMC.IsZero() {
		return
	}

	// LP 激励是设备池的第三个出口，若完全
	// 绕过 depin 的每日线性释放闸门（既不 CheckDailyReleaseCap 也不
	// RecordDailyRelease）。后果是 5000 MC/日 在日释放账本之外静默流出，
	// 「4015 天线性释放」的可对账性被破坏：账本记的是「已释放 X」，实际设备池
	// 少了 X + 5000 MC/日，长此以往释放曲线与金库余额对不上，且无法归因。
	// 现在与 DePIN 任务奖励、节点资本津贴共用同一闸门。
	if k.depinKeeper == nil {
		ctx.Logger().Error("dex: LP incentive withheld — depin daily release gate not wired")
		return
	}
	if !incentiveAmt.IsUint64() {
		ctx.Logger().Error("dex: LP incentive withheld — amount exceeds uint64 release ledger",
			"amount", incentiveAmt.String())
		return
	}
	want := incentiveAmt.Uint64()
	allowed, dailyCap, remaining, err := k.depinKeeper.CheckDailyReleaseCap(ctx, want)
	if err != nil {
		ctx.Logger().Error("dex: LP incentive withheld — daily release cap check failed", "err", err)
		return
	}
	if !allowed {
		ctx.EventManager().EmitEvent(sdk.NewEvent(
			"dex.LPIncentiveThrottled",
			sdk.NewAttribute("requested", incentiveAmt.String()),
			sdk.NewAttribute("daily_cap", strconv.FormatUint(dailyCap, 10)),
			sdk.NewAttribute("remaining", strconv.FormatUint(remaining, 10)),
		))
		return
	}

	// Distribute proportionally.
	remainder := incentiveAmt
	distributed := sdk.ZeroInt()
	for i, pr := range pools {
		share := incentiveAmt.Mul(pr.umcReserve).Quo(totalUMC)

		// Last pool gets the remainder to avoid rounding dust.
		if i == len(pools)-1 {
			share = remainder
		} else {
			remainder = remainder.Sub(share)
		}

		if share.IsZero() {
			continue
		}

		coin := sdk.NewCoins(sdk.NewCoin(InitialPoolDenomMC, share))
		if err := k.bankKeeper.SendCoinsFromModuleToModule(
			ctx, depinmoduletypes.ModuleName, types.ModuleName, coin,
		); err != nil {
			// 失败池的份额必须归还 remainder —— 否则
			// 末池会拿走失败池的份额（其余额显著高于其按储备应得的比例）。
			remainder = remainder.Add(share)
			ctx.Logger().Error("LP incentive treasury transfer failed",
				"pool_id", pr.poolID,
				"amount", share.String(),
				"error", err,
			)
			continue
		}

		// Inject the incentive into the pool's umc reserve.
		pool, found := k.GetPool(ctx, pr.poolID)
		if !found {
			continue
		}
		if pool.DenomA == InitialPoolDenomMC {
			reserveA, _ := sdk.NewIntFromString(pool.ReserveA)
			pool.ReserveA = reserveA.Add(share).String()
		} else {
			reserveB, _ := sdk.NewIntFromString(pool.ReserveB)
			pool.ReserveB = reserveB.Add(share).String()
		}
		k.SetPool(ctx, pool)
		distributed = distributed.Add(share)
	}

	// 按实际出账记账（而非按预算金额）：任何中途失败都不得虚增「已释放」，
	// 否则释放账本会再次与真实余额脱节。
	if distributed.IsPositive() && distributed.IsUint64() {
		k.depinKeeper.RecordDailyRelease(ctx, distributed.Uint64())
	}

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeFeeDistribution,
		sdk.NewAttribute("action", "lp_incentive"),
		sdk.NewAttribute("amount", distributed.String()),
		sdk.NewAttribute("height", strconv.FormatInt(ctx.BlockHeight(), 10)),
		sdk.NewAttribute("pools_count", strconv.Itoa(len(pools))),
	))
}

// GetLastLPIncentiveDay 返回最近一次 LP 激励发放所属的 UTC 日序号；从未发放过时 ok=false。
func (k Keeper) GetLastLPIncentiveDay(ctx sdk.Context) (uint64, bool) {
	bz := ctx.KVStore(k.storeKey).Get(types.LPIncentiveDayKey)
	if bz == nil {
		return 0, false
	}
	day, err := strconv.ParseUint(string(bz), 10, 64)
	if err != nil {
		return 0, false
	}
	return day, true
}

// SetLastLPIncentiveDay 记录最近一次 LP 激励发放的 UTC 日序号。
func (k Keeper) SetLastLPIncentiveDay(ctx sdk.Context, day uint64) {
	ctx.KVStore(k.storeKey).Set(types.LPIncentiveDayKey, []byte(strconv.FormatUint(day, 10)))
}
