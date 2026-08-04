package keeper

import (
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"mcchain/x/liquidstaking/types"
)

const day = 24 * time.Hour

// TestLiquidStakeMintsReceiptAndDelegates covers the happy path: MC leaves the
// user, gets delegated through the module account, and the user receives a
// 1:1 ulmc receipt on an empty pool.
func TestLiquidStakeMintsReceiptAndDelegates(t *testing.T) {
	f := setupLiquidStaking(t)
	val := f.staking.addValidator(t, false)
	user := f.fundedAddr(t, 500_000_000) // 500 MC

	shares, err := f.k.LiquidStake(f.ctx, user, val, 100_000_000) // 100 MC
	require.NoError(t, err)
	require.Equal(t, uint64(100_000_000), shares, "first depositor mints 1:1")

	// User paid MC and holds the receipt.
	require.Equal(t, int64(400_000_000), f.bank.amountOf(user.String(), types.BondDenom).Int64())
	require.Equal(t, int64(100_000_000), f.bank.amountOf(user.String(), types.LiquidBondDenom).Int64())

	// The stake is actually delegated, not sitting idle in the module account.
	moduleAddr := f.k.ModuleAddress().String()
	require.Equal(t, int64(100_000_000), f.staking.delegations[moduleAddr][val.String()].Int64())
	require.Equal(t, int64(0), f.bank.amountOf(moduleAddr, types.BondDenom).Int64())

	ps := f.k.GetPoolState(f.ctx)
	require.Equal(t, uint64(100_000_000), ps.TotalBondedUmc)
	require.Equal(t, uint64(100_000_000), ps.TotalSharesUlmc)
	require.Equal(t, uint64(100_000_000), f.k.GetValidatorBond(f.ctx, val.String()))
	require.True(t, f.k.ExchangeRate(f.ctx).Equal(sdk.OneDec()), "empty-pool rate is 1.0")
}

// TestLiquidStakeRejectsBadInput covers the guard rails.
func TestLiquidStakeRejectsBadInput(t *testing.T) {
	f := setupLiquidStaking(t)
	val := f.staking.addValidator(t, false)
	jailed := f.staking.addValidator(t, true)
	user := f.fundedAddr(t, 500_000_000)

	// Below the 1 MC minimum.
	_, err := f.k.LiquidStake(f.ctx, user, val, 999_999)
	require.ErrorIs(t, err, types.ErrBelowMinimumStake)

	// Jailed validator.
	_, err = f.k.LiquidStake(f.ctx, user, jailed, 10_000_000)
	require.ErrorIs(t, err, types.ErrValidatorJailed)

	// Unknown validator.
	unknown := f.staking.addValidator(t, false)
	delete(f.staking.validators, unknown.String())
	_, err = f.k.LiquidStake(f.ctx, user, unknown, 10_000_000)
	require.ErrorIs(t, err, types.ErrValidatorNotFound)

	// Governance pause.
	p := f.k.GetParams(f.ctx)
	p.Enabled = false
	require.NoError(t, f.k.SetParams(f.ctx, p))
	_, err = f.k.LiquidStake(f.ctx, user, val, 10_000_000)
	require.ErrorIs(t, err, types.ErrModuleDisabled)
}

// TestRewardsRaiseExchangeRate proves the core liquid-staking property: rewards
// compound into the pool without minting new shares, so every existing holder's
// receipt becomes worth more MC.
func TestRewardsRaiseExchangeRate(t *testing.T) {
	f := setupLiquidStaking(t)
	val := f.staking.addValidator(t, false)
	alice := f.fundedAddr(t, 1_000_000_000)

	_, err := f.k.LiquidStake(f.ctx, alice, val, 100_000_000)
	require.NoError(t, err)

	// 10 MC of staking rewards accrue to the module's delegation.
	f.distr.setReward(val.String(), 10_000_000)
	compounded, err := f.k.AccrueRewards(f.ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(10_000_000), compounded)

	ps := f.k.GetPoolState(f.ctx)
	require.Equal(t, uint64(110_000_000), ps.TotalBondedUmc)
	require.Equal(t, uint64(100_000_000), ps.TotalSharesUlmc, "share supply must not change")
	require.Equal(t, uint64(10_000_000), ps.CumulativeRewardsUmc)
	require.Equal(t, "1.100000000000000000", f.k.ExchangeRate(f.ctx).String())

	// A later depositor must mint fewer shares for the same MC.
	bob := f.fundedAddr(t, 1_000_000_000)
	bobShares, err := f.k.LiquidStake(f.ctx, bob, val, 110_000_000)
	require.NoError(t, err)
	require.Equal(t, uint64(100_000_000), bobShares, "110 MC at rate 1.1 buys 100 ulmc")
}

