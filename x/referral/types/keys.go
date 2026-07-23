package types

const (
	ModuleName   = "referral"
	StoreKey     = ModuleName
	RouterKey    = ModuleName
	MemStoreKey  = "mem_" + ModuleName

	// ---- 存储前缀 ----
	ReferralKeyPrefix        = "Referral/value/"
	ReferralCountKeyPrefix   = "Referral/count/"
	ReferralByInviterPrefix  = "Referral/inviter/"
	ReferralByInviteePrefix  = "Referral/invitee/"
	PendingRewardKeyPrefix   = "PendingReward/value/"
	RewardPoolKey            = "RewardPool"

	// ---- 推荐状态 ----
	ReferralStatusActive  = "active"
	ReferralStatusClaimed = "claimed"

	// ---- 默认参数（白皮书 §25）----
	DefaultLevel1RewardRateBps uint32 = 1000           // 一代 10%
	DefaultLevel2RewardRateBps uint32 = 500            // 二代 5%
	DefaultLevel3RewardRateBps uint32 = 200            // 三代 2%
	DefaultMinPayout                  = "100000000"     // 100 MC = 100,000,000 umc
	DefaultMaxReferralsPerUser uint64 = 100
	DefaultCooldownBlocks      uint64 = 100
	DefaultDailyPerUserCap     uint64 = 500000000       // 500 MC
	DefaultDailyNetworkCap     uint64 = 20600000000     // 20,600 MC
	MaxReferralDepth           uint32 = 3               // 最多三代

	// ---- 日熔断存储键 ----
	DailyCapKeyPrefix         = "DailyCap/value/"
	DailyPerUserCapKeyPrefix  = "DailyCap/peruser/"
	DailyNetworkCapKey        = "DailyCap/network/"

	// ---- 生态基金模块账户 ----
	EcosystemModuleAccount = "ecosystem"
)

func KeyPrefix(p string) []byte {
	return []byte(p)
}
