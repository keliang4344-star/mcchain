package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

// RegisterCodec registers the module's messages on the legacy amino codec.
func RegisterCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgLiquidStake{}, "mcchain/liquidstaking/LiquidStake", nil)
	cdc.RegisterConcrete(&MsgLiquidUnstake{}, "mcchain/liquidstaking/LiquidUnstake", nil)
	cdc.RegisterConcrete(&MsgClaimMatured{}, "mcchain/liquidstaking/ClaimMatured", nil)
}

// RegisterInterfaces registers the module's messages on the interface registry.
func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgLiquidStake{},
		&MsgLiquidUnstake{},
		&MsgClaimMatured{},
	)

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}

var (
	Amino     = codec.NewLegacyAmino()
	ModuleCdc = codec.NewProtoCodec(cdctypes.NewInterfaceRegistry())
)
