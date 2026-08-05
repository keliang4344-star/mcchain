package types

// Event types emitted by x/liquidstaking.
const (
	EventTypeLiquidStake   = "liquid_stake"
	EventTypeLiquidUnstake = "liquid_unstake"
	EventTypeLiquidClaim   = "liquid_claim"
	EventTypeRewardsAccrue = "liquid_rewards_accrue"
	EventTypeSlashApplied  = "liquid_slash_applied"

	AttributeKeyDelegator     = "delegator"
	AttributeKeyValidator     = "validator"
	AttributeKeyAmountUmc     = "amount_umc"
	AttributeKeySharesUlmc    = "shares_ulmc"
	AttributeKeyUnbondingID   = "unbonding_id"
	AttributeKeyCompletionAt  = "completion_at"
	AttributeKeyExchangeRate  = "exchange_rate"
	AttributeKeySlashFraction = "slash_fraction"
)
