package simulation

import (
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"mcchain/x/edgeai/keeper"
	"mcchain/x/edgeai/types"
)

// bondDenom 与链上主代币一致（umc）。
const bondDenom = "umc"

// SimulateMsgCreateTask 模拟并真实广播一条 CreateTask 消息。
// creator 必须持有 >= reward 的余额以完成 escrow（需求方付费模型）；
// 不足时由 GenAndDeliverTxWithRandFees 计为跳过，不会令仿真失败。
func SimulateMsgCreateTask(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		reward := uint64(simtypes.RandIntBetween(r, 1, 1000))
		msg := &types.MsgCreateTask{
			Creator:     simAccount.Address.String(),
			Description: simtypes.RandStringOfLength(r, 10),
			Reward:      reward,
		}
		return deliver(r, app, ctx, simAccount, ak, bk, msg, msg.Type(),
			sdk.NewCoins(sdk.NewInt64Coin(bondDenom, int64(reward))))
	}
}
