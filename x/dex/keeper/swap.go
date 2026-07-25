package keeper

import (
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

	amountOut := CalcSwapOutput(reserveIn, reserveOut, amountIn, pool.FeeRateBps)
	if amountOut.LTE(sdk.ZeroInt()) {
		return sdk.ZeroInt(), types.ErrInsufficientLiquidity
	}
	if amountOut.LT(minAmountOut) {
		return sdk.ZeroInt(), types.ErrSlippageExceeded
	}

	// Calculate fee and non-LP portion to deduct from pool reserves.
	// The LP portion (50%) stays in the reserve; burn (50%) is extracted
	// from the pool. No treasury share.
	feeTotal := amountIn.MulRaw(int64(pool.FeeRateBps)).QuoRaw(10000)
	nonLPFee := feeTotal.MulRaw(nonLPFeeBps).QuoRaw(10000)

	// Update reserves: subtract non-LP fee so only the LP portion (20%)
	// remains in the pool reserve.
	newReserveIn := reserveIn.Add(amountIn).Sub(nonLPFee)
	newReserveOut := reserveOut.Sub(amountOut)
	k.updateReservesByDenom(&pool, denomIn, newReserveIn)
	k.updateReservesByDenom(&pool, denomOut, newReserveOut)
	k.SetPool(ctx, pool)

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
