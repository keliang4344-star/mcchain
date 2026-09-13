package keeper

import (
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"mcchain/x/liquidstaking/types"
)

// TestMsgServerStakeUnstakeClaim drives the whole lifecycle through the
// protobuf Msg service rather than the keeper, which is the path a user
// transaction actually takes.
func TestMsgServerStakeUnstakeClaim(t *testing.T) {
	f := setupLiquidStaking(t)
	srv := NewMsgServerImpl(*f.k)
	goCtx := sdk.WrapSDKContext(f.ctx)

	val := f.staking.addValidator(t, false)
	user := f.fundedAddr(t, 500_000_000)

	// Stake.
	stakeRes, err := srv.LiquidStake(goCtx, &types.MsgLiquidStake{
		Delegator: user.String(),
		Validator: val.String(),
		AmountUmc: 100_000_000,
	})
	require.NoError(t, err)
	require.Equal(t, uint64(100_000_000), stakeRes.SharesMintedUlmc)
	require.Equal(t, int64(100_000_000), f.bank.GetBalance(f.ctx, user, types.LiquidBondDenom).Amount.Int64())

	// Unstake half.
	unstakeRes, err := srv.LiquidUnstake(goCtx, &types.MsgLiquidUnstake{
		Delegator:  user.String(),
		Validator:  val.String(),
		SharesUlmc: 50_000_000,
	})
	require.NoError(t, err)
	require.Equal(t, uint64(50_000_000), unstakeRes.AmountUmc)
	require.Greater(t, unstakeRes.CompletionUnixTime, f.ctx.BlockTime().Unix())

	// Claiming before maturity is rejected rather than silently succeeding, so
	// a premature transaction fails instead of burning gas for nothing.
	_, err = srv.ClaimMatured(goCtx, &types.MsgClaimMatured{Delegator: user.String()})
	require.ErrorIs(t, err, types.ErrNothingToClaim)

	// Advance past the unbonding period and claim.
	f.advance(22 * 24 * time.Hour)
	goCtx = sdk.WrapSDKContext(f.ctx)

	claimRes, err := srv.ClaimMatured(goCtx, &types.MsgClaimMatured{Delegator: user.String()})
	require.NoError(t, err)
	require.Equal(t, uint64(50_000_000), claimRes.AmountUmc)
	require.Equal(t, uint64(1), claimRes.EntriesClaimed)
}

// TestMsgServerRejectsInvalidInput checks the service surface rejects malformed
// requests instead of forwarding them to the keeper.
func TestMsgServerRejectsInvalidInput(t *testing.T) {
	f := setupLiquidStaking(t)
	srv := NewMsgServerImpl(*f.k)
	goCtx := sdk.WrapSDKContext(f.ctx)

	val := f.staking.addValidator(t, false)

	_, err := srv.LiquidStake(goCtx, &types.MsgLiquidStake{
		Delegator: "not-an-address",
		Validator: val.String(),
		AmountUmc: 100_000_000,
	})
	require.ErrorIs(t, err, types.ErrInvalidAddress)

	user := f.fundedAddr(t, 10_000_000)
	_, err = srv.LiquidStake(goCtx, &types.MsgLiquidStake{
		Delegator: user.String(),
		Validator: "not-a-validator",
		AmountUmc: 100_000_000,
	})
	require.ErrorIs(t, err, types.ErrInvalidAddress)

	_, err = srv.LiquidUnstake(goCtx, &types.MsgLiquidUnstake{
		Delegator:  "not-an-address",
		SharesUlmc: 1,
	})
	require.ErrorIs(t, err, types.ErrInvalidAddress)

	_, err = srv.ClaimMatured(goCtx, &types.MsgClaimMatured{Delegator: "not-an-address"})
	require.ErrorIs(t, err, types.ErrInvalidAddress)
}

// TestMsgValidateBasic covers the stateless checks run before a transaction
// ever reaches the module.
func TestMsgValidateBasic(t *testing.T) {
	f := setupLiquidStaking(t)
	val := f.staking.addValidator(t, false)
	user := f.fundedAddr(t, 1_000_000)

	require.NoError(t, (&types.MsgLiquidStake{
		Delegator: user.String(), Validator: val.String(), AmountUmc: 1,
	}).ValidateBasic())

	require.Error(t, (&types.MsgLiquidStake{
		Delegator: user.String(), Validator: val.String(), AmountUmc: 0,
	}).ValidateBasic())

	require.Error(t, (&types.MsgLiquidStake{
		Delegator: "bad", Validator: val.String(), AmountUmc: 1,
	}).ValidateBasic())

	// An empty validator is legal on unstake: the pool picks the largest bond.
	require.NoError(t, (&types.MsgLiquidUnstake{
		Delegator: user.String(), Validator: "", SharesUlmc: 1,
	}).ValidateBasic())

	require.Error(t, (&types.MsgLiquidUnstake{
		Delegator: user.String(), Validator: "", SharesUlmc: 0,
	}).ValidateBasic())

	require.NoError(t, (&types.MsgClaimMatured{Delegator: user.String()}).ValidateBasic())
	require.Error(t, (&types.MsgClaimMatured{Delegator: ""}).ValidateBasic())
}

// TestGrpcQueryServer exercises the read service the wallet and explorer use.
func TestGrpcQueryServer(t *testing.T) {
	f := setupLiquidStaking(t)
	srv := NewMsgServerImpl(*f.k)
	goCtx := sdk.WrapSDKContext(f.ctx)

	val := f.staking.addValidator(t, false)
	user := f.fundedAddr(t, 500_000_000)

	_, err := srv.LiquidStake(goCtx, &types.MsgLiquidStake{
		Delegator: user.String(), Validator: val.String(), AmountUmc: 200_000_000,
	})
	require.NoError(t, err)

	params, err := f.k.Params(goCtx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.True(t, params.Enabled)
	require.Equal(t, types.DefaultParams().MinStakeUmc, params.MinStakeUmc)

	pool, err := f.k.Pool(goCtx, &types.QueryPoolRequest{})
	require.NoError(t, err)
	require.Equal(t, uint64(200_000_000), pool.TotalBondedUmc)
	require.Equal(t, uint64(200_000_000), pool.TotalSharesUlmc)
	require.Equal(t, "1.000000000000000000", pool.ExchangeRate)

	_, err = srv.LiquidUnstake(goCtx, &types.MsgLiquidUnstake{
		Delegator: user.String(), Validator: val.String(), SharesUlmc: 40_000_000,
	})
	require.NoError(t, err)

	unb, err := f.k.Unbondings(goCtx, &types.QueryUnbondingsRequest{Delegator: user.String()})
	require.NoError(t, err)
	require.Len(t, unb.Entries, 1)
	require.Equal(t, uint64(40_000_000), unb.Entries[0].AmountUmc)
	require.False(t, unb.Entries[0].Claimed)

	_, err = f.k.Unbondings(goCtx, &types.QueryUnbondingsRequest{Delegator: "bad"})
	require.Error(t, err)
}
