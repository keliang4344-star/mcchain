package types

const (
	ModuleName  = "referral"
	StoreKey    = ModuleName
	RouterKey   = ModuleName
	MemStoreKey = "mem_" + ModuleName

	// ---- 存储前缀 ----
	ReferralKeyPrefix       = "Referral/value/"
	ReferralCountKeyPrefix  = "Referral/count/"
	ReferralByInviterPrefix = "Referral/inviter/"
	ReferralByInviteePrefix = "Referral/invitee/"
	PendingRewardKeyPrefix  = "PendingReward/value/"
	RewardPoolKey           = "RewardPool"

	// ---- 推荐状态 ----
	ReferralStatusActive  = "active"
	ReferralStatusClaimed = "claimed"

	// ---- 默认参数（白皮书 §25，10 代分级推荐）----
	// 权重表：10%/5%/3%/2%/1%/0.5%×5 = 23.5%（网络获客成本封顶线）。
	// 前 3 档进治理 Params（Level1-3RewardRateBps），第 4-10 档为链上常量
	// （Level4-10RewardRateBps）；挖矿节点享满 10 代，非节点只享前 5 代。
	DefaultLevel1RewardRateBps  uint32 = 1000        // 一代 10%
	DefaultLevel2RewardRateBps  uint32 = 500         // 二代 5%
	DefaultLevel3RewardRateBps  uint32 = 300         // 三代 3%（10 代模型权重）
	DefaultLevel4RewardRateBps  uint32 = 200         // 四代 2%
	DefaultLevel5RewardRateBps  uint32 = 100         // 五代 1%
	DefaultLevel6RewardRateBps  uint32 = 50          // 六代 0.5%
	DefaultLevel7RewardRateBps  uint32 = 50          // 七代 0.5%
	DefaultLevel8RewardRateBps  uint32 = 50          // 八代 0.5%
	DefaultLevel9RewardRateBps  uint32 = 50          // 九代 0.5%
	DefaultLevel10RewardRateBps uint32 = 50          // 十代 0.5%
	DefaultMinPayout                   = "100000000" // 100 MC = 100,000,000 umc
	DefaultMaxReferralsPerUser  uint64 = 100
	DefaultCooldownBlocks       uint64 = 100
	DefaultDailyPerUserCap      uint64 = 500000000   // 500 MC
	DefaultDailyNetworkCap      uint64 = 20600000000 // 20,600 MC
	MaxReferralDepth            uint32 = 10          // 最多十代（节点享满 10 代）
	// NonNodeMaxDepth 非挖矿节点推荐人最多享受的代际深度（前 5 代）。
	NonNodeMaxDepth uint32 = 5

	// ---- 日熔断存储键 ----
	DailyCapKeyPrefix        = "DailyCap/value/"
	DailyPerUserCapKeyPrefix = "DailyCap/peruser/"
	DailyNetworkCapKey       = "DailyCap/network/"
	// DailyCapPruneCursorKey 记录过期日计数器清理的游标（在 peruser 前缀内的续扫位置）。
	// 清理必须按块预算分片执行，游标保证跨块续扫且不漏键。
	DailyCapPruneCursorKey = "DailyCap/prunecursor"

	// ---- 生态基金模块账户 ----
	EcosystemModuleAccount = "ecosystem"

	// BaseDenom 链上基础记账单位。referral 模块只涉及 umc，既不新印也不销毁。
	BaseDenom = "umc"

	// ---- 返佣预算配速释放 ----
	//
	// ReleaseScheduleKey 存储 ReleaseSchedule（JSON）。用 KVStore + 模块默认值而非
	// proto 字段，避免为运营参数引入 proto 变更。
	ReleaseScheduleKey = "ReleaseSchedule"

	// DefaultReleaseTotalDays 默认释放窗口总天数：4015 天 ≈ 11 年。
	// 与 x/depin 的 DefaultVaultDays 完全一致——返佣预算与挖矿池是同源切分，
	// 必须用同一个窗口，否则两个预算的耗竭时点会再次错位。
	DefaultReleaseTotalDays uint32 = 4015

	// DefaultReleasePhase1Days 冷启动前置投放窗口：首年 365 天。
	DefaultReleasePhase1Days uint32 = 365

	// DefaultReleasePhase1MultiplierBps 冷启动期日上限倍数：200 = 2 倍。
	// 配速项（剩余预算/剩余天数）会钳制总支出，因此加大倍数只改变「何时花」，
	// 不可能超发。
	DefaultReleasePhase1MultiplierBps uint32 = 200

	// ---- 延后账本 ----
	//
	// DeferredRewardPrefix 延后账本键前缀：prefix + 8 字节大端 dayIndex + inviter。
	DeferredRewardPrefix = "DeferredReward/entry/"

	// DeferredRewardTotalKey 延后余额全局计数（可观测 + 参与预算可用额计算）。
	DeferredRewardTotalKey = "DeferredReward/total"

	// PendingRewardTotalKey 全局「已承诺但未领取」返佣总额（umc）。
	//
	// 返佣在计提时只登记负债（pending rewards），
	// 真正扣减生态账户余额发生在 ClaimRewards。因此「账户余额」并不等于
	// 「还能承诺多少」——已经承诺出去的 pending 仍是账户里的钱。
	// 配速上限若只看余额，会把已承诺的钱当成未花掉的钱，在用户不领取的情况下
	// 每天按同一速率继续承诺，累计承诺额可以超过预算总额，最后一批用户
	// 领取时账户已空 → 宣传过的返佣拿不到。
	// 这里维护一个全局计数，让 EffectiveNetworkCap 能用「余额 − 已承诺」做配速基数。
	PendingRewardTotalKey = "PendingReward/total"

	// DeferredDrainBudget 每块补发的最大条目数（成本恒定，防 BeginBlock 超时）。
	DeferredDrainBudget = 64

	// DeferredCountScanBudget 查询延后条数时最多扫描的条目数（防止查询放大）。
	DeferredCountScanBudget = 512

	// ---- 事件与属性名（untyped event，避免为可观测量引入 proto 变更）----
	EventTypeReferralRewardDeferred  = "referral.RewardDeferred"
	EventTypeReferralRewardReleased  = "referral.RewardReleased"
	EventTypeReferralBudgetLow       = "referral.BudgetLow"
	EventTypeReferralBudgetExhausted = "referral.BudgetExhausted"

	AttrInviter   = "inviter"
	AttrAmount    = "amount"
	AttrReason    = "reason"
	AttrDayIndex  = "day_index"
	AttrRemaining = "remaining"
	AttrThreshold = "threshold"

	// DeferReasonDailyCap 延后原因：当日额度已满。
	DeferReasonDailyCap = "daily_network_cap"

	// BudgetLowThresholdBps 预算低水位告警阈值（bps）：剩余预算低于初始预算的
	// 5% 时发出 BudgetLow 事件。
	BudgetLowThresholdBps uint32 = 500

	// InitialEcosystemBudget 推荐返佣生态预算的创世额度（umc）。
	//
	// 必须与 x/tokenomics/types.ReferralEcosystemBudget 完全相等：
	//   82,500,000,000,000 umc = 82.5M MC = 55% 设备激励池的 15%（占总量 8.25%）。
	// 一致性由 x/tokenomics/keeper/genesis.go 的创世不变量校验强制（TOKENOMICS-invariant），
	// 任何一侧被改动而另一侧没跟上，创世会直接失败——避免预算口径再次漂移。
	InitialEcosystemBudget uint64 = 82_500_000_000_000

	// BudgetLowAlertKeyPrefix 低水位告警的按日去重键前缀（避免每块重复发事件）。
	BudgetLowAlertKeyPrefix = "BudgetLowAlert/"
)

func KeyPrefix(p string) []byte {
	return []byte(p)
}
