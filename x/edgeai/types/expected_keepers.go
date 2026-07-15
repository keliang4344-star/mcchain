package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// AccountKeeper 最小账户接口（与 cosmos-sdk x/simulation.AccountKeeper 一致）。
type AccountKeeper interface {
	GetAccount(ctx sdk.Context, addr sdk.AccAddress) authtypes.AccountI
}

// 跨模块依赖边界说明（C1）：
//   - EdgeAI 自身不持 Minter、不直接 mint（B1 总量 cap 由 tokenomics 唯一掌控）。
//   - 贡献奖励统一经 PayoutKeeper（由 depin 模块实现）从 depin 模块账户出币；
//     "谁出币"= depin 模块账户，"谁记账"= tokenomics 的 minted_supply 不因此变化。
//   - phonenode 仅提供认证闸口与作弊 slash 钩子，不参与出币。

// PhonenodeKeeper B3 depends on phonenode for attestation check + slash hook.
type PhonenodeKeeper interface {
	HasNode(ctx sdk.Context, addr string) bool
	IsAttested(ctx sdk.Context, addr string) bool
	// SlashIfBad is used for anti-cheat punishment when a dispute resolves as cheat.
	SlashIfBad(ctx sdk.Context, addr, reason string, penaltyBps uint32) error
}

// BankKeeper defines the expected bank keeper (for module account operations).
// 需求方付费（escrow）模型下，EdgeAI 经 bankKeeper 完成：
//   - SendCoinsFromAccountToModule：任务创建时由 creator 向 edgeai 模块账户托管 reward；
//   - SendCoinsFromModuleToAccount：BeginBlock 结算时由 edgeai 模块账户向 submitter 拨付；
//   - SpendableCoins：创建任务前校验 creator 余额是否足以托管。
type BankKeeper interface {
	SpendableCoins(ctx sdk.Context, addr sdk.AccAddress) sdk.Coins
	SendCoinsFromAccountToModule(ctx sdk.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	SendCoinsFromModuleToAccount(ctx sdk.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
}

// PayoutKeeper 历史支付接口（B3.1 R4）：原设计由 depin 模块账户出币（受 B1 总量 cap 约束）。
// 自"需求方付费（escrow）"改造后，EdgeAI 拨付改经 bankKeeper 从 edgeai 模块账户出币，
// 不再调用 PayoutReward；此接口保留以维持既有接线兼容，不再被拨付路径使用。
type PayoutKeeper interface {
	PayoutReward(ctx sdk.Context, addr sdk.AccAddress, amount uint64) error
}
