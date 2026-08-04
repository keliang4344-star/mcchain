package simulation

import (
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"mcchain/x/dex/keeper"
	"mcchain/x/dex/types"
)

// SimulateMsgCreatePool generates a MsgCreatePool.
func SimulateMsgCreatePool(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		amtA := sdk.NewInt(r.Int63n(1_000_000)+10_000).String()
		amtB := sdk.NewInt(r.Int63n(1_000_000)+10_000).String()
		msg := &types.MsgCreatePool{
			Creator:    simAccount.Address.String(),
			DenomA:     "umc",
			DenomB:     "uusdc",
			AmountA:    amtA,
			AmountB:    amtB,
			FeeRateBps: 30,
			PoolId:     uint64(r.Int63n(100)) + 1,
		}
		return genDeliver(r, app, ctx, simAccount, ak, bk, msg, msg.Type(), sdk.Coins{})
	}
}

// SimulateMsgAddLiquidity generates a MsgAddLiquidity.
func SimulateMsgAddLiquidity(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		msg := &types.MsgAddLiquidity{
			Creator:    simAccount.Address.String(),
			PoolId:     uint64(r.Int63n(100)) + 1,
			AmountAMax: sdk.NewInt(r.Int63n(1_000_000)+10_000).String(),
			AmountBMax: sdk.NewInt(r.Int63n(1_000_000)+10_000).String(),
			MinLpOut:   "1",
		}
		return genDeliver(r, app, ctx, simAccount, ak, bk, msg, msg.Type(), sdk.Coins{})
	}
}

// SimulateMsgRemoveLiquidity generates a MsgRemoveLiquidity.
func SimulateMsgRemoveLiquidity(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		msg := &types.MsgRemoveLiquidity{
			Creator:  simAccount.Address.String(),
			PoolId:   uint64(r.Int63n(100)) + 1,
			LpAmount: sdk.NewInt(r.Int63n(1000)+1).String(),
			MinAOut:  "1",
			MinBOut:  "1",
		}
		return genDeliver(r, app, ctx, simAccount, ak, bk, msg, msg.Type(), sdk.Coins{})
	}
}

// SimulateMsgSwapExactIn generates a MsgSwapExactIn.
func SimulateMsgSwapExactIn(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		msg := &types.MsgSwapExactIn{
			Creator:      simAccount.Address.String(),
			PoolId:       uint64(r.Int63n(100)) + 1,
			DenomIn:      "umc",
			DenomOut:     "uusdc",
			AmountIn:     sdk.NewInt(r.Int63n(100_000)+1000).String(),
			MinAmountOut: "1",
		}
		return genDeliver(r, app, ctx, simAccount, ak, bk, msg, msg.Type(), sdk.Coins{})
	}
}

// SimulateMsgSubmitSettlementBatch generates a MsgSubmitSettlementBatch.
func SimulateMsgSubmitSettlementBatch(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		recipient, _ := simtypes.RandomAcc(r, accs)
		msg := &types.MsgSubmitSettlementBatch{
			Creator:    simAccount.Address.String(),
			BatchId:    simtypes.RandStringOfLength(r, 16),
			MerkleRoot: simtypes.RandStringOfLength(r, 64),
			Entries: []*types.SettlementEntry{
				{
					Recipient: recipient.Address.String(),
					AmountUmc: uint64(r.Int63n(1000) + 1),
				},
			},
		}
		return genDeliver(r, app, ctx, simAccount, ak, bk, msg, msg.Type(), sdk.Coins{})
	}
}

// SimulateMsgFinalizeSettlementBatch generates a MsgFinalizeSettlementBatch.
func SimulateMsgFinalizeSettlementBatch(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		msg := &types.MsgFinalizeSettlementBatch{
			Creator: simAccount.Address.String(),
			BatchId: simtypes.RandStringOfLength(r, 16),
		}
		return genDeliver(r, app, ctx, simAccount, ak, bk, msg, msg.Type(), sdk.Coins{})
	}
}
