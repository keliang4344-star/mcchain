package keeper

import (
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"mcchain/x/dex/types"
)

// Initial pool constants (whitepaper lines 504-505).
// Pool 1: MC (umc) / USDT (uusdt) at price 0.02 USDT/MC.
// LP tokens are minted to the community module as protocol-owned liquidity.
const (
	InitialPoolDenomMC   = "umc"
	InitialPoolDenomUSDT = "uusdt"

	// 5,000,000 MC × 10^6 = 5e12 umc
	InitialPoolMC = "5000000000000"

	// 100,000 USDT × 10^6 = 1e11 uusdt
	InitialPoolUSDT = "100000000000"

	// Fee rate for the genesis pool: 30 bps = 0.30%
	InitialPoolFeeRateBps = 30
)

// InitGenesisPool creates the initial liquidity pool at genesis if configured.
//
// Whitepaper lines 504-505: the chain starts with a single MC/USDT pool
// seeded with 5,000,000 MC and 100,000 USDT, implying an initial price of
// 0.02 USDT per MC. LP tokens are minted to the community module account
// (protocol-owned liquidity).
//
// This function is idempotent: if pool 1 already exists it is skipped.
func (k Keeper) InitGenesisPool(ctx sdk.Context) {
	// Idempotent guard: skip if pool 1 already exists.
	if _, found := k.GetPool(ctx, 1); found {
		return
	}

	amountMC, okMC := sdk.NewIntFromString(InitialPoolMC)
	amountUSDT, okUSDT := sdk.NewIntFromString(InitialPoolUSDT)
	if !okMC || !okUSDT {
		panic("invalid initial pool reserve constants")
	}

	// Genesis validation: initial price must be 0.02 USDT/MC, i.e.
	// MC reserve == USDT reserve × 50 (whitepaper lines 504-505).
	if !amountMC.Equal(amountUSDT.MulRaw(50)) {
		panic("genesis pool ratio invalid: expected 0.02 USDT/MC (MC = USDT * 50)")
	}

	// Ensure denoms are sorted alphabetically (required by the AMM).
	denoms := []string{InitialPoolDenomMC, InitialPoolDenomUSDT}
	sort.Strings(denoms)

	// Build the pool. The reserves are stored in the pool struct in sorted
	// order so we need to map the amounts accordingly.
	var reserveA, reserveB sdk.Int
	if denoms[0] == InitialPoolDenomMC {
		reserveA = amountMC
		reserveB = amountUSDT
	} else {
		reserveA = amountUSDT
		reserveB = amountMC
	}

	// Calculate initial LP tokens: geometric mean = sqrt(reserveA * reserveB).
	// We use integer square root; this is the standard AMM convention for
	// the first LP position.
	lpMinted := sdk.NewIntFromUint64(integerSqrt(
		reserveA.Mul(reserveB).BigInt().Uint64(),
	))

	pool := types.Pool{
		Id:         1,
		DenomA:     denoms[0],
		DenomB:     denoms[1],
		ReserveA:   reserveA.String(),
		ReserveB:   reserveB.String(),
		TotalLp:    lpMinted.String(),
		FeeRateBps: InitialPoolFeeRateBps,
		Owner:      "", // genesis pool has no owner; LP tokens go to community module
	}

	// ZERO-INFLATION GUARD: the initial pool reserves MUST be pre-funded, never minted.
	// MC (umc) is carved from the foundation T0 unlock and transferred to the dex module
	// account by x/tokenomics at genesis (within the 1B cap). USDT (uusdt) is provided
	// externally by the market-maker partner (whitepaper §24). The chain MUST NOT mint
	// either asset here — doing so would break the 10亿 MC hard cap.
	dexAddr := authtypes.NewModuleAddress(types.ModuleName)
	mcBal := k.bankKeeper.GetBalance(ctx, dexAddr, InitialPoolDenomMC)
	usdtBal := k.bankKeeper.GetBalance(ctx, dexAddr, InitialPoolDenomUSDT)
	if mcBal.Amount.LT(amountMC) || usdtBal.Amount.LT(amountUSDT) {
		ctx.Logger().Info("genesis pool skipped: reserves not pre-funded",
			"mc_have", mcBal.Amount.String(), "mc_need", amountMC.String(),
			"usdt_have", usdtBal.Amount.String(), "usdt_need", amountUSDT.String(),
			"note", "MC from foundation T0 (tokenomics), USDT from market maker")
		return
	}

	// Store the pool.
	k.SetPool(ctx, pool)
	k.SetNextPoolID(ctx, 2)

	// Mint LP tokens to the community module as protocol-owned liquidity.
	// LP share tokens (pool1 denom) are NOT MC and do not count toward the 1B cap,
	// so dex retains Minter permission solely for minting LP shares.
	lpDenom := types.PoolDenom(1)
	lpCoins := sdk.NewCoins(sdk.NewCoin(lpDenom, lpMinted))
	if err := k.bankKeeper.MintCoins(ctx, types.ModuleName, lpCoins); err != nil {
		panic(err)
	}
	if err := k.bankKeeper.SendCoinsFromModuleToModule(
		ctx, types.ModuleName, types.CommunityModuleName, lpCoins,
	); err != nil {
		panic(err)
	}

	ctx.Logger().Info("genesis pool created",
		"pool_id", 1,
		"denom_a", denoms[0],
		"denom_b", denoms[1],
		"reserve_a", reserveA.String(),
		"reserve_b", reserveB.String(),
		"lp_minted", lpMinted.String(),
	)
}

// integerSqrt returns the floor of the integer square root of x using
// Newton's method. Returns 0 for x == 0.
func integerSqrt(x uint64) uint64 {
	if x <= 1 {
		return x
	}
	r := x
	for r*r > x {
		r = (r + x/r) / 2
	}
	return r
}
