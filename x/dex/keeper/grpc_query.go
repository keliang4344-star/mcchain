package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mcchain/x/dex/types"
)

var _ types.QueryServer = Keeper{}

func (k Keeper) Pool(goCtx context.Context, req *types.QueryPoolRequest) (*types.QueryPoolResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	pool, found := k.GetPool(ctx, req.PoolId)
	if !found {
		return nil, status.Error(codes.NotFound, "pool not found")
	}

	return &types.QueryPoolResponse{Pool: &pool}, nil
}

func (k Keeper) Pools(goCtx context.Context, req *types.QueryPoolsRequest) (*types.QueryPoolsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	pools := k.GetAllPools(ctx)

	return &types.QueryPoolsResponse{
		Pools:      pools,
		Pagination: nil,
	}, nil
}

func (k Keeper) EstimateSwap(goCtx context.Context, req *types.QueryEstimateSwapRequest) (*types.QueryEstimateSwapResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	pool, found := k.GetPool(ctx, req.PoolId)
	if !found {
		return nil, status.Error(codes.NotFound, "pool not found")
	}

	amountIn, ok := sdk.NewIntFromString(req.AmountIn)
	if !ok || amountIn.LTE(sdk.ZeroInt()) {
		return nil, status.Error(codes.InvalidArgument, "invalid amount")
	}

	reserveIn, reserveOut, err := k.getReservesByDenom(pool, req.DenomIn, req.DenomOut)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	amountOut := CalcSwapOutput(reserveIn, reserveOut, amountIn, pool.FeeRateBps)

	// Calculate price impact in bps
	priceImpact := sdk.ZeroInt()
	if reserveIn.GT(sdk.ZeroInt()) && reserveOut.GT(sdk.ZeroInt()) {
		// price impact = 10000 - (output * reserveIn) / (input * reserveOut) * 10000
		idealOutput := amountIn.Mul(reserveOut).Quo(reserveIn)
		if idealOutput.GT(sdk.ZeroInt()) {
			impact := sdk.NewInt(10000).Sub(amountOut.Mul(sdk.NewInt(10000)).Quo(idealOutput))
			priceImpact = impact
		}
	}

	return &types.QueryEstimateSwapResponse{
		AmountOut:      amountOut.String(),
		PriceImpactBps: priceImpact.String(),
	}, nil
}

func (k Keeper) LiquidityLock(goCtx context.Context, req *types.QueryLiquidityLockRequest) (*types.QueryLiquidityLockResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	lock, found := k.GetLiquidityLock(ctx, req.LpAddress, req.PoolId)
	if !found {
		return nil, status.Error(codes.NotFound, "liquidity lock not found")
	}

	return &types.QueryLiquidityLockResponse{Lock: &lock}, nil
}

func (k Keeper) Price(goCtx context.Context, req *types.QueryPriceRequest) (*types.QueryPriceResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	pool, found := k.GetPool(ctx, req.PoolId)
	if !found {
		return nil, status.Error(codes.NotFound, "pool not found")
	}

	reserveA, okA := sdk.NewIntFromString(pool.ReserveA)
	reserveB, okB := sdk.NewIntFromString(pool.ReserveB)
	if !okA || !okB {
		return nil, status.Error(codes.Internal, "invalid reserve data")
	}

	if reserveA.IsZero() || reserveB.IsZero() {
		return nil, status.Error(codes.FailedPrecondition, "pool reserves are zero")
	}

	// price of denomA in terms of denomB: reserveB / reserveA
	// price of denomB in terms of denomA: reserveA / reserveB
	var price sdk.Int
	var priceDenom string
	if req.Denom == pool.DenomA || req.Denom == "" {
		price = reserveB.Mul(sdk.NewInt(1000000)).Quo(reserveA)
		priceDenom = pool.DenomB
	} else if req.Denom == pool.DenomB {
		price = reserveA.Mul(sdk.NewInt(1000000)).Quo(reserveB)
		priceDenom = pool.DenomA
	} else {
		return nil, status.Error(codes.InvalidArgument, "denom not in pool")
	}

	return &types.QueryPriceResponse{
		Price:      price.String(),
		PriceDenom: priceDenom,
	}, nil
}
