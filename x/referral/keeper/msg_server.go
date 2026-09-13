package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/referral/types"
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

// CreateReferral handles MsgCreateReferral.
// It validates the referral request and persists a new referral record on-chain.
func (m msgServer) CreateReferral(goCtx context.Context, msg *types.MsgCreateReferral) (*types.MsgCreateReferralResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if msg.Inviter == "" {
		return nil, types.ErrSelfReferral // reuse: inviter cannot be empty
	}
	if msg.Invitee == "" {
		return nil, types.ErrInviteeNotRegistered // reuse: invitee must be valid
	}

	referralID, err := m.Keeper.CreateReferral(ctx, msg.Inviter, msg.Invitee, msg.InviteCode)
	if err != nil {
		return nil, err
	}

	return &types.MsgCreateReferralResponse{ReferralId: referralID}, nil
}

// ClaimReferralReward handles MsgClaimReferralReward.
// It pays out accumulated pending rewards from the ecosystem module account.
func (m msgServer) ClaimReferralReward(goCtx context.Context, msg *types.MsgClaimReferralReward) (*types.MsgClaimReferralRewardResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if msg.Claimer == "" {
		return nil, types.ErrNoPendingRewards // reuse: claimer cannot be empty
	}

	reward, err := m.Keeper.ClaimRewards(ctx, msg.Claimer)
	if err != nil {
		return nil, err
	}

	return &types.MsgClaimReferralRewardResponse{Amount: reward.Amount.String()}, nil
}
