package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"mcchain/x/depin/types"
)

// RegisterDevice 注册设备。
//
// AUTH-1 修复（阻断级授权缺陷）：
// 本消息的 GetSigners() 返回的是 Creator，而此前处理器却直接用未经认证的
// msg.Address 建档。由于 Keeper.RegisterDevice 对已存在地址返回 ErrDeviceExists，
// 任何人只需支付一笔手续费，即可用任意 Address 抢注他人地址的设备槽位，并写入
// 由攻击者指定的 Model/Os，使真实持有者**永久无法注册自己的设备**（不可逆 DoS）。
// 因此设备身份必须与交易签名者严格同一：Address 留空时取 Creator，
// 显式填写时必须与 Creator 完全一致。
func (k msgServer) RegisterDevice(goCtx context.Context, msg *types.MsgRegisterDevice) (*types.MsgRegisterDeviceResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// 签名者身份即设备身份（AUTH-1）
	deviceAddr, err := resolveSelfAddress(msg.Creator, msg.Address, "device")
	if err != nil {
		return nil, err
	}

	// 注册设备（仅入网；attestation 通过后才可提交贡献）
	if _, err := k.Keeper.RegisterDevice(ctx, deviceAddr, msg.Model, msg.Os); err != nil {
		return nil, err
	}

	return &types.MsgRegisterDeviceResponse{}, nil
}

// resolveSelfAddress 校验「目标地址 == 交易签名者」并返回规范化后的地址。
//
// 规则（AUTH-1）：
//   - creator 必须是合法 bech32 地址（它是 GetSigners() 的唯一来源，已由 SDK 验签）；
//   - target 为空时视为「作用于自身」，返回 creator；
//   - target 非空时必须与 creator 逐字节相等，否则拒绝——防止未授权方
//     代他人创建/变更链上身份记录。
func resolveSelfAddress(creator, target, what string) (string, error) {
	if _, err := sdk.AccAddressFromBech32(creator); err != nil {
		return "", sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address (%s)", err)
	}
	if target == "" {
		return creator, nil
	}
	if _, err := sdk.AccAddressFromBech32(target); err != nil {
		return "", sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid %s address (%s)", what, err)
	}
	if target != creator {
		return "", sdkerrors.Wrapf(
			sdkerrors.ErrUnauthorized,
			"%s address %s must equal the transaction signer %s", what, target, creator,
		)
	}
	return target, nil
}
