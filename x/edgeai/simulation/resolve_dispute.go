package simulation

import (
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"mcchain/x/edgeai/keeper"
	"mcchain/x/edgeai/types"
)

// SimulateMsgResolveDispute 模拟并广播一条 ResolveDispute 消息。
// 仿真中随机账户大概率非仲裁者（authz 门控），由框架计为跳过。
func SimulateMsgResolveDispute(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		resolution := "honest"
		if r.Intn(2) == 0 {
			resolution = "cheat"
		}
		msg := &types.MsgResolveDispute{
			Creator:    simAccount.Address.String(),
			TaskId:     "1",
			Resolution: resolution,
		}
		return deliver(r, app, ctx, simAccount, ak, bk, msg, msg.Type(), sdk.Coins{})
	}
}
