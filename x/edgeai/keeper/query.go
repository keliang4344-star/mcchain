package keeper

import (
	"context"
	"encoding/json"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"mcchain/x/edgeai/types"
)

var _ types.QueryServer = Keeper{}

func (k Keeper) Params(goCtx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	return &types.QueryParamsResponse{Params: k.GetParams(ctx)}, nil
}

func (k Keeper) Task(goCtx context.Context, req *types.QueryTaskRequest) (*types.QueryTaskResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	task, err := k.GetTask(ctx, req.TaskId)
	if err != nil || task == nil {
		return nil, status.Errorf(codes.NotFound, "task %s not found", req.TaskId)
	}
	bz, _ := json.Marshal(task)
	return &types.QueryTaskResponse{TaskJson: string(bz)}, nil
}

func (k Keeper) Tasks(goCtx context.Context, req *types.QueryTasksRequest) (*types.QueryTasksResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	return &types.QueryTasksResponse{TaskIds: k.AllTaskIDs(ctx)}, nil
}

// Results 返回全部已提交结果（JSON 序列化，便于链下消费）。
func (k Keeper) Results(goCtx context.Context, req *types.QueryResultsRequest) (*types.QueryResultsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	out := make([]string, 0)
	for _, r := range k.AllResults(ctx) {
		bz, _ := json.Marshal(r)
		out = append(out, string(bz))
	}
	return &types.QueryResultsResponse{ResultsJson: out}, nil
}

// Disputes 返回全部争议记录（JSON 序列化，便于链下消费）。
func (k Keeper) Disputes(goCtx context.Context, req *types.QueryDisputesRequest) (*types.QueryDisputesResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	out := make([]string, 0)
	for _, d := range k.AllDisputes(ctx) {
		bz, _ := json.Marshal(d)
		out = append(out, string(bz))
	}
	return &types.QueryDisputesResponse{DisputesJson: out}, nil
}
