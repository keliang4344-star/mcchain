package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mcchain/x/referral/types"
)

var _ types.QueryServer = Keeper{}

// Referral returns a single referral by ID.
func (k Keeper) Referral(goCtx context.Context, req *types.QueryReferralRequest) (*types.QueryReferralResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	ref, found := k.GetReferral(ctx, req.ReferralId)
	if !found {
		return nil, status.Error(codes.NotFound, "referral not found")
	}

	return &types.QueryReferralResponse{Referral: ref}, nil
}

// ReferralsByInviter returns all referrals made by a given inviter.
func (k Keeper) ReferralsByInviter(goCtx context.Context, req *types.QueryReferralsByInviterRequest) (*types.QueryReferralsByInviterResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	refs := k.GetReferralsByInviter(ctx, req.Inviter)

	return &types.QueryReferralsByInviterResponse{Referrals: refs}, nil
}

// PendingRewards returns the pending reward amount for a given claimer.
func (k Keeper) PendingRewards(goCtx context.Context, req *types.QueryPendingRewardsRequest) (*types.QueryPendingRewardsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	reward := k.GetPendingRewards(ctx, req.Claimer)

	return &types.QueryPendingRewardsResponse{Amount: reward.Amount.String()}, nil
}
