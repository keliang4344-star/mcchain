package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/edgeai/types"
)

type msgServer struct {
	Keeper
}

func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = msgServer{}

// SubmitRecompute 第二验证层入口：记录链上重算结果指纹并立即评估。
// 与原始提交结果不一致 → 作弊确认（slash + 声誉扣减 + 回收奖励）；
// 一致 → 质疑不成立，挑战方承担轻度声誉扣减。
func (k msgServer) SubmitRecompute(goCtx context.Context, msg *types.MsgSubmitRecompute) (*types.MsgSubmitRecomputeResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if err := k.Keeper.RecordRecompute(ctx, msg.TaskId, msg.Creator, msg.RecomputeHash); err != nil {
		return nil, err
	}
	cheat, err := k.Keeper.EvaluateRecompute(ctx, msg.TaskId)
	if err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"edgeai.RecomputeEvaluated",
		sdk.NewAttribute("task_id", msg.TaskId),
		sdk.NewAttribute("requester", msg.Creator),
		sdk.NewAttribute("cheat_detected", fmt.Sprintf("%t", cheat)),
	))

	return &types.MsgSubmitRecomputeResponse{CheatDetected: cheat}, nil
}
