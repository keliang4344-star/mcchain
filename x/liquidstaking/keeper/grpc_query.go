package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mcchain/x/liquidstaking/types"
)

var _ types.QueryServer = Keeper{}

// Params returns the module configuration.
func (k Keeper) Params(goCtx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	p := k.GetParams(ctx)
	return &types.QueryParamsResponse{
		Enabled:                    p.Enabled,
		MinStakeUmc:                p.MinStakeUmc,
		MaxValidatorShareBps:       p.MaxValidatorShareBps,
		UnbondingClaimGraceSeconds: p.UnbondingClaimGraceSeconds,
	}, nil
}

// Pool returns pool accounting and the current receipt exchange rate.
func (k Keeper) Pool(goCtx context.Context, req *types.QueryPoolRequest) (*types.QueryPoolResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	ps := k.GetPoolState(ctx)
	return &types.QueryPoolResponse{
		TotalBondedUmc:       ps.TotalBondedUmc,
		TotalSharesUlmc:      ps.TotalSharesUlmc,
		TotalUnbondingUmc:    ps.TotalUnbondingUmc,
		CumulativeRewardsUmc: ps.CumulativeRewardsUmc,
		ExchangeRate:         k.ExchangeRate(ctx).String(),
	}, nil
}

// Unbondings returns the unbonding entries of one delegator.
func (k Keeper) Unbondings(goCtx context.Context, req *types.QueryUnbondingsRequest) (*types.QueryUnbondingsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	if _, err := sdk.AccAddressFromBech32(req.Delegator); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid delegator address")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	entries := k.GetDelegatorUnbondings(ctx, req.Delegator)
	out := make([]*types.UnbondingEntryView, 0, len(entries))
	for _, e := range entries {
		out = append(out, &types.UnbondingEntryView{
			Id:                 e.ID,
			Delegator:          e.Delegator,
			Validator:          e.Validator,
			AmountUmc:          e.AmountUmc,
			SharesBurnedUlmc:   e.SharesBurnedUlmc,
			CompletionUnixTime: e.CompletionUnixTime,
			Claimed:            e.Claimed,
		})
	}
	return &types.QueryUnbondingsResponse{Entries: out}, nil
}
