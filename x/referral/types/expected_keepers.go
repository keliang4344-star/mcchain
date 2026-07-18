package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// BankKeeper defines the expected bank keeper interface for the referral module.
// The referral module needs:
//   - SendCoinsFromModuleToAccount: to pay rewards from the ecosystem module account to inviters
//   - GetBalance: to check ecosystem pool balance
type BankKeeper interface {
	SendCoinsFromModuleToAccount(ctx sdk.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	GetBalance(ctx sdk.Context, addr sdk.AccAddress, denom string) sdk.Coin
}

// PhonenodeKeeper defines the expected phonenode keeper interface.
// The referral module uses this to verify that an invitee is a legitimate registered node.
// This is the core anti-sybil protection for the referral system.
type PhonenodeKeeper interface {
	// HasNode returns true if the given address is a registered phonenode.
	HasNode(ctx sdk.Context, addr string) bool
}
