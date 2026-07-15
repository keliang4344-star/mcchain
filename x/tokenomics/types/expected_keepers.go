package types

import (
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// AccountKeeper 定义 tokenomics 依赖的账户 keeper 最小接口。
type AccountKeeper interface {
	// GetAccount 返回指定地址的账户（不存在返回 nil）。
	GetAccount(ctx sdk.Context, addr sdk.AccAddress) authtypes.AccountI
	// SetAccount 持久化账户（cosmos-sdk v0.47.3 auth keeper.SetAccount 返回 void）。
	SetAccount(ctx sdk.Context, acc authtypes.AccountI)
	// NewAccountWithAddress 按地址创建新基础账户（分配 account number）。
	NewAccountWithAddress(ctx sdk.Context, addr sdk.AccAddress) authtypes.AccountI
}

// BankKeeper 定义 tokenomics 依赖的 bank keeper 最小接口（移动/铸造模块币）。
type BankKeeper interface {
	// MintCoins 向指定模块账户铸造新币（仅 tokenomics 持有 Minter，Q7）。
	// 签名必须与 cosmos-sdk v0.47.3 bank.BaseKeeper.MintCoins 一致。
	MintCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error
	// SendCoinsFromModuleToAccount 从模块账户向外部地址拨付（团队 vesting 账户）。
	SendCoinsFromModuleToAccount(ctx sdk.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	// SendCoinsFromModuleToModule 模块账户间转账（社区/生态/生态→depin 切片）。
	SendCoinsFromModuleToModule(ctx sdk.Context, senderModule, recipientModule string, amt sdk.Coins) error
	// GetBalance 返回某地址某 denom 的余额（查询 allocations 当前余额）。
	GetBalance(ctx sdk.Context, addr sdk.AccAddress, denom string) sdk.Coin
}
