package simulation

import (
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"mcchain/x/edgeai/keeper"
	"mcchain/x/edgeai/types"
)

// SimulateMsgOpenDispute 模拟并广播一条 OpenDispute 消息。
// 仿真中随机 task 大概率不存在或已无 pending 结果，由框架计为跳过。
func SimulateMsgOpenDispute(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		msg := &types.MsgOpenDispute{
			Creator: simAccount.Address.String(),
			TaskId:  "1",
			Reason:  simtypes.RandStringOfLength(r, 12),
		}
		return deliver(r, app, ctx, simAccount, ak, bk, msg, msg.Type(), sdk.Coins{})
	}
}
