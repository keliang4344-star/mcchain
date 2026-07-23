package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/dex/types"
)

// AddLiquidity adds liquidity to an existing pool.
// Returns LP tokens minted and actual asset amounts used.
func (k Keeper) AddLiquidity(
	ctx sdk.Context,
	poolID uint64,
	amountAMax, amountBMax sdk.Int,
	minLPOut sdk.Int,
	creator string,
) (lpMinted, actualA, actualB sdk.Int, err error) {
	if amountAMax.LTE(sdk.ZeroInt()) || amountBMax.LTE(sdk.ZeroInt()) {
		return sdk.ZeroInt(), sdk.ZeroInt(), sdk.ZeroInt(), types.ErrZeroAmount
	}

	pool, found := k.GetPool(ctx, poolID)
	if !found {
		return sdk.ZeroInt(), sdk.ZeroInt(), sdk.ZeroInt(), types.ErrPoolNotFound
	}

	reserveA, okA := sdk.NewIntFromString(pool.ReserveA)
	reserveB, okB := sdk.NewIntFromString(pool.ReserveB)
	totalLP, okLP := sdk.NewIntFromString(pool.TotalLp)
	if !okA || !okB || !okLP {
		return sdk.ZeroInt(), sdk.ZeroInt(), sdk.ZeroInt(), types.ErrInvalidDenom
	}

	lpMinted, actualA, actualB = CalcAddLiquidity(reserveA, reserveB, amountAMax, amountBMax, totalLP)
	if lpMinted.LTE(sdk.ZeroInt()) {
		return sdk.ZeroInt(), sdk.ZeroInt(), sdk.ZeroInt(), types.ErrInsufficientLiquidity
	}
	if lpMinted.LT(minLPOut) {
		return sdk.ZeroInt(), sdk.ZeroInt(), sdk.ZeroInt(), types.ErrSlippageExceeded
	}

	// Transfer assets from creator to module
	creatorAddr, err := sdk.AccAddressFromBech32(creator)
	if err != nil {
		return sdk.ZeroInt(), sdk.ZeroInt(), sdk.ZeroInt(), err
	}

	coinsA := sdk.NewCoins(sdk.NewCoin(pool.DenomA, actualA))
	if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, creatorAddr, types.ModuleName, coinsA); err != nil {
		return sdk.ZeroInt(), sdk.ZeroInt(), sdk.ZeroInt(), err
	}

	coinsB := sdk.NewCoins(sdk.NewCoin(pool.DenomB, actualB))
	if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, creatorAddr, types.ModuleName, coinsB); err != nil {
		return sdk.ZeroInt(), sdk.ZeroInt(), sdk.ZeroInt(), err
	}

	// Update reserves
	pool.ReserveA = reserveA.Add(actualA).String()
	pool.ReserveB = reserveB.Add(actualB).String()
	pool.TotalLp = totalLP.Add(lpMinted).String()
	k.SetPool(ctx, pool)

	// Mint LP tokens to creator
	lpDenom := types.PoolDenom(poolID)
	lpCoins := sdk.NewCoins(sdk.NewCoin(lpDenom, lpMinted))
	if err := k.bankKeeper.MintCoins(ctx, types.ModuleName, lpCoins); err != nil {
		return sdk.ZeroInt(), sdk.ZeroInt(), sdk.ZeroInt(), err
	}
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, creatorAddr, lpCoins); err != nil {
		return sdk.ZeroInt(), sdk.ZeroInt(), sdk.ZeroInt(), err
	}

	// Record LP lock: LP tokens are locked for params.LpLockBlocks (~7 days)
	// per whitepaper line 508. Re-adding liquidity refreshes the lock and
	// accumulates the locked amount.
	params := k.GetParams(ctx)
	lockHeight := uint64(ctx.BlockHeight())
	lockedAmount := lpMinted
	if existing, found := k.GetLiquidityLock(ctx, creator, poolID); found {
		if prev, ok := sdk.NewIntFromString(existing.LpAmount); ok {
			lockedAmount = lockedAmount.Add(prev)
		}
	}
	k.SetLiquidityLock(ctx, types.LiquidityLock{
		LpAddress:    creator,
		PoolId:       poolID,
		LockHeight:   lockHeight,
		UnlockHeight: lockHeight + params.LpLockBlocks,
		LpAmount:     lockedAmount.String(),
	})

	return lpMinted, actualA, actualB, nil
}

// RemoveLiquidity removes liquidity from a pool and returns the proportional assets.
func (k Keeper) RemoveLiquidity(
	ctx sdk.Context,
	poolID uint64,
	lpAmount sdk.Int,
	minAOut, minBOut sdk.Int,
	creator string,
) (amountA, amountB sdk.Int, err error) {
	if lpAmount.LTE(sdk.ZeroInt()) {
		return sdk.ZeroInt(), sdk.ZeroInt(), types.ErrZeroAmount
	}

	pool, found := k.GetPool(ctx, poolID)
	if !found {
		return sdk.ZeroInt(), sdk.ZeroInt(), types.ErrPoolNotFound
	}

	reserveA, okA := sdk.NewIntFromString(pool.ReserveA)
	reserveB, okB := sdk.NewIntFromString(pool.ReserveB)
	totalLP, okLP := sdk.NewIntFromString(pool.TotalLp)
	if !okA || !okB || !okLP {
		return sdk.ZeroInt(), sdk.ZeroInt(), types.ErrInvalidDenom
	}

	amountA, amountB = CalcRemoveLiquidity(reserveA, reserveB, lpAmount, totalLP)
	if amountA.LT(minAOut) || amountB.LT(minBOut) {
		return sdk.ZeroInt(), sdk.ZeroInt(), types.ErrSlippageExceeded
	}

	// Check LP token lock: per whitepaper line 508, LP tokens are locked
	// for LpLockBlocks (default ~100800 = 7 days at 6s/block).
	if k.HasActiveLock(ctx, creator, poolID) {
		return sdk.ZeroInt(), sdk.ZeroInt(), types.ErrLpLocked
	}

	// Burn LP tokens from creator
	creatorAddr, err := sdk.AccAddressFromBech32(creator)
	if err != nil {
		return sdk.ZeroInt(), sdk.ZeroInt(), err
	}

	lpDenom := types.PoolDenom(poolID)
	lpCoins := sdk.NewCoins(sdk.NewCoin(lpDenom, lpAmount))
	if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, creatorAddr, types.ModuleName, lpCoins); err != nil {
		return sdk.ZeroInt(), sdk.ZeroInt(), err
	}
	if err := k.bankKeeper.BurnCoins(ctx, types.ModuleName, lpCoins); err != nil {
		return sdk.ZeroInt(), sdk.ZeroInt(), err
	}

	// Update reserves
	pool.ReserveA = reserveA.Sub(amountA).String()
	pool.ReserveB = reserveB.Sub(amountB).String()
	pool.TotalLp = totalLP.Sub(lpAmount).String()
	k.SetPool(ctx, pool)

	// Send assets to creator
	coinsA := sdk.NewCoins(sdk.NewCoin(pool.DenomA, amountA))
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, creatorAddr, coinsA); err != nil {
		return sdk.ZeroInt(), sdk.ZeroInt(), err
	}

	coinsB := sdk.NewCoins(sdk.NewCoin(pool.DenomB, amountB))
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, creatorAddr, coinsB); err != nil {
		return sdk.ZeroInt(), sdk.ZeroInt(), err
	}

	return amountA, amountB, nil
}
