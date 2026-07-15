package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"mcchain/x/edgeai/types"
)

func (k msgServer) SubmitResult(goCtx context.Context, msg *types.MsgSubmitResult) (*types.MsgSubmitResultResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	nodeAddr := msg.Creator

	if _, err := sdk.AccAddressFromBech32(nodeAddr); err != nil {
		return nil, sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator (%s)", err)
	}

	// B2 attested‑execution gate
	if !k.Keeper.phonenodeKeeper.IsAttested(ctx, nodeAddr) {
		return nil, sdkerrors.Wrap(sdkerrors.ErrUnauthorized, "node not attested; result rejected")
	}

	task, err := k.Keeper.GetTask(ctx, msg.TaskId)
	if err != nil || task == nil {
		return nil, sdkerrors.Wrap(types.ErrTaskNotFound, msg.TaskId)
	}
	if task.Status != types.TaskStatusOpen {
		return nil, types.ErrTaskNotOpen
	}

	// 任务分配语义（B3.1 最小可用）：任务已指定 Assignee 时，仅该节点可提交结果，
	// 防止多节点重复抢占同一任务（与「首个有效结果发币」互补，明确归属）。
	if task.Assignee != "" && task.Assignee != nodeAddr {
		return nil, types.ErrNotAssigned
	}

	if k.Keeper.HasResult(ctx, msg.TaskId, nodeAddr) {
		return nil, types.ErrDuplicateResult
	}

	r := &Result{
		TaskId:           msg.TaskId,
		Submitter:        nodeAddr,
		ResultHash:       msg.ResultHash,
		AttestationNonce: msg.AttestationNonce,
		Status:           types.ResultStatusPending,
		SubmittedAt:      ctx.BlockTime().Unix(),
		SubmittedAtBlock: ctx.BlockHeight(),
	}
	if err := k.Keeper.SetResult(ctx, r); err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("edgeai.ResultSubmitted",
			sdk.NewAttribute("task_id", msg.TaskId),
			sdk.NewAttribute("submitter", nodeAddr),
		),
	)
	return &types.MsgSubmitResultResponse{}, nil
}
