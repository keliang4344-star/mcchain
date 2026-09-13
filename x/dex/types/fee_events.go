package types

// Fee distribution event types and attribute keys.
const (
	// EventTypeFeeDistribution is emitted when swap fees are distributed.
	EventTypeFeeDistribution = "dex.fee_distribution"

	// Attribute keys for FeeDistribution events.
	AttrKeyFeeAmount     = "fee_amount"
	AttrKeyFeeBurned     = "burned"
	AttrKeyFeeToLP       = "to_lp"
	AttrKeyFeeToTreasury = "to_treasury"
	AttrKeyFeeDenom      = "fee_denom"
	AttrKeyPoolID        = "pool_id"
)

// CommunityModuleName is the module account name for the protocol treasury.
// Swap fees (30%) are sent here as protocol revenue.
const CommunityModuleName = "community"

// ---------------------------------------------------------------------------
// 结算通道的显式状态事件
// ---------------------------------------------------------------------------

const (
	// EventTypeSettlementEnabled 结算通道被开启并指定了运营地址。
	EventTypeSettlementEnabled = "dex.settlement_enabled"
	// EventTypeSettlementHalted 结算通道被熔断。
	EventTypeSettlementHalted = "dex.settlement_halted"
	// EventTypeSettlementDefaulted 创世时结算通道以「未启用」状态落地。
	// 之所以要发事件：该状态若为隐式（Halted=false 但 Authority=治理账户，
	// 谁都无法提交），链上没有任何迹象表明结算不可用。现在状态是显式的、可观测的。
	EventTypeSettlementDefaulted = "dex.settlement_defaulted"

	// AttrSettlementAuthority 结算运营地址。
	AttrSettlementAuthority = "authority"
	// AttrReason 事件原因。
	AttrReason = "reason"
)
