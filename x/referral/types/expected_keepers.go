package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// AccountKeeper defines the expected auth keeper surface needed by the
// referral module and its simulation layer.
type AccountKeeper interface {
	GetAccount(ctx sdk.Context, addr sdk.AccAddress) authtypes.AccountI
}

// BankKeeper defines the expected bank keeper interface for the referral module.
// The referral module needs:
//   - SendCoinsFromModuleToAccount: to pay rewards from the ecosystem module account to inviters
//   - GetBalance: to check ecosystem pool balance
//
// 接口不含 MintCoins / BurnCoins：推荐奖励 100% 足额发给推荐人，
// 「领取时销毁 1%」已按白皮书《优化定稿版》§24.6 否决清单撤销，
// referral 模块既不新印也不销毁，从类型层面物理禁止。
type BankKeeper interface {
	SpendableCoins(ctx sdk.Context, addr sdk.AccAddress) sdk.Coins
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
