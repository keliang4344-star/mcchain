package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

func RegisterCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgCreateReferral{}, "mcchain/referral/CreateReferral", nil)
	cdc.RegisterConcrete(&MsgClaimReferralReward{}, "mcchain/referral/ClaimReferralReward", nil)
}

func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)

	registry.RegisterImplementations(
		(*codec.ProtoMarshaler)(nil),
		&MsgCreateReferral{},
		&MsgCreateReferralResponse{},
		&MsgClaimReferralReward{},
		&MsgClaimReferralRewardResponse{},
		&Referral{},
	)
}

var (
	Amino     = codec.NewLegacyAmino()
	ModuleCdc = codec.NewProtoCodec(cdctypes.NewInterfaceRegistry())
)
