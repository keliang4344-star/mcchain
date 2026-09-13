package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

type AccountKeeper interface {
	GetModuleAddress(name string) sdk.AccAddress
	GetModuleAccount(ctx sdk.Context, name string) authtypes.ModuleAccountI
	GetAccount(ctx sdk.Context, addr sdk.AccAddress) authtypes.AccountI
}

type BankKeeper interface {
	SpendableCoins(ctx sdk.Context, addr sdk.AccAddress) sdk.Coins
	SendCoinsFromModuleToAccount(ctx sdk.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	SendCoinsFromAccountToModule(ctx sdk.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	SendCoinsFromModuleToModule(ctx sdk.Context, senderModule, recipientModule string, amt sdk.Coins) error
	MintCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error
	BurnCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error
	GetBalance(ctx sdk.Context, addr sdk.AccAddress, denom string) sdk.Coin
	HasBalance(ctx sdk.Context, addr sdk.AccAddress, amt sdk.Coin) bool
}

// DepinReleaseKeeper 设备激励池（depin）的每日线性释放闸门。
//
// LP 激励的资金来源是 depin 模块账户（白皮书 §24），因此它必须与 DePIN 任务
// 奖励、节点资本津贴共用同一个日释放额度闸门；否则 5000 MC/日 会在日释放账本
// 之外悄悄流出，线性释放曲线与真实出账长期对不上账。
type DepinReleaseKeeper interface {
	// CheckDailyReleaseCap 判断本次拨付是否仍在当日释放额度内。
	CheckDailyReleaseCap(ctx sdk.Context, amount uint64) (allowed bool, dailyCap uint64, remaining uint64, err error)
	// RecordDailyRelease 在拨付成功后记账，计入当日已释放与金库累计。
	RecordDailyRelease(ctx sdk.Context, amount uint64)
}