// TestUnstakeAndClaimFullCycle walks stake → unstake → wait out the unbonding
// period → claim, and checks the receipt is burned and the MC returned.
func TestUnstakeAndClaimFullCycle(t *testing.T) {
	f := setupLiquidStaking(t)
	val := f.staking.addValidator(t, false)
	user := f.fundedAddr(t, 500_000_000)

	_, err := f.k.LiquidStake(f.ctx, user, val, 200_000_000)
	require.NoError(t, err)

	entry, err := f.k.LiquidUnstake(f.ctx, user, val.String(), 120_000_000)
	require.NoError(t, err)
	require.Equal(t, uint64(120_000_000), entry.AmountUmc)
	require.Equal(t, uint64(1), entry.ID)

	// Receipt burned, not just moved.
	require.Equal(t, int64(80_000_000), f.bank.amountOf(user.String(), types.LiquidBondDenom).Int64())
	require.Equal(t, int64(120_000_000), f.bank.burned[types.LiquidBondDenom].Int64())

	ps := f.k.GetPoolState(f.ctx)
	require.Equal(t, uint64(80_000_000), ps.TotalBondedUmc)
	require.Equal(t, uint64(80_000_000), ps.TotalSharesUlmc)
	require.Equal(t, uint64(120_000_000), ps.TotalUnbondingUmc)

	// Claiming before maturity must fail.
	_, err = f.k.ClaimMatured(f.ctx, user)
	require.ErrorIs(t, err, types.ErrNothingToClaim)

	// Wait out the 21-day unbonding period.
	f.advance(22 * day)
	paid, err := f.k.ClaimMatured(f.ctx, user)
	require.NoError(t, err)
	require.Equal(t, uint64(120_000_000), paid)

	// 500 - 200 staked + 120 redeemed = 420 MC.
	require.Equal(t, int64(420_000_000), f.bank.amountOf(user.String(), types.BondDenom).Int64())
	require.Empty(t, f.k.GetDelegatorUnbondings(f.ctx, user.String()), "claimed receipts are pruned")
	require.Equal(t, uint64(0), f.k.GetPoolState(f.ctx).TotalUnbondingUmc)

	// Nothing left to claim.
	_, err = f.k.ClaimMatured(f.ctx, user)
	require.ErrorIs(t, err, types.ErrNothingToClaim)
}

// TestUnstakeRedeemsAccruedRewards checks that a holder who stayed through a
// reward epoch redeems more MC than they deposited.
func TestUnstakeRedeemsAccruedRewards(t *testing.T) {
	f := setupLiquidStaking(t)
	val := f.staking.addValidator(t, false)
	user := f.fundedAddr(t, 1_000_000_000)

	shares, err := f.k.LiquidStake(f.ctx, user, val, 100_000_000)
	require.NoError(t, err)

	f.distr.setReward(val.String(), 20_000_000) // +20 MC
	_, err = f.k.AccrueRewards(f.ctx)
	require.NoError(t, err)

	entry, err := f.k.LiquidUnstake(f.ctx, user, val.String(), shares)
	require.NoError(t, err)
	require.Equal(t, uint64(120_000_000), entry.AmountUmc, "100 MC staked redeems 120 MC after rewards")

	f.advance(22 * day)
	paid, err := f.k.ClaimMatured(f.ctx, user)
	require.NoError(t, err)
	require.Equal(t, uint64(120_000_000), paid)

	ps := f.k.GetPoolState(f.ctx)
	require.Equal(t, uint64(0), ps.TotalSharesUlmc)
	require.Equal(t, uint64(0), ps.TotalBondedUmc)
}

// TestUnstakeAutoSelectsLargestValidator verifies the empty-validator path
// drains the most concentrated position first.
func TestUnstakeAutoSelectsLargestValidator(t *testing.T) {
	f := setupLiquidStaking(t)
	small := f.staking.addValidator(t, false)
	large := f.staking.addValidator(t, false)
	user := f.fundedAddr(t, 1_000_000_000)

	_, err := f.k.LiquidStake(f.ctx, user, small, 50_000_000)
	require.NoError(t, err)
	_, err = f.k.LiquidStake(f.ctx, user, large, 300_000_000)
	require.NoError(t, err)

	entry, err := f.k.LiquidUnstake(f.ctx, user, "", 100_000_000)
	require.NoError(t, err)
	require.Equal(t, large.String(), entry.Validator)
	require.Equal(t, uint64(200_000_000), f.k.GetValidatorBond(f.ctx, large.String()))
	require.Equal(t, uint64(50_000_000), f.k.GetValidatorBond(f.ctx, small.String()))
}

