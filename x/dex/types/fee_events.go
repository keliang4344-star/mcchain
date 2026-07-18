package types

// Fee distribution event types and attribute keys.
const (
	// EventTypeFeeDistribution is emitted when swap fees are distributed.
	EventTypeFeeDistribution = "dex.fee_distribution"

	// Attribute keys for FeeDistribution events.
	AttrKeyFeeAmount    = "fee_amount"
	AttrKeyFeeBurned    = "burned"
	AttrKeyFeeToLP      = "to_lp"
	AttrKeyFeeToTreasury = "to_treasury"
	AttrKeyFeeDenom     = "fee_denom"
	AttrKeyPoolID       = "pool_id"
)

// CommunityModuleName is the module account name for the protocol treasury.
// Swap fees (30%) are sent here as protocol revenue.
const CommunityModuleName = "community"
