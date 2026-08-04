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
// and to route the slash treasury share. R1 iron rule: this module MUST NOT mint
// MC — the treasury share is sourced by transferring from an existing reserve pool
// (staking-security), never by MintCoins. MintCoins is intentionally absent from
// this interface so the code physically cannot print new supply.
type BankKeeper interface {
	SpendableCoins(ctx sdk.Context, addr sdk.AccAddress) sdk.Coins
	// GetBalance returns the balance of denom held by addr. Used to cap the
	// treasury transfer at the security pool's available balance.
	GetBalance(ctx sdk.Context, addr sdk.AccAddress, denom string) sdk.Coin
	// SendCoinsFromModuleToModule moves coins from one module account to another.
	// Used to route the slash treasury share from the staking-security pool to the
	// protocol treasury (a transfer, not a mint).
	SendCoinsFromModuleToModule(ctx sdk.Context, senderModule, recipientModule string, amt sdk.Coins) error
	// BurnCoins burns coins from a module account. Retained for interface symmetry;
	// the actual slash burn is performed by staking.Slash, not by this keeper.
	BurnCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error
	// SendCoinsFromModuleToAccount sends coins from a module account to an address.
	SendCoinsFromModuleToAccount(ctx sdk.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
}

// StakingKeeper B2 slashing 所需的 staking 接口子集（仅取方法签名，避免 x/phonenode 依赖具体实现）。
type StakingKeeper interface {
	// Validator returns the validator with the given operator address (ValAddress).
	Validator(ctx sdk.Context, addr sdk.ValAddress) stakingtypes.ValidatorI
	// ValidatorByConsAddr returns the validator with the given consensus address.
	ValidatorByConsAddr(ctx sdk.Context, consAddr sdk.ConsAddress) stakingtypes.ValidatorI
	// BondDenom returns the staking bond denom, used to denominate routed slash coins.
	BondDenom(ctx sdk.Context) string
}

// SlashingKeeper B2 slashing 所需的 slashing 接口子集。slash 一律走 staking.Slash/Jail，
// 绝不调用 MintCoins（B2 铁律：slash 不破 B1 cap）。
type SlashingKeeper interface {
	// Slash slashes a validator for the given fraction (bps/10000) and power proxy.
	Slash(ctx sdk.Context, consAddr sdk.ConsAddress, fraction sdk.Dec, power, distributionHeight int64)
	// Jail jails a validator by its consensus address.
	Jail(ctx sdk.Context, consAddr sdk.ConsAddress)
}
