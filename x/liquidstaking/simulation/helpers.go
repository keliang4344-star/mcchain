package simulation

import (
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/std"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"
	"mcchain/x/liquidstaking/types"
)

// FindAccount finds a specific address from an account list.
func FindAccount(accs []simtypes.Account, address string) (simtypes.Account, bool) {
	creator, err := sdk.AccAddressFromBech32(address)
	if err != nil {
		panic(err)
	}
	return simtypes.FindAccount(accs, creator)
}

// makeEncoding builds an encoding config that registers only the liquidstaking
// custom Msgs, avoiding import cycles with the app package.
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

// genDeliver builds the encoding config and broadcasts a message through the
// simulation framework. Messages rejected by module logic (e.g. validator not
// found) are recorded as skipped.
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
