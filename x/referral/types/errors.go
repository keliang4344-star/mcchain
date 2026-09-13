package types

import (
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var (
	ErrReferralNotFound          = sdkerrors.Register(ModuleName, 1600, "referral not found")
	ErrInviteeAlreadyReferred    = sdkerrors.Register(ModuleName, 1601, "invitee already referred by another inviter")
	ErrSelfReferral              = sdkerrors.Register(ModuleName, 1602, "cannot refer yourself")
	ErrInviteeNotRegistered      = sdkerrors.Register(ModuleName, 1603, "invitee is not a registered phonenode")
	ErrCooldownNotElapsed        = sdkerrors.Register(ModuleName, 1604, "referral cooldown has not elapsed")
	ErrMaxReferralsReached       = sdkerrors.Register(ModuleName, 1605, "inviter has reached max referrals")
	ErrNoPendingRewards          = sdkerrors.Register(ModuleName, 1606, "no pending rewards to claim")
	ErrBelowMinPayout            = sdkerrors.Register(ModuleName, 1607, "pending rewards below minimum payout threshold")
	ErrInvalidInviteCode         = sdkerrors.Register(ModuleName, 1608, "invalid invite code")
	ErrEcosystemPoolInsufficient = sdkerrors.Register(ModuleName, 1609, "ecosystem pool insufficient funds")
	ErrReferralCycle             = sdkerrors.Register(ModuleName, 1610, "referral would create a cycle (referred address is an ancestor of the referrer)")

	// ErrReferralBudgetExhausted 推荐返佣生态预算已耗尽。
	//
	// 之所以要有独立错误码：预算耗尽时的失败会来自 bank 的
	// SendCoinsFromModuleToAccount（余额不足），前端与客服拿到的是一个通用
	// 转账错误，无法区分「参数写错」与「预算真的花完了」。独立错误码让这个
	// 状态可以被明确提示、被监控按码告警。
	ErrReferralBudgetExhausted = sdkerrors.Register(ModuleName, 1611, "referral ecosystem budget exhausted")

	// ErrInvalidReleaseSchedule 释放节奏配置非法。
	ErrInvalidReleaseSchedule = sdkerrors.Register(ModuleName, 1612, "invalid referral release schedule")
)
