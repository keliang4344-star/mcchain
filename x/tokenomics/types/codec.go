package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
)

// RegisterCodec 注册 amino 编码（tokenomics 无 Msg，无需注册具体消息类型）。
func RegisterCodec(_ *codec.LegacyAmino) {
	// 本模块无 Msg，amino 注册为空。
}

// RegisterInterfaces 注册接口类型（tokenomics 无 Msg / 接口类型，无需注册）。
func RegisterInterfaces(_ cdctypes.InterfaceRegistry) {
	// 本模块无接口实现需注册。
}

var (
	// Amino 是模块遗留 amino 编解码器（占位，保持与 starport 约定一致）。
	Amino = codec.NewLegacyAmino()
	// ModuleCdc 是模块 proto 编解码器（占位）。
	ModuleCdc = codec.NewProtoCodec(cdctypes.NewInterfaceRegistry())
)
