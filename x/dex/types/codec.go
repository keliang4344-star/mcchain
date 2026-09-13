package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

func RegisterCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgCreatePool{}, "mcchain/dex/CreatePool", nil)
	cdc.RegisterConcrete(&MsgAddLiquidity{}, "mcchain/dex/AddLiquidity", nil)
	cdc.RegisterConcrete(&MsgRemoveLiquidity{}, "mcchain/dex/RemoveLiquidity", nil)
	cdc.RegisterConcrete(&MsgSwapExactIn{}, "mcchain/dex/SwapExactIn", nil)
	cdc.RegisterConcrete(&MsgSubmitSettlementBatch{}, "mcchain/dex/SubmitSettlementBatch", nil)
	cdc.RegisterConcrete(&MsgFinalizeSettlementBatch{}, "mcchain/dex/FinalizeSettlementBatch", nil)
}

func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgCreatePool{},
		&MsgAddLiquidity{},
		&MsgRemoveLiquidity{},
		&MsgSwapExactIn{},
		&MsgSubmitSettlementBatch{},
		&MsgFinalizeSettlementBatch{},
	)

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}

var (
	Amino     = codec.NewLegacyAmino()
	ModuleCdc = codec.NewProtoCodec(cdctypes.NewInterfaceRegistry())
)
