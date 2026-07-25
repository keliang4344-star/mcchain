package simulation

import (
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"mcchain/x/phonenode/keeper"
	"mcchain/x/phonenode/types"
)

func SimulateMsgSubmitStateProof(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		msg := &types.MsgSubmitStateProof{
			Creator: simAccount.Address.String(),
			Root:    simtypes.RandStringOfLength(r, 16),
			Leaf:    simtypes.RandStringOfLength(r, 16),
			Index:   simtypes.RandStringOfLength(r, 8),
			Proof:   simtypes.RandStringOfLength(r, 32),
		}
		return genDeliver(r, app, ctx, simAccount, ak, bk, msg, msg.Type(), sdk.Coins{})
	}
}
