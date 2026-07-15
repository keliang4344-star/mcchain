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

// BankKeeper defines the expected interface needed to retrieve account balances.
type BankKeeper interface {
	SpendableCoins(ctx sdk.Context, addr sdk.AccAddress) sdk.Coins
	// Methods imported from bank should be defined here
}

// StakingKeeper B2 slashing 所需的 staking 接口子集（仅取方法签名，避免 x/phonenode 依赖具体实现）。
type StakingKeeper interface {
	// Validator returns the validator with the given operator address (ValAddress).
	Validator(ctx sdk.Context, addr sdk.ValAddress) stakingtypes.ValidatorI
	// ValidatorByConsAddr returns the validator with the given consensus address.
	ValidatorByConsAddr(ctx sdk.Context, consAddr sdk.ConsAddress) stakingtypes.ValidatorI
}

// SlashingKeeper B2 slashing 所需的 slashing 接口子集。slash 一律走 staking.Slash/Jail，
// 绝不调用 MintCoins（B2 铁律：slash 不破 B1 cap）。
type SlashingKeeper interface {
	// Slash slashes a validator for the given fraction (bps/10000) and power proxy.
	Slash(ctx sdk.Context, consAddr sdk.ConsAddress, fraction sdk.Dec, power, distributionHeight int64)
	// Jail jails a validator by its consensus address.
	Jail(ctx sdk.Context, consAddr sdk.ConsAddress)
}
