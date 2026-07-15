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

	if err := k.Keeper.SubmitAttestation(ctx, nodeAddr, msg.RootHash, msg.Nonce, msg.DeviceIdHash); err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"phonenode.Attestation",
			sdk.NewAttribute("address", nodeAddr),
			sdk.NewAttribute("nonce", msg.Nonce),
			sdk.NewAttribute("device_id_hash", msg.DeviceIdHash),
		),
	)

	return &types.MsgSubmitAttestationResponse{}, nil
}
