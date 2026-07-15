package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"mcchain/x/phonenode/types"
)

func (k msgServer) RegisterNode(goCtx context.Context, msg *types.MsgRegisterNode) (*types.MsgRegisterNodeResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// 节点地址必须合法（mc 前缀 bech32）
	if _, err := sdk.AccAddressFromBech32(msg.Address); err != nil {
		return nil, sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid node address (%s)", err)
	}

	if _, err := k.Keeper.RegisterNode(ctx, msg.Address, msg.Model, msg.Os, msg.Role); err != nil {
		return nil, err
	}

	return &types.MsgRegisterNodeResponse{}, nil
}
