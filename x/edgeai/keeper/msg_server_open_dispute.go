package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"mcchain/x/edgeai/types"
)

func (k msgServer) OpenDispute(goCtx context.Context, msg *types.MsgOpenDispute) (*types.MsgOpenDisputeResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if _, err := sdk.AccAddressFromBech32(msg.Creator); err != nil {
		return nil, sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator (%s)", err)
	}

	task, err := k.Keeper.GetTask(ctx, msg.TaskId)
	if err != nil || task == nil {
		return nil, sdkerrors.Wrap(types.ErrTaskNotFound, msg.TaskId)
	}
	if task.Status == types.TaskStatusDisputed {
		return nil, types.ErrDisputeExists
	}

	existing, _ := k.Keeper.GetDispute(ctx, msg.TaskId)
	if existing != nil {
		return nil, types.ErrDisputeExists
	}

	d := &Dispute{
		TaskId:        msg.TaskId,
		Challenger:    msg.Creator,
		Reason:        msg.Reason,
		Status:        "open",
		Resolution:    "none",
		OpenedAt:      ctx.BlockTime().Unix(),
		OpenedAtBlock: ctx.BlockHeight(),
	}
	// B3.1：记录被质疑结果的提交者，供仲裁裁定 cheat 时 slash。
	if r, rerr := k.Keeper.GetResultByTask(ctx, msg.TaskId); rerr == nil && r != nil {
		d.Submitter = r.Submitter
	}
	if err := k.Keeper.SetDispute(ctx, d); err != nil {
		return nil, err
	}
	task.Status = types.TaskStatusDisputed
	_ = k.Keeper.SetTask(ctx, task)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("edgeai.DisputeOpened",
			sdk.NewAttribute("task_id", msg.TaskId),
			sdk.NewAttribute("challenger", msg.Creator),
		),
	)
	return &types.MsgOpenDisputeResponse{}, nil
}
