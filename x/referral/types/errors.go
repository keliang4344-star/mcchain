package types

import (
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var (
	ErrReferralNotFound         = sdkerrors.Register(ModuleName, 1600, "referral not found")
	ErrInviteeAlreadyReferred   = sdkerrors.Register(ModuleName, 1601, "invitee already referred by another inviter")
	ErrSelfReferral             = sdkerrors.Register(ModuleName, 1602, "cannot refer yourself")
	ErrInviteeNotRegistered     = sdkerrors.Register(ModuleName, 1603, "invitee is not a registered phonenode")
	ErrCooldownNotElapsed       = sdkerrors.Register(ModuleName, 1604, "referral cooldown has not elapsed")
	ErrMaxReferralsReached      = sdkerrors.Register(ModuleName, 1605, "inviter has reached max referrals")
	ErrNoPendingRewards         = sdkerrors.Register(ModuleName, 1606, "no pending rewards to claim")
	ErrBelowMinPayout           = sdkerrors.Register(ModuleName, 1607, "pending rewards below minimum payout threshold")
	ErrInvalidInviteCode        = sdkerrors.Register(ModuleName, 1608, "invalid invite code")
	ErrEcosystemPoolInsufficient = sdkerrors.Register(ModuleName, 1609, "ecosystem pool insufficient funds")
)
