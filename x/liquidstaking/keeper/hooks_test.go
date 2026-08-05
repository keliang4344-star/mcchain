package keeper

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"mcchain/x/liquidstaking/types"
)

// TestSlashHookWritesDownPool is the regression guard for the bank-run bug:
// a validator slash must immediately reduce the pool's bonded stake and the
// ulmc/umc exchange rate, otherwise early redeemers drain stake that no longer
// exists and the last holders absorb the whole loss.
func TestSlashHookWritesDownPool(t *testing.T) {
	f := setupLiquidStaking(t)
	val := f.staking.addValidator(t, false)
	user := f.fundedAddr(t, 500_000_000)

	_, err := f.k.LiquidStake(f.ctx, user, val, 100_000_000) // 100 MC, rate 1.0
	require.NoError(t, err)
	require.True(t, f.k.ExchangeRate(f.ctx).Equal(sdk.OneDec()))

	// x/staking reports a 5% effective slash on that validator.
	require.NoError(t, f.k.Hooks().BeforeValidatorSlashed(f.ctx, val, sdk.NewDecWithPrec(5, 2)))

	ps := f.k.GetPoolState(f.ctx)
	require.Equal(t, uint64(95_000_000), ps.TotalBondedUmc, "5% written off the pool")
	require.Equal(t, uint64(100_000_000), ps.TotalSharesUlmc, "share supply is untouched")
	require.Equal(t, uint64(5_000_000), ps.CumulativeSlashedUmc)
	require.Equal(t, uint64(95_000_000), f.k.GetValidatorBond(f.ctx, val.String()))
	require.True(t, f.k.ExchangeRate(f.ctx).Equal(sdk.NewDecWithPrec(95, 2)), "rate falls to 0.95")

	// The loss is shared, not front-run: redeeming everything now returns 95 MC.
	require.Equal(t, uint64(95_000_000), umcForShares(ps, 100_000_000))
}

// TestSlashHookPricesLaterDepositorsCorrectly checks that someone joining after
// a slash buys shares at the discounted rate instead of subsidising the loss.
func TestSlashHookPricesLaterDepositorsCorrectly(t *testing.T) {
	f := setupLiquidStaking(t)
	val := f.staking.addValidator(t, false)
	first := f.fundedAddr(t, 500_000_000)
	second := f.fundedAddr(t, 500_000_000)

	_, err := f.k.LiquidStake(f.ctx, first, val, 100_000_000)
	require.NoError(t, err)
	require.NoError(t, f.k.Hooks().BeforeValidatorSlashed(f.ctx, val, sdk.NewDecWithPrec(20, 2)))

	// Pool: 80 MC backing 100 ulmc → rate 0.8. A 80 MC deposit must mint 100 ulmc.
	shares, err := f.k.LiquidStake(f.ctx, second, val, 80_000_000)
	require.NoError(t, err)
	require.Equal(t, uint64(100_000_000), shares)

	ps := f.k.GetPoolState(f.ctx)
	require.Equal(t, uint64(160_000_000), ps.TotalBondedUmc)
	require.Equal(t, uint64(200_000_000), ps.TotalSharesUlmc)
	require.True(t, f.k.ExchangeRate(f.ctx).Equal(sdk.NewDecWithPrec(8, 1)))
}

// TestSlashHookEdgeCases covers no-ops, rounding and the wiped-pool guard.
func TestSlashHookEdgeCases(t *testing.T) {
	f := setupLiquidStaking(t)
	val := f.staking.addValidator(t, false)
	other := f.staking.addValidator(t, false)
	user := f.fundedAddr(t, 500_000_000)

	_, err := f.k.LiquidStake(f.ctx, user, val, 100_000_000)
	require.NoError(t, err)

	// A slash on a validator this module never delegated to changes nothing.
	require.NoError(t, f.k.Hooks().BeforeValidatorSlashed(f.ctx, other, sdk.NewDecWithPrec(50, 2)))
	require.Equal(t, uint64(100_000_000), f.k.GetPoolState(f.ctx).TotalBondedUmc)

	// A zero fraction is a no-op.
	require.NoError(t, f.k.Hooks().BeforeValidatorSlashed(f.ctx, val, sdk.ZeroDec()))
	require.Equal(t, uint64(100_000_000), f.k.GetPoolState(f.ctx).TotalBondedUmc)

	// A tiny fraction rounds the loss up, never down: the pool may understate
	// its backing by 1 umc but must never overstate it.
	require.NoError(t, f.k.Hooks().BeforeValidatorSlashed(f.ctx, val, sdk.NewDecWithPrec(1, 9)))
	require.Equal(t, uint64(99_999_999), f.k.GetPoolState(f.ctx).TotalBondedUmc)

	// A 100% slash wipes the pool; deposits are closed until governance acts.
	require.NoError(t, f.k.Hooks().BeforeValidatorSlashed(f.ctx, val, sdk.OneDec()))
	ps := f.k.GetPoolState(f.ctx)
	require.Equal(t, uint64(0), ps.TotalBondedUmc)
	require.Equal(t, uint64(0), f.k.GetValidatorBond(f.ctx, val.String()))

	_, err = f.k.LiquidStake(f.ctx, user, val, 10_000_000)
	require.ErrorIs(t, err, types.ErrPoolWipedOut)

	// Unstaking out of a wiped pool is refused rather than paying zero.
	_, err = f.k.LiquidUnstake(f.ctx, user, val.String(), 100_000_000)
	require.ErrorIs(t, err, types.ErrEmptyPool)
}
