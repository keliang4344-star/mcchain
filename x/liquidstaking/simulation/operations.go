package simulation

import (
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"mcchain/x/liquidstaking/keeper"
	"mcchain/x/liquidstaking/types"
)

// SimulateMsgLiquidStake generates a MsgLiquidStake.
func SimulateMsgLiquidStake(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		// Derive a valid valoper address from a random account; the validator may
		// not exist on-chain, in which case the message is recorded as skipped.
		valAcc, _ := simtypes.RandomAcc(r, accs)
		valAddr := sdk.ValAddress(valAcc.Address).String()
		msg := &types.MsgLiquidStake{
			Delegator: simAccount.Address.String(),
			Validator: valAddr,
			AmountUmc: uint64(r.Int63n(500_000_000) + 1_000_000),
		}
		return genDeliver(r, app, ctx, simAccount, ak, bk, msg, msg.Type(), sdk.Coins{})
	}
}

// SimulateMsgLiquidUnstake generates a MsgLiquidUnstake.
func SimulateMsgLiquidUnstake(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		msg := &types.MsgLiquidUnstake{
			Delegator:  simAccount.Address.String(),
			Validator:  "",
			SharesUlmc: uint64(r.Int63n(10_000) + 1),
		}
		return genDeliver(r, app, ctx, simAccount, ak, bk, msg, msg.Type(), sdk.Coins{})
	}
}

// SimulateMsgClaimMatured generates a MsgClaimMatured.
func SimulateMsgClaimMatured(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		msg := &types.MsgClaimMatured{
			Delegator: simAccount.Address.String(),
		}
		return genDeliver(r, app, ctx, simAccount, ak, bk, msg, msg.Type(), sdk.Coins{})
	}
}
