package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"mcchain/x/depin/types"
)

// AttestDevice 提交设备认证。
//
// AUTH-1 修复：此前处理器只认 msg.Address（未经认证的普通字段），任何第三方都能
// 为他人设备提交 attestation——既可重放截获的认证材料，也可在软预言机模式下
// 直接把他人设备置为已认证。设备认证必须由设备账户本人签名提交。
func (k msgServer) AttestDevice(goCtx context.Context, msg *types.MsgAttestDevice) (*types.MsgAttestDeviceResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// 签名者身份即设备身份（AUTH-1）
	deviceAddr, err := resolveSelfAddress(msg.Creator, msg.Address, "device")
	if err != nil {
		return nil, err
	}

	st, err := k.Keeper.GetDevice(ctx, deviceAddr)
	if err != nil {
		return nil, err
	}

	// T2 可插拔预言机：attestation 校验交由 DefaultOracle（默认 SoftOracle，
	// 行为与历史一致；生产可 SetOracle(NewTeeOracle(pk)) 切换真实验签）。
	if err := types.DefaultOracle.VerifyDeviceAttestation(ctx, deviceAddr, msg.Challenge, msg.Signature); err != nil {
		return nil, err
	}

	st.Attested = true
	if err := k.Keeper.SetDevice(ctx, st); err != nil {
		return nil, err
	}

	return &types.MsgAttestDeviceResponse{}, nil
}
