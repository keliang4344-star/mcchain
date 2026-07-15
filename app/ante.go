package app

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// MinSelfDelegationLowerBound is the chain-wide minimum self delegation enforced
// for every validator, expressed in the base denom (umc).
//
//	1e11 umc == 100_000 MC == 100k MC
//
// It is the single source of truth shared by the ante decorator (transaction
// path) and the InitChainer fallback (genesis validators, which bypass the ante
// chain).
const MinSelfDelegationLowerBound = 100_000_000_000 // umc = 100k MC

// MinSelfDelegationDecorator enforces a global minimum self delegation for
// validators. Cosmos SDK v0.47 has no built-in global floor, so we inspect
// every transaction's messages and reject MsgCreateValidator / MsgEditValidator
// whose MinSelfDelegation is below the chain-wide bound.
//
// Genesis validators are created directly by InitGenesis (not via tx) and are
// therefore handled separately in App.InitChainer.
type MinSelfDelegationDecorator struct{}

var _ sdk.AnteDecorator = MinSelfDelegationDecorator{}

// AnteHandle implements sdk.AnteDecorator.
func (msd MinSelfDelegationDecorator) AnteHandle(
	ctx sdk.Context,
	tx sdk.Tx,
	simulate bool,
	next sdk.AnteHandler,
) (sdk.Context, error) {
	for _, msg := range tx.GetMsgs() {
		switch m := msg.(type) {
		case *stakingtypes.MsgCreateValidator:
			if m.MinSelfDelegation.LT(sdk.NewInt(MinSelfDelegationLowerBound)) {
				return ctx, sdkerrors.Wrapf(
					sdkerrors.ErrInvalidRequest,
					"min self delegation %s < lower bound %d umc",
					m.MinSelfDelegation.String(),
					MinSelfDelegationLowerBound,
				)
			}
		case *stakingtypes.MsgEditValidator:
			// A zero/min-self-delegation value means "do not change"; the field
			// is only validated when it is actually set (non-nil pointer and non-nil underlying).
			// 注意：m.MinSelfDelegation 是 *sdk.Int，nil 指针时不能直接调 .IsNil()（会解引用 panic）。
			if m.MinSelfDelegation != nil && !m.MinSelfDelegation.IsNil() && m.MinSelfDelegation.IsPositive() {
				if m.MinSelfDelegation.LT(sdk.NewInt(MinSelfDelegationLowerBound)) {
					return ctx, sdkerrors.Wrapf(
						sdkerrors.ErrInvalidRequest,
						"min self delegation %s < lower bound %d umc",
						m.MinSelfDelegation.String(),
						MinSelfDelegationLowerBound,
					)
				}
			}
		}
	}
	return next(ctx, tx, simulate)
}
