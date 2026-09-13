package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"mcchain/x/phonenode/types"
)

func (k msgServer) SubmitAttestation(goCtx context.Context, msg *types.MsgSubmitAttestation) (*types.MsgSubmitAttestationResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// 提交者即节点身份（Creator）。必须已注册。
	nodeAddr := msg.Creator
	if _, err := sdk.AccAddressFromBech32(nodeAddr); err != nil {
		return nil, sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address (%s)", err)
	}

	if err := k.Keeper.SubmitAttestation(ctx, nodeAddr, msg.RootHash, msg.Nonce, msg.DeviceIdHash, msg.DevicePubKey, msg.Signature); err != nil {
		return nil, err
	}

	// 事件里不再广播原始 device_id_hash。事件日志是永久、全公开、可被任意
	// 索引器抓取的索引面，广播原文等于对外提供一条稳定的「设备指纹 ↔ 地址」映射，
	// 配合链下泄露的设备库即可反查到具体设备。改为广播带链 ID 域分隔的短承诺，
	// 保留「同一设备是否重复出现」的观测能力，但不提供直接反查能力。
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"phonenode.Attestation",
			sdk.NewAttribute("address", nodeAddr),
			sdk.NewAttribute("nonce", msg.Nonce),
			sdk.NewAttribute("device_commitment", DeviceIDCommitment(ctx.ChainID(), msg.DeviceIdHash)),
			sdk.NewAttribute("tier", "self"),
		),
	)

	return &types.MsgSubmitAttestationResponse{}, nil
}
