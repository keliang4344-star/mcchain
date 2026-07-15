package simulation

import (
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"mcchain/x/edgeai/keeper"
	"mcchain/x/edgeai/types"
)

// SimulateMsgSubmitResult 模拟并真实广播一条 SubmitResult 消息。
// 结果能否成功入账依赖任务存在且提交者已 attest（B2 闸口），随机仿真中多计为跳过。
func SimulateMsgSubmitResult(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		msg := &types.MsgSubmitResult{
			Creator:          simAccount.Address.String(),
			TaskId:           "1",
			ResultHash:       simtypes.RandStringOfLength(r, 16),
			AttestationNonce: simtypes.RandStringOfLength(r, 8),
		}
		return deliver(r, app, ctx, simAccount, ak, bk, msg, msg.Type(), sdk.Coins{})
	}
}
