package keeper

import (
	"context"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"mcchain/x/edgeai/types"
)

// ResolveDispute 争议仲裁者裁定：
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
		// 裁定作弊：对结果提交者执行**不可逆**罚没（slash 质押 + 声誉扣减）。
		//
		// 不可逆罚没只在此人工仲裁路径执行 —— 自动重算路径
		// （applyCheatOutcome）已改为「只冻结拨付、不罚没」，因为链上无法验证
		// 验证人自报的重算哈希是否真的经过正确计算。
		//
		// 资金处置（回收 85% + 退还创建者 15%）与任务状态改写统一由
		// resolveDispute 执行，此处不得重复调用 —— 模块账户混同托管金，
		// 调用两次会第二次划走其他任务的资金。
		if dispute.Submitter != "" {
			if serr := k.Keeper.phonenodeKeeper.SlashIfBad(ctx, dispute.Submitter, "cheat_result", types.CheatSlashBps); serr != nil {
				ctx.Logger().Error("edgeai: cheat slash failed", "task_id", msg.TaskId, "submitter", dispute.Submitter, "err", serr.Error())
			}
			// 声誉更新：仲裁裁定作弊 → -10（白皮书行 497）
			k.Keeper.DecrementReputation(ctx, dispute.Submitter, types.ReputationCheatDecrease)
		}
	}

	k.Keeper.resolveDispute(ctx, dispute, msg.Resolution)

	// 第二验证层：若争议期间有链上重算记录，结算时统一评估其证据
	// （与原始结果一致→质疑不成立；不一致→作弊确认，与仲裁结论互证/告警）。
	if _, ok := k.Keeper.GetRecompute(ctx, msg.TaskId); ok {
		if _, eerr := k.Keeper.EvaluateRecompute(ctx, msg.TaskId); eerr != nil {
			ctx.Logger().Error("edgeai: evaluate recompute on resolve failed", "task_id", msg.TaskId, "err", eerr.Error())
		}
	}

	return &types.MsgResolveDisputeResponse{}, nil
}
