package simulation

import (
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"
	"github.com/cosmos/cosmos-sdk/std"
	"mcchain/x/depin/types"
)

// FindAccount find a specific address from an account list
func FindAccount(accs []simtypes.Account, address string) (simtypes.Account, bool) {
	creator, err := sdk.AccAddressFromBech32(address)
	if err != nil {
		panic(err)
	}
	return simtypes.FindAccount(accs, creator)
}

// makeEncoding 构建仅注册 depin 自定义 Msg 的编码配置（直接基于 types 包，避免循环依赖）。
func makeEncoding() (client.TxConfig, *codec.ProtoCodec) {
	amino := codec.NewLegacyAmino()
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(interfaceRegistry)
	txCfg := tx.NewTxConfig(cdc, tx.DefaultSignModes)
	std.RegisterLegacyAminoCodec(amino)
	std.RegisterInterfaces(interfaceRegistry)
	types.RegisterCodec(amino)
	types.RegisterInterfaces(interfaceRegistry)
	return txCfg, cdc
}

// genDeliver 构造编码配置（注册 depin 自定义 Msg）并通过仿真框架真实广播一条消息。
// 若消息在 deliver 阶段被模块逻辑拒绝（如未 attest 的设备的贡献），框架会将其计为跳过。
func genDeliver(
	r *rand.Rand,
	app *baseapp.BaseApp,
	ctx sdk.Context,
	simAccount simtypes.Account,
	ak types.AccountKeeper,
	bk types.BankKeeper,
	msg sdk.Msg,
	msgType string,
	spent sdk.Coins,
) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
	txCfg, cdc := makeEncoding()
	opMsg, _, err := simulation.GenAndDeliverTxWithRandFees(simulation.OperationInput{
		R:               r,
		App:             app,
		TxGen:           txCfg,
		Cdc:             cdc,
		Msg:             msg,
		MsgType:         msgType,
		CoinsSpentInMsg: spent,
		Context:         ctx,
		SimAccount:      simAccount,
		AccountKeeper:   ak,
		Bankkeeper:      bk,
		ModuleName:      types.ModuleName,
	})
	if err != nil {
		return simtypes.NoOpMsg(types.ModuleName, msgType, "delivery skipped: "+err.Error()), nil, nil
	}
	return opMsg, nil, nil
}
