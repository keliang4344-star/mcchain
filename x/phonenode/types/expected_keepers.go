package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// AccountKeeper defines the expected account keeper used for simulations (noalias)
type AccountKeeper interface {
	GetAccount(ctx sdk.Context, addr sdk.AccAddress) types.AccountI
	// Methods imported from account should be defined here
}

// BankKeeper defines the expected interface needed to retrieve account balances
// and to route slashed stake. R1 iron rule: this module MUST NOT mint MC —
// slashed stake is only moved by transfer (bonded pool → black hole address for
// the 40% burn share, bonded pool → staking-security pool for the 60% share),
// never created. MintCoins is intentionally absent from this interface so the
// code physically cannot print new supply. BurnCoins is absent as well: burning
// is expressed chain-wide as a transfer to the canonical black hole address.
type BankKeeper interface {
	SpendableCoins(ctx sdk.Context, addr sdk.AccAddress) sdk.Coins
	// GetBalance returns the balance of denom held by addr.
	GetBalance(ctx sdk.Context, addr sdk.AccAddress, denom string) sdk.Coin
	// SendCoinsFromModuleToModule moves coins from one module account to another.
	// Used to route the 60% share of slashed stake from the staking bonded pool
	// into the staking-security pool (a transfer, not a mint and not a burn).
	SendCoinsFromModuleToModule(ctx sdk.Context, senderModule, recipientModule string, amt sdk.Coins) error
	// SendCoinsFromModuleToAccount sends coins from a module account to an address.
	// Used to route the 40% burn share of slashed stake from the staking bonded
	// pool to the canonical black hole address (permanently unspendable).
	SendCoinsFromModuleToAccount(ctx sdk.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
}

// StakingKeeper B2 slashing 所需的 staking 接口子集（仅取方法签名，避免 x/phonenode 依赖具体实现）。
//
// 为把「罚没 40% 销毁 / 60% 回流质押安全池」落地，本模块不再使用会把被罚金额
// 100% 烧毁、且去向不可拆分的 slashing.Slash，而是自行完成等价动作：
// 触发 distribution 的 BeforeValidatorSlashed 钩子、扣减验证人 tokens、
// 再把等额资金从 bonded pool 按 40/60 分别路由到黑洞地址与质押安全池。
type StakingKeeper interface {
	// Validator returns the validator with the given operator address (ValAddress).
	Validator(ctx sdk.Context, addr sdk.ValAddress) stakingtypes.ValidatorI
	// ValidatorByConsAddr returns the validator with the given consensus address.
	ValidatorByConsAddr(ctx sdk.Context, consAddr sdk.ConsAddress) stakingtypes.ValidatorI
	// BondDenom returns the staking bond denom, used to denominate routed slash coins.
	BondDenom(ctx sdk.Context) string
	// GetValidator returns the concrete validator record (needed to mutate tokens).
	GetValidator(ctx sdk.Context, addr sdk.ValAddress) (stakingtypes.Validator, bool)
	// RemoveValidatorTokens deducts tokens from a validator and refreshes its
	// power index. Unlike staking.Slash it does NOT burn the bonded pool coins,
	// which is exactly what lets us redirect them to the security pool.
	RemoveValidatorTokens(ctx sdk.Context, validator stakingtypes.Validator, tokensToRemove sdk.Int) stakingtypes.Validator
	// Hooks exposes the staking hooks so that a custom slash still notifies
	// distribution (BeforeValidatorSlashed), keeping delegator reward accounting
	// correct — omitting this would let delegators over-withdraw after a slash.
	Hooks() stakingtypes.StakingHooks
}

// SlashingKeeper B2 所需的 slashing 接口子集，只保留 Jail（吊销出块权）。
//
// 这里刻意不暴露 slashing.Slash：SDK v0.47 的该方法会把被罚金额 100% 无条件
// 烧毁、去向不可拆分，与「40% 销毁 / 60% 回流质押安全池、绝不新印」的定稿口径冲突。
// 把它挡在接口之外，本模块在编译期就不具备「直接烧币」的能力，销毁只能经由
// 黑洞地址转账这一条可审计通道完成（同理，BankKeeper 亦不含 MintCoins 与 BurnCoins）。
type SlashingKeeper interface {
	// Jail jails a validator by its consensus address.
	Jail(ctx sdk.Context, consAddr sdk.ConsAddress)
}
