package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
)

// AccountKeeper defines the expected account keeper used for simulations (noalias)
type AccountKeeper interface {
	GetAccount(ctx sdk.Context, addr sdk.AccAddress) types.AccountI
	// Methods imported from account should be defined here
}

// BankKeeper defines the expected interface needed to retrieve account balances
// and move module coins.
//
// Q7：depin 不再自铸，故 BankKeeper 接口移除 MintCoins；奖励仅从生态池拨付的
// InitialPool（已转至 depin 模块账户）经 SendCoinsFromModuleToAccount 对外拨付。
// BurnCoins 用于 5% 销毁通道（通缩飞轮）。
type BankKeeper interface {
	SpendableCoins(ctx sdk.Context, addr sdk.AccAddress) sdk.Coins
	// SendCoinsFromModuleToAccount 从 DePIN 模块账户向贡献设备拨付奖励（方案 A：DePIN 池拨付，不铸造）。
	// 签名必须与 cosmos-sdk v0.47.3 的 bank.BaseKeeper 一致，否则 app 装配阶段编译失败。
	SendCoinsFromModuleToAccount(ctx sdk.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	// BurnCoins 从模块账户销毁代币（5% DePIN 通缩飞轮）。
	BurnCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error
	// Methods imported from bank should be defined here
}

// PhonenodeKeeper defines the minimal surface of the phonenode module that the
// depin module depends on. Only sdk types are used on purpose, so that
// x/depin/types does NOT import x/phonenode/types (which would create an
// import cycle). The association key is the node Address, which equals the
// depin device address (SubmitContribution.Creator).
type PhonenodeKeeper interface {
	// HasNode reports whether a mobile node with the given address is registered.
	HasNode(ctx sdk.Context, addr string) bool
	// IsAttested reports whether the node holds a currently valid attestation (B2 反女巫)。
	IsAttested(ctx sdk.Context, addr string) bool
}
