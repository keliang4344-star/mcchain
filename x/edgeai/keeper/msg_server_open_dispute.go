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

	// 争议只允许针对**未结算**的任务。
	//
	// 攻击链（修复前）：任务 A 已结算（Done，85% 已发给提交者）→ 验证人 V 发
	// MsgOpenDispute(A) 把状态改写成 Disputed（只拦 Disputed、不拦 Done 时）→
	// 该状态改写使 clawbackSubmitterReward 的「Done 不可回收」闸门失效 →
	// SubmitRecompute(A, 伪哈希) 判 cheat → clawback 从 edgeai 模块账户转走
	// A.Reward 的 85%。而 A 的钱早已发完，转出的实际是 B/C 任务的托管金 ——
	// 后续 B/C 结算因余额不足失败。任何满足 VerifierMinStake 的验证人即可触发。
	//
	// 修复后：Done / Cheated / Expired 等终态一律不可开争议，攻击链在第一步断开。
	// 已结算任务若确有问题，走链下争议流程，不在链上重开。
	switch task.Status {
	case types.TaskStatusOpen, types.TaskStatusAssigned:
		// 允许：任务未结算，托管金仍在 escrow，争议不会挪用他人资金。
	default:
		return nil, sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest,
			"task %s is in terminal status %q; only open/assigned tasks can be disputed",
			msg.TaskId, task.Status)
	}

	existing, _ := k.Keeper.GetDispute(ctx, msg.TaskId)
	if existing != nil {
		return nil, types.ErrDisputeExists
	}

	// 争议资格闸门。
	//
	// 若任何地址可对任意任务零成本发起争议，而 BeginBlock 的争议超时路径
	// 会把未裁定的争议按 cheat 结案 —— 攻击者可用两笔交易让无辜提交者被
	// slash + 托管金回收。现在要求：
	//   1. 任务必须已有提交结果（无结果即无可争议对象）；
	//   2. 发起人必须是该任务的被指派验证者（持有 verification 记录）。
	// 普通第三方不再具备争议发起权；带证据的作弊路径由 SubmitVerification
	// 的哈希不一致自动建争议（唯一权威证据源）。
	if _, rerr := k.Keeper.GetResultByTask(ctx, msg.TaskId); rerr != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "task has no submitted result to dispute")
	}
	if !k.Keeper.HasVerification(ctx, msg.TaskId, msg.Creator) {
		return nil, sdkerrors.Wrap(sdkerrors.ErrUnauthorized,
			"only an assigned verifier may open a dispute for this task")
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
	// 记录被质疑结果的提交者，供仲裁裁定 cheat 时 slash。
	if r, rerr := k.Keeper.GetResultByTask(ctx, msg.TaskId); rerr == nil && r != nil {
		d.Submitter = r.Submitter
	}
	if err := k.Keeper.SetDispute(ctx, d); err != nil {
		return nil, err
	}
	task.Status = types.TaskStatusDisputed
	// 状态写入失败必须让整笔交易回滚，否则会留下
	// 「争议已建档、任务却仍是 Open」的不一致状态。
	if err := k.Keeper.SetTask(ctx, task); err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("edgeai.DisputeOpened",
			sdk.NewAttribute("task_id", msg.TaskId),
			sdk.NewAttribute("challenger", msg.Creator),
		),
	)
	return &types.MsgOpenDisputeResponse{}, nil
}
