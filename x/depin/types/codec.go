package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

func RegisterCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgRegisterDevice{}, "depin/RegisterDevice", nil)
	cdc.RegisterConcrete(&MsgAttestDevice{}, "depin/AttestDevice", nil)
	cdc.RegisterConcrete(&MsgSubmitContribution{}, "depin/SubmitContribution", nil)
	cdc.RegisterConcrete(&MsgSubmitAttestation{}, "depin/SubmitAttestation", nil)
	// this line is used by starport scaffolding # 2
}

func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgRegisterDevice{},
	)
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgAttestDevice{},
	)
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgSubmitContribution{},
	)
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgSubmitAttestation{},
	)
	// this line is used by starport scaffolding # 3

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}

var (
	Amino     = codec.NewLegacyAmino()
	ModuleCdc = codec.NewProtoCodec(cdctypes.NewInterfaceRegistry())
)
