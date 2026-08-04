package types

import sdkerrors "cosmossdk.io/errors"

// x/liquidstaking sentinel errors. Code space starts at 1201 to stay clear of
// the ranges already used by depin / phonenode / edgeai / dex / referral.
var (
	ErrModuleDisabled     = sdkerrors.Register(ModuleName, 1201, "liquid staking is disabled by governance")
	ErrInvalidDenom       = sdkerrors.Register(ModuleName, 1202, "invalid denomination")
	ErrBelowMinimumStake  = sdkerrors.Register(ModuleName, 1203, "amount is below the minimum liquid stake")
	ErrValidatorNotFound  = sdkerrors.Register(ModuleName, 1204, "validator not found")
	ErrValidatorJailed    = sdkerrors.Register(ModuleName, 1205, "validator is jailed")
	ErrValidatorCapExceed = sdkerrors.Register(ModuleName, 1206, "validator would exceed the per-validator stake cap")
	ErrInsufficientShares = sdkerrors.Register(ModuleName, 1207, "insufficient liquid shares")
	ErrEmptyPool          = sdkerrors.Register(ModuleName, 1208, "liquid staking pool is empty")
	ErrNothingToClaim     = sdkerrors.Register(ModuleName, 1209, "no matured unbonding receipt to claim")
	ErrInvalidAddress     = sdkerrors.Register(ModuleName, 1210, "invalid address")
	ErrDustRedemption     = sdkerrors.Register(ModuleName, 1211, "redemption rounds down to zero")
)
