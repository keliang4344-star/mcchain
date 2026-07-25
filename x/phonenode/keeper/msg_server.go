package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"mcchain/x/phonenode/types"
)

type msgServer struct {
	Keeper
}

// NewMsgServerImpl returns an implementation of the MsgServer interface
// for the provided Keeper.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = msgServer{}

// UpdateVerifierStatus 处理更新节点验证者状态的消息。
func (k msgServer) UpdateVerifierStatus(goCtx context.Context, msg *types.MsgUpdateVerifierStatus) (*types.MsgUpdateVerifierStatusResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if err := k.Keeper.UpdateVerifierStatus(ctx, msg.NodeId, msg.Status); err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"phonenode.VerifierStatusUpdated",
			sdk.NewAttribute("node_id", msg.NodeId),
			sdk.NewAttribute("status", msg.Status),
		),
	)

	return &types.MsgUpdateVerifierStatusResponse{}, nil
}
