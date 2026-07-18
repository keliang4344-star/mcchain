package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"mcchain/x/edgeai/types"
)

func (k msgServer) CreateTask(goCtx context.Context, msg *types.MsgCreateTask) (*types.MsgCreateTaskResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	creatorAddr, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return nil, sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator (%s)", err)
	}

	// 需求方付费（escrow）：创建任务即由 creator 向 edgeai 模块账户托管 reward，
	// 结算时由该托管金拨付 submitter。reward=0 视为无奖励任务，跳过托管。
	if msg.Reward > 0 {
		rewardCoins := sdk.NewCoins(sdk.NewInt64Coin(types.EdgeAIDenom, int64(msg.Reward)))
		if k.bankKeeper.SpendableCoins(ctx, creatorAddr).AmountOf(types.EdgeAIDenom).LT(rewardCoins.AmountOf(types.EdgeAIDenom)) {
			return nil, sdkerrors.Wrapf(sdkerrors.ErrInsufficientFunds, "creator balance insufficient to escrow reward %d umc", msg.Reward)
		}
		if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, creatorAddr, types.ModuleName, rewardCoins); err != nil {
			return nil, fmt.Errorf("edgeai: escrow reward failed: %w", err)
		}
	}

	id := k.nextTaskID(ctx)
	t := &Task{
		Id:             id,
		Creator:        msg.Creator,
		Description:    msg.Description,
		Reward:         msg.Reward,
		Status:         types.TaskStatusOpen,
		CreatedAt:      ctx.BlockTime().Unix(),
		CreatedAtBlock: ctx.BlockHeight(),
	}
	if err := k.Keeper.SetTask(ctx, t); err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("edgeai.TaskCreated",
			sdk.NewAttribute("task_id", id),
			sdk.NewAttribute("creator", msg.Creator),
		),
	)
	return &types.MsgCreateTaskResponse{}, nil
}