// TestUnstakeRejectsOverdraw makes sure a user cannot redeem shares they do not
// hold.
func TestUnstakeRejectsOverdraw(t *testing.T) {
	f := setupLiquidStaking(t)
	val := f.staking.addValidator(t, false)
	user := f.fundedAddr(t, 500_000_000)

	_, err := f.k.LiquidStake(f.ctx, user, val, 100_000_000)
	require.NoError(t, err)

	_, err = f.k.LiquidUnstake(f.ctx, user, val.String(), 100_000_001)
	require.ErrorIs(t, err, types.ErrInsufficientShares)

	// Empty pool rejects outright.
	g := setupLiquidStaking(t)
	_, err = g.k.LiquidUnstake(g.ctx, g.fundedAddr(t, 1), "", 1)
	require.ErrorIs(t, err, types.ErrEmptyPool)
}

// TestValidatorConcentrationCap proves the per-validator cap engages once the
// pool is spread across enough validators to satisfy it.
func TestValidatorConcentrationCap(t *testing.T) {
	f := setupLiquidStaking(t)
	user := f.fundedAddr(t, 10_000_000_000)

	// Default cap is 20% => at least five validators before it can bind.
	vals := make([]string, 0, 5)
	for i := 0; i < 5; i++ {
		v := f.staking.addValidator(t, false)
		vals = append(vals, v.String())
		_, err := f.k.LiquidStake(f.ctx, user, v, 100_000_000)
		require.NoError(t, err, "cap must not block bootstrapping")
	}

	// Pool is 500 MC across five validators (20% each). Adding to one of them
	// pushes it over the cap.
	valAddr, err := sdk.ValAddressFromBech32(vals[0])
	require.NoError(t, err)
	_, err = f.k.LiquidStake(f.ctx, user, valAddr, 100_000_000)
	require.ErrorIs(t, err, types.ErrValidatorCapExceed)

	// A sixth validator is still accepted.
	v6 := f.staking.addValidator(t, false)
	_, err = f.k.LiquidStake(f.ctx, user, v6, 100_000_000)
	require.NoError(t, err)
}

// TestGenesisRoundTrip verifies state survives export/import.
func TestGenesisRoundTrip(t *testing.T) {
	f := setupLiquidStaking(t)
	val := f.staking.addValidator(t, false)
	user := f.fundedAddr(t, 500_000_000)

	_, err := f.k.LiquidStake(f.ctx, user, val, 200_000_000)
	require.NoError(t, err)
	_, err = f.k.LiquidUnstake(f.ctx, user, val.String(), 50_000_000)
	require.NoError(t, err)

	gs := types.GenesisState{
		Params:          f.k.GetParams(f.ctx),
		PoolState:       f.k.GetPoolState(f.ctx),
		UnbondingQueue:  f.k.AllUnbondings(f.ctx),
		ValidatorBonds:  f.k.AllValidatorBonds(f.ctx),
		NextUnbondingID: f.k.NextUnbondingID(f.ctx),
	}
	require.NoError(t, gs.Validate())
	require.Len(t, gs.UnbondingQueue, 1)
	require.Len(t, gs.ValidatorBonds, 1)
	require.Equal(t, uint64(2), gs.NextUnbondingID)

	// Import into a clean keeper.
	g := setupLiquidStaking(t)
	require.NoError(t, g.k.SetParams(g.ctx, gs.Params))
	g.k.SetPoolState(g.ctx, gs.PoolState)
	for _, e := range gs.UnbondingQueue {
		g.k.SetUnbondingEntry(g.ctx, e)
	}
	g.k.ImportValidatorBonds(g.ctx, gs.ValidatorBonds)
	g.k.ImportNextUnbondingID(g.ctx, gs.NextUnbondingID)

	require.Equal(t, gs.PoolState, g.k.GetPoolState(g.ctx))
	require.Equal(t, gs.NextUnbondingID, g.k.NextUnbondingID(g.ctx))
	require.Equal(t, gs.ValidatorBonds, g.k.AllValidatorBonds(g.ctx))
	require.Equal(t, gs.UnbondingQueue, g.k.AllUnbondings(g.ctx))
}

// TestDefaultGenesisValidates guards the launch parameters.
func TestDefaultGenesisValidates(t *testing.T) {
	require.NoError(t, types.DefaultGenesis().Validate())

	bad := types.DefaultParams()
	bad.MaxValidatorShareBps = 0
	require.Error(t, bad.Validate())

	bad = types.DefaultParams()
	bad.MinStakeUmc = 0
	require.Error(t, bad.Validate())
}
