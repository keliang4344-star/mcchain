package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"mcchain/x/phonenode/types"
)

func (k msgServer) SubmitStateProof(goCtx context.Context, msg *types.MsgSubmitStateProof) (*types.MsgSubmitStateProofResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// 提交者即节点身份（Creator）。必须已注册。
	nodeAddr := msg.Creator
	if _, err := sdk.AccAddressFromBech32(nodeAddr); err != nil {
		return nil, sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address (%s)", err)
	}

	if _, err := k.Keeper.GetNode(ctx, nodeAddr); err != nil {
		return nil, err
	}

	if _, err := k.Keeper.SubmitStateProof(ctx, nodeAddr, msg.Root, msg.Leaf, msg.Index, msg.Proof); err != nil {
		return nil, err
	}

	return &types.MsgSubmitStateProofResponse{}, nil
}
