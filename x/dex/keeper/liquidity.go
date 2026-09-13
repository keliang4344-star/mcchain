package keeper

import (
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/internal/safemath"
	"mcchain/x/dex/types"
	tokenomicstypes "mcchain/x/tokenomics/types"
)

// splitInitialLP 拆分首存铸出的 LP 总量：一部分永久锁死，其余归存款人。
//
// lpTotal 必须严格大于 types.MinimumLiquidity，否则首存过小、锁定份额会
// 吃掉全部份额，直接判定为流动性不足并拒绝，而不是给存款人铸 0 份。
func splitInitialLP(lpTotal sdk.Int) (userLP, lockedLP sdk.Int, ok bool) {
	lockedLP = sdk.NewInt(types.MinimumLiquidity)
	if lpTotal.IsNil() || lpTotal.LTE(lockedLP) {
		return sdk.ZeroInt(), sdk.ZeroInt(), false
	}
	return lpTotal.Sub(lockedLP), lockedLP, true
}

// lockMinimumLiquidity 铸出 MinimumLiquidity 份 LP 并永久打入黑洞地址。
//
// 黑洞地址由 tokenomics 的 black_hole 名称确定性派生，无人能构造出对应私钥，
// 且它不是已注册的模块账户（不在 bank 的 blocked 列表内），因此这笔份额收得进、
// 永远发不出——等价于 Uniswap V2 把 MINIMUM_LIQUIDITY 打给 address(0)。
//
// 注意：这里铸出的份额已计入 pool.TotalLp，因此 bank 侧 LP denom 的总供应与
// 池子记账始终一致，不会出现「记账有、链上无」的悬空份额。
func (k Keeper) lockMinimumLiquidity(ctx sdk.Context, lpDenom string, lockedLP sdk.Int) error {
	lockedCoins := sdk.NewCoins(sdk.NewCoin(lpDenom, lockedLP))
	if err := k.mintLPShares(ctx, lockedCoins); err != nil {
		return err
	}
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(
		ctx, types.ModuleName, tokenomicstypes.BlackHoleAddress(), lockedCoins,
	); err != nil {
		return err
	}
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"dex.MinimumLiquidityLocked",
		sdk.NewAttribute("lp_denom", lpDenom),
		sdk.NewAttribute("amount", lockedLP.String()),
		sdk.NewAttribute("black_hole", tokenomicstypes.BlackHoleAddress().String()),
	))
	return nil
}

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

	// 只要池子当前 LP 供应为 0（新池，或曾被全额赎回后重新播种），本次
	// 就是「首存」，必须永久锁死 MinimumLiquidity 份，防止首存者按操纵价独占定价权。
	// lpTotalMinted 是记账口径（用户份 + 锁定份），lpMinted 才是发给用户的份额。
	lockedLP := sdk.ZeroInt()
	lpTotalMinted := lpMinted
	if !totalLP.IsPositive() {
		var ok bool
		lpMinted, lockedLP, ok = splitInitialLP(lpTotalMinted)
		if !ok {
			return sdk.ZeroInt(), sdk.ZeroInt(), sdk.ZeroInt(), types.ErrInsufficientLiquidity
		}
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
	pool.TotalLp = totalLP.Add(lpTotalMinted).String()
	k.SetPool(ctx, pool)
	// lpMinted derives from user-supplied amounts and may exceed the int64
	// range; Int.Int64() would panic and abort the block, so saturate instead.
	telemetry.IncrCounter(safemath.Float32(lpMinted), "dex", "lp_minted")

	// Mint LP tokens to creator
	lpDenom := types.PoolDenom(poolID)
	lpCoins := sdk.NewCoins(sdk.NewCoin(lpDenom, lpMinted))
	if err := k.mintLPShares(ctx, lpCoins); err != nil {
		return sdk.ZeroInt(), sdk.ZeroInt(), sdk.ZeroInt(), err
	}
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, creatorAddr, lpCoins); err != nil {
		return sdk.ZeroInt(), sdk.ZeroInt(), sdk.ZeroInt(), err
	}

	// 首存的锁定份额同批铸出并直接进黑洞，永不可赎回。
	if lockedLP.IsPositive() {
		if err := k.lockMinimumLiquidity(ctx, lpDenom, lockedLP); err != nil {
			return sdk.ZeroInt(), sdk.ZeroInt(), sdk.ZeroInt(), err
		}
	}

	// Record LP lock: LP tokens are locked for params.LpLockBlocks (~4.7 days at the ~4 s block time)
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

	// Bound the burn against the outstanding LP supply BEFORE computing the
	// payout. Without this, redeeming more LP than the pool ever issued drove
	// both reserves and TotalLp negative, permanently corrupting pool state and
	// letting the caller withdraw more than their proportional share.
	if !totalLP.IsPositive() {
		return sdk.ZeroInt(), sdk.ZeroInt(), types.ErrPoolEmpty
	}
	if lpAmount.GT(totalLP) {
		return sdk.ZeroInt(), sdk.ZeroInt(), types.ErrInsufficientLiquidity
	}

	amountA, amountB = CalcRemoveLiquidity(reserveA, reserveB, lpAmount, totalLP)
	if !amountA.IsPositive() && !amountB.IsPositive() {
		// The burn is too small to redeem anything on either leg; refuse it
		// rather than burning the caller's LP for nothing.
		return sdk.ZeroInt(), sdk.ZeroInt(), types.ErrInsufficientLiquidity
	}
	if amountA.GT(reserveA) || amountB.GT(reserveB) {
		// Unreachable given the bound above; kept as a hard invariant so a
		// future refactor can never pay out more than the reserve holds.
		return sdk.ZeroInt(), sdk.ZeroInt(), types.ErrInsufficientLiquidity
	}
	if amountA.LT(minAOut) || amountB.LT(minBOut) {
		return sdk.ZeroInt(), sdk.ZeroInt(), types.ErrSlippageExceeded
	}

	// Check LP token lock: per whitepaper line 508, LP tokens are locked
	// for LpLockBlocks (default ~100800 ≈ 4.7 days at the ~4 s block time per the whitepaper).
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
	if err := k.burnLPShares(ctx, lpCoins); err != nil {
		return sdk.ZeroInt(), sdk.ZeroInt(), err
	}

	// Update reserves
	pool.ReserveA = reserveA.Sub(amountA).String()
	pool.ReserveB = reserveB.Sub(amountB).String()
	pool.TotalLp = totalLP.Sub(lpAmount).String()
	k.SetPool(ctx, pool)
	// lpAmount is user-supplied and may exceed the int64 range; Int.Int64()
	// would panic and abort the block, so saturate instead.
	telemetry.IncrCounter(safemath.Float32(lpAmount), "dex", "lp_burned")

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
