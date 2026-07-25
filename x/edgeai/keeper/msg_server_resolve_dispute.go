package keeper

import (
	"context"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"mcchain/x/edgeai/types"
)

// ResolveDispute B3.1 争议仲裁者裁定：
//   - 调用者必须是 edgeai 参数 arbitrator（部署时设为团队多签地址）；
//   - resolution 为 "honest" → 争议按乐观有效结案，BeginBlock 照常拨付；
//   - resolution 为 "cheat" → 标记任务作弊（拒绝拨付）并对结果提交者执行 slash。
func (k msgServer) ResolveDispute(goCtx context.Context, msg *types.MsgResolveDispute) (*types.MsgResolveDisputeResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if _, err := sdk.AccAddressFromBech32(msg.Creator); err != nil {
		return nil, sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator (%s)", err)
	}

	params := k.Keeper.GetParams(ctx)
	if params.Arbitrator == "" {
		return nil, types.ErrArbitratorNotSet
	}
	if msg.Creator != params.Arbitrator {
		return nil, sdkerrors.Wrap(sdkerrors.ErrUnauthorized, "only the configured arbitrator may resolve disputes")
	}
	if msg.Resolution != "honest" && msg.Resolution != "cheat" {
		return nil, types.ErrInvalidResolution
	}

	dispute, err := k.Keeper.GetDispute(ctx, msg.TaskId)
	if err != nil {
		return nil, err
	}
	if dispute == nil || dispute.Status != "open" {
		return nil, types.ErrDisputeNotOpen
	}
	task, err := k.Keeper.GetTask(ctx, msg.TaskId)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, sdkerrors.Wrap(types.ErrTaskNotFound, msg.TaskId)
	}

	if msg.Resolution == "cheat" {
		// O1 业务指标：edgeai 争议裁定作弊计数。
		telemetry.IncrCounter(1, "edgeai", "dispute_cheat_count")
		// 裁定作弊：拒绝拨付 + slash 结果提交者（硬约束：不 mint，仅 slash）。
		if dispute.Submitter != "" {
			if serr := k.Keeper.phonenodeKeeper.SlashIfBad(ctx, dispute.Submitter, "cheat_result", types.CheatSlashBps); serr != nil {
				ctx.Logger().Error("edgeai: cheat slash failed", "task_id", msg.TaskId, "submitter", dispute.Submitter, "err", serr.Error())
			}
			// 声誉更新：仲裁裁定作弊 → -10（白皮书行 497）
			k.Keeper.DecrementReputation(ctx, dispute.Submitter, types.ReputationCheatDecrease)
		}
		task.Status = types.TaskStatusCheated
		_ = k.Keeper.SetTask(ctx, task)
	}

	k.Keeper.resolveDispute(ctx, dispute, msg.Resolution)
	return &types.MsgResolveDisputeResponse{}, nil
}
