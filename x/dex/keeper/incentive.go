package keeper

import (
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/dex/types"
)

// DistributeLPIncentive is called at the start of each day (every BlocksPerDay
// blocks) to distribute the daily LP incentive across all pools that contain
// umc.
//
// Whitepaper line 507: 5,000 MC per day LP incentive for the first 6 months.
// The incentive is deposited directly into the umc side of every pool that
// contains umc, proportionally to each pool's umc reserve. This automatically
// benefits LP holders without needing to enumerate them.
//
// The incentive is sourced from the protocol treasury (community module).
// If the treasury has insufficient umc, the distribution is skipped for
// that day.
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
		poolID      uint64
		umcReserve  sdk.Int
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

	// Distribute proportionally.
	remainder := incentiveAmt
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
			ctx, types.CommunityModuleName, types.ModuleName, coin,
		); err != nil {
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
	}

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeFeeDistribution,
		sdk.NewAttribute("action", "lp_incentive"),
		sdk.NewAttribute("amount", incentiveAmt.String()),
		sdk.NewAttribute("height", strconv.FormatInt(ctx.BlockHeight(), 10)),
		sdk.NewAttribute("pools_count", strconv.Itoa(len(pools))),
	))
}
