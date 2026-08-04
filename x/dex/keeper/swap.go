package keeper

import (
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/dex/types"
)

// SwapExactIn performs an exact-input swap on the pool.
// Returns the output amount transferred to the trader.
func (k Keeper) SwapExactIn(
	ctx sdk.Context,
	poolID uint64,
	denomIn, denomOut string,
	amountIn sdk.Int,
	minAmountOut sdk.Int,
	creator string,
) (sdk.Int, error) {
	if amountIn.LTE(sdk.ZeroInt()) {
		return sdk.ZeroInt(), types.ErrZeroAmount
	}
	if denomIn == denomOut {
		return sdk.ZeroInt(), types.ErrSwapSameDenom
	}

	pool, found := k.GetPool(ctx, poolID)
	if !found {
		return sdk.ZeroInt(), types.ErrPoolNotFound
	}

	reserveIn, reserveOut, err := k.getReservesByDenom(pool, denomIn, denomOut)
	if err != nil {
		return sdk.ZeroInt(), err
	}

	// Calculate fee and the non-LP (burn) portion removed from pool reserves.
	// The LP portion (FeeLPBps) stays in the reserve; the non-LP portion
	// (burn + treasury) is extracted and burned via ProcessSwapFee. The swap's
	// effective input is amountIn minus the non-LP fee, so amountOut and the
	// reserve update stay consistent and preserve x*y=k.
	feeTotal := amountIn.MulRaw(int64(pool.FeeRateBps)).QuoRaw(10000)
	nonLPFee := feeTotal.MulRaw(nonLPFeeBps).QuoRaw(10000)

	// Effective input added to the reserve = amountIn - nonLPFee.
	amountOut := CalcSwapOutput(reserveIn, reserveOut, amountIn.Sub(nonLPFee), 0)
	if amountOut.LTE(sdk.ZeroInt()) {
		return sdk.ZeroInt(), types.ErrInsufficientLiquidity
	}
	if amountOut.LT(minAmountOut) {
		return sdk.ZeroInt(), types.ErrSlippageExceeded
	}

	// Update reserves: add effective input (amountIn - nonLPFee); the non-LP
	// fee is later burned by ProcessSwapFee, keeping module balance == reserves.
	newReserveIn := reserveIn.Add(amountIn).Sub(nonLPFee)
	newReserveOut := reserveOut.Sub(amountOut)
	k.updateReservesByDenom(&pool, denomIn, newReserveIn)
	k.updateReservesByDenom(&pool, denomOut, newReserveOut)
	k.SetPool(ctx, pool)

	telemetry.IncrCounter(1, "dex", "swap_count")
	telemetry.IncrCounter(float32(amountIn.Int64()), "dex", "swap_volume_in")

	// Transfer input from trader to module
	traderAddr, err := sdk.AccAddressFromBech32(creator)
	if err != nil {
		return sdk.ZeroInt(), err
	}
	coinsIn := sdk.NewCoins(sdk.NewCoin(denomIn, amountIn))
	if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, traderAddr, types.ModuleName, coinsIn); err != nil {
		return sdk.ZeroInt(), err
	}

	// Transfer output from module to trader
	coinsOut := sdk.NewCoins(sdk.NewCoin(denomOut, amountOut))
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, traderAddr, coinsOut); err != nil {
		return sdk.ZeroInt(), err
	}

	// Distribute the collected fee: burn 50%, treasury 30%, LP 20%
	if err := k.ProcessSwapFee(ctx, poolID, denomIn, amountIn, pool.FeeRateBps); err != nil {
		return sdk.ZeroInt(), err
	}

	return amountOut, nil
}

func (k Keeper) getReservesByDenom(pool types.Pool, denomIn, denomOut string) (reserveIn, reserveOut sdk.Int, err error) {
	reserveA, okA := sdk.NewIntFromString(pool.ReserveA)
	reserveB, okB := sdk.NewIntFromString(pool.ReserveB)
	if !okA || !okB {
		return sdk.ZeroInt(), sdk.ZeroInt(), types.ErrInvalidDenom
	}

	if denomIn == pool.DenomA && denomOut == pool.DenomB {
		return reserveA, reserveB, nil
	}
	if denomIn == pool.DenomB && denomOut == pool.DenomA {
		return reserveB, reserveA, nil
	}
	return sdk.ZeroInt(), sdk.ZeroInt(), types.ErrInvalidTokenPair
}

func (k Keeper) updateReservesByDenom(pool *types.Pool, denom string, newReserve sdk.Int) {
	if denom == pool.DenomA {
		pool.ReserveA = newReserve.String()
	} else {
		pool.ReserveB = newReserve.String()
	}
}
