package keeper

import (
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TrackDepinReward is called from the depin module after a DePIN task payout
// to credit the submitter's inviter with a referral bonus.
//
// Parameters:
//   - submitter:  the address of the user who earned the depin reward
//   - rewardAmount:  reward amount in umc (smallest denom)
func (k Keeper) TrackDepinReward(ctx sdk.Context, submitter string, rewardAmount sdkmath.Int) error {
	return k.TrackReward(ctx, submitter, rewardAmount)
}

// TrackEdgeAIReward is called from the edgeai module after an EdgeAI task
// settlement to credit the submitter's inviter with a referral bonus.
//
// Parameters:
//   - submitter:  the address of the user who earned the edgeai reward
//   - rewardAmount:  reward amount in umc (smallest denom)
func (k Keeper) TrackEdgeAIReward(ctx sdk.Context, submitter string, rewardAmount sdkmath.Int) error {
	return k.TrackReward(ctx, submitter, rewardAmount)
}
