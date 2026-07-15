package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

func RegisterCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgCreatePool{}, "mcchain/dex/CreatePool", nil)
	cdc.RegisterConcrete(&MsgAddLiquidity{}, "mcchain/dex/AddLiquidity", nil)
	cdc.RegisterConcrete(&MsgRemoveLiquidity{}, "mcchain/dex/RemoveLiquidity", nil)
	cdc.RegisterConcrete(&MsgSwapExactIn{}, "mcchain/dex/SwapExactIn", nil)
}

func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}

var (
	Amino     = codec.NewLegacyAmino()
	ModuleCdc = codec.NewProtoCodec(cdctypes.NewInterfaceRegistry())
)
