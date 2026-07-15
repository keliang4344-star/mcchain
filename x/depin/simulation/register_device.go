package simulation

import (
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"mcchain/x/depin/keeper"
	"mcchain/x/depin/types"
)

func SimulateMsgRegisterDevice(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		msg := &types.MsgRegisterDevice{
			Creator: simAccount.Address.String(),
			Model:   simtypes.RandStringOfLength(r, 8),
		}
		return genDeliver(r, app, ctx, simAccount, ak, bk, msg, msg.Type(), sdk.Coins{})
	}
}
