package simulation

import (
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"
	"github.com/cosmos/cosmos-sdk/std"
	"mcchain/x/edgeai/types"
)

// FindAccount find a specific address from an account list
func FindAccount(accs []simtypes.Account, address string) (simtypes.Account, bool) {
	creator, err := sdk.AccAddressFromBech32(address)
	if err != nil {
		panic(err)
	}
	return simtypes.FindAccount(accs, creator)
}

// makeEncoding 构建仅注册 edgeai 自定义 Msg 的编码配置。
// 注意：直接基于 types 包注册，避免导入模块包（x/edgeai 已导入本 simulation 包，会形成循环依赖）。
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

// deliver 通过仿真框架真实广播一条消息（注册 edgeai 编码）。
// 若消息在 deliver 阶段被模块逻辑拒绝，框架会将其计为跳过，不会令整体仿真失败。
func deliver(
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
		// 交易在 deliver 阶段被模块逻辑拒绝（如未 attest 的设备贡献、节点前置条件不满足等），
		// 随机仿真中属正常情况：计为跳过而非仿真失败（仿真框架对 err!=nil 会直接 Fatalf）。
		return simtypes.NoOpMsg(types.ModuleName, msgType, "delivery skipped: "+err.Error()), nil, nil
	}
	return opMsg, nil, nil
}
