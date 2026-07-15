package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mcchain/x/tokenomics/types"
)

var _ types.QueryServer = Keeper{}

// Supply 实现 gRPC Query/Supply（R3：总量查询）。
func (k Keeper) Supply(goCtx context.Context, req *types.QuerySupplyRequest) (*types.QuerySupplyResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	return k.QuerySupply(ctx), nil
}

// Allocations 实现 gRPC Query/Allocations（R3：分配查询，实时读取各池余额）。
func (k Keeper) Allocations(goCtx context.Context, req *types.QueryAllocationsRequest) (*types.QueryAllocationsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	return k.QueryAllocations(ctx), nil
}

// Release 实现 gRPC Query/Release（R3：释放进度查询，实时计算）。
func (k Keeper) Release(goCtx context.Context, req *types.QueryReleaseRequest) (*types.QueryReleaseResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	return k.QueryRelease(ctx), nil
}

// QuerySupply 返回总量上限与已发行量（只读）。
func (k Keeper) QuerySupply(ctx sdk.Context) *types.QuerySupplyResponse {
	return &types.QuerySupplyResponse{
		TotalSupplyCap: types.TotalSupplyCap,
		MintedSupply:   k.GetMintedSupply(ctx).Uint64(),
		Denom:          types.DefaultDenom,
	}
}

// QueryAllocations 返回三大池分配视图，current_balance 为各池当前 bank 余额（实时）。
func (k Keeper) QueryAllocations(ctx sdk.Context) *types.QueryAllocationsResponse {
	allocs := k.GetAllocations(ctx)
	views := make([]types.PoolView, 0, len(allocs))
	for _, a := range allocs {
		var balance sdk.Coin = sdk.NewInt64Coin(types.DefaultDenom, 0)
		if addr, err := sdk.AccAddressFromBech32(a.Address); err == nil {
			balance = k.bankKeeper.GetBalance(ctx, addr, types.DefaultDenom)
		}
		views = append(views, types.PoolView{
			Name:           a.Name,
			PercentBps:     a.PercentBps,
			AllocatedAmount: a.AllocatedAmount,
			CurrentBalance: uint64(balance.Amount.Int64()),
			Address:        a.Address,
		})
	}
	return &types.QueryAllocationsResponse{Allocations: views}
}

// QueryRelease 返回团队池释放进度。进度基于曲线元数据 + 当前区块时间实时计算（Q9，不改状态）。
func (k Keeper) QueryRelease(ctx sdk.Context) *types.QueryReleaseResponse {
	rs := k.GetReleaseSchedule(ctx)
	now := ctx.BlockTime().Unix()
	vested, remaining, progress := ComputeVested(rs.TotalLocked, rs.StartTime, rs.EndTime, now)
	return &types.QueryReleaseResponse{
		Team: types.TeamRelease{
			Address:      rs.TeamAddress,
			TotalLocked:  rs.TotalLocked,
			Vested:       vested,
			Remaining:    remaining,
			ProgressBps:  progress,
			StartTime:    rs.StartTime,
			CliffTime:    rs.CliffTime,
			EndTime:      rs.EndTime,
		},
	}
}
