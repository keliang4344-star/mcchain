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
		// AUTH-1：设备地址必须等于签名者。此前此处漏设 Address，
		// 消息一律因地址非法被拒，该仿真操作等同空跑。
		msg := &types.MsgRegisterDevice{
			Creator: simAccount.Address.String(),
			Address: simAccount.Address.String(),
			Model:   simtypes.RandStringOfLength(r, 8),
			Os:      "android",
		}
		return genDeliver(r, app, ctx, simAccount, ak, bk, msg, msg.Type(), sdk.Coins{})
	}
}
