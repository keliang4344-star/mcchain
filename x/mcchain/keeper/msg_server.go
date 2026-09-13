package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/mcchain/types"
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

// InitiateHandover 发起渐进治理移交：登记新治理地址并启动时间锁。
// 只有当前治理主体（CurrentGovernor）可以发起。
func (k msgServer) InitiateHandover(goCtx context.Context, msg *types.MsgInitiateHandover) (*types.MsgInitiateHandoverResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if err := k.Keeper.AssertGovernanceAuthority(ctx, msg.Authority); err != nil {
		return nil, err
	}
	if err := k.Keeper.InitiateHandover(ctx, msg.NewGovernor); err != nil {
		return nil, err
	}

	cfg := k.Keeper.GetGovernanceHandoverConfig(ctx)
	return &types.MsgInitiateHandoverResponse{ActivationHeight: cfg.ActivationHeight}, nil
}

// CompleteHandover 执行渐进治理移交：仅在时间锁到期后允许，执行一次即终态。
func (k msgServer) CompleteHandover(goCtx context.Context, msg *types.MsgCompleteHandover) (*types.MsgCompleteHandoverResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if err := k.Keeper.AssertGovernanceAuthority(ctx, msg.Authority); err != nil {
		return nil, err
	}

	cfg := k.Keeper.GetGovernanceHandoverConfig(ctx)
	if !cfg.Enabled {
		return nil, fmt.Errorf("mcchain: governance handover is disabled")
	}
	if cfg.Executed {
		return nil, fmt.Errorf("mcchain: governance handover already executed")
	}
	if err := k.Keeper.CompleteHandover(ctx); err != nil {
		return nil, err
	}

	after := k.Keeper.GetGovernanceHandoverConfig(ctx)
	return &types.MsgCompleteHandoverResponse{NewGovernor: after.NewGovernor}, nil
}
