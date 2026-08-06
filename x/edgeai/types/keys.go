package types

const (
	// ModuleName defines the module name
	ModuleName = "edgeai"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RouterKey defines the module's message routing key
	RouterKey = ModuleName

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_edgeai"

	// EdgeAIDenom 任务奖励计价单位（与全链一致：umc）。
	EdgeAIDenom = "umc"
)

func KeyPrefix(p string) []byte {
	return []byte(p)
}

// Task status values
const (
	TaskStatusOpen     = "open"
	TaskStatusAssigned = "assigned"
	TaskStatusDone     = "done"
	TaskStatusDisputed = "disputed"
	TaskStatusCheated  = "cheated"  // B3.1：仲裁裁定作弊，拒绝拨付
	TaskStatusExpired  = "expired" // 任务超时过期，退还 escrow 给创建者
)

// BeginBlock 结算限流常量
const (
	// TaskExpireBlocks 任务最大存活区块数，超时未结算自动过期并退还托管金。
	TaskExpireBlocks uint64 = 10000
	// MaxTasksPerBlock 每区块最多结算任务数，防止 BeginBlock 过重阻塞出块。
	MaxTasksPerBlock uint64 = 20
)

// ---------------------------------------------------------------------------
// SCALE-1：BeginBlock 有界扫描上限
//
// 硬约束：BeginBlock / EndBlock 内的任何遍历都必须是 O(常数)，
// 绝不允许出现 O(全量任务) / O(全量结果) / O(全量节点) 的整库扫描——
// 在 5.5 亿设备的目标规模下，一次全量扫描即可让出块超时、全网停摆。
//
// 实现方式：为「待处理集合」建独立索引，并配持久化游标做轮转扫描，
// 每区块只消费固定预算，游标随状态一同进入 AppHash，全网确定性一致。
// ---------------------------------------------------------------------------
const (
	// MaxPendingResultScanPerBlock 每区块从 pending 结果索引中最多消费的条目数
	// （硬上限，防止单个任务挂载超多提交者时预算被击穿）。
	MaxPendingResultScanPerBlock int = 512

	// MaxOpenTaskScanPerBlock 每区块从 open 任务索引中最多检查的任务数（过期回收）。
	MaxOpenTaskScanPerBlock int = 128

	// MaxReputationScanPerBlock 每区块最多检查的声誉记录数（衰减轮转）。
	MaxReputationScanPerBlock int = 128

	// DoneTaskRingSize 「近期已完成任务」环形缓冲容量，供验证者抽检采样。
	// 抽检的意义在于覆盖新近完成的任务，无需对历史全库采样，故以定长环替代全表扫描。
	DoneTaskRingSize uint64 = 256
)

// CheatSlashBps B3.1：争议裁定作弊时对结果提交者的 slash 基点（10%）。
const CheatSlashBps uint32 = 1000

// EdgeAI reward split ratios (基点, 10000 = 100%):
//   85% → submitter (executor node，执行节点足额到手)
//   15% → verifier reserve (verifier 抽检后领取)
//
// 【已撤销】原 5% 结算销毁（EdgeAIBurnRatioBps）：按白皮书《优化定稿版》§24.6
// 否决清单撤销，该 5% 已并入提交者份额（80% → 85%）。任务奖励是需求方托管的
// 真实付费，全额属于完成计算的节点与核验者，不承担通缩职能；全链通缩只来自
// 协议使用费（gas 7%、DEX 手续费的 50%）与作恶罚没（40%）。
// 常量已删除而非置零，以从编译层面杜绝该逻辑被重新引用。
const (
	EdgeAISubmitterRatioBps       uint32 = 8500
	EdgeAIVerifierReserveRatioBps uint32 = 1500
)

// Verifier constants
const (
	// VerifierRewardPerSample is the reward paid to a verifier node for each
	// successful verification sampling (1 MC = 1000000 umc).
	// TODO: promote to a proper on-chain param in a future proto update.
	VerifierRewardPerSample uint64 = 1000000

	// MaxVerificationsPerBlock caps verifier sampling per BeginBlock to prevent
	// excessive computation.
	MaxVerificationsPerBlock uint64 = 5
)

// Verification status values
const (
	VerificationStatusAssigned  = "assigned"
	VerificationStatusCompleted = "completed"
)

// Result status values
const (
	ResultStatusPending  = "pending"
	ResultStatusValid    = "valid"
	ResultStatusRejected = "rejected"
)

// Multi-Verifier Scoring constants (白皮书行 496-497)。
// TODO: 待 protoc 重新生成 params.pb.go 后可迁移为链上参数。
const (
	// DefaultVerifierCount 多验证者投票评分系统中每轮抽检的验证者数量。
	DefaultVerifierCount uint32 = 3

	// DefaultThresholdScore 验证评分阈值（0-100），中位数低于此值则拒绝并进入争议。
	DefaultThresholdScore uint32 = 30

	// DefaultScriptTimeout 验证脚本默认超时时间（秒）。
	DefaultScriptTimeout uint32 = 30

	// DefaultScriptNetworkAllowed 验证脚本是否默认允许网络访问。
	DefaultScriptNetworkAllowed bool = false
)

// Node Reputation constants (白皮书行 497)。
const (
	// DefaultReputationScore 新节点的初始声誉分数。
	DefaultReputationScore uint32 = 100

	// MaxReputationScore 声誉分数上限。
	MaxReputationScore uint32 = 100

	// MinReputationScore 声誉分数下限。
	MinReputationScore uint32 = 0

	// ReputationPassIncrease 任务通过时声誉增加量。
	ReputationPassIncrease uint32 = 1

	// ReputationCheatDecrease 任务拒绝/作弊时声誉减少量。
	ReputationCheatDecrease uint32 = 10

	// ReputationFrivolousDecrease 第二验证层重算结果与原始一致（质疑不成立）时，
	// 对挑战方声誉的轻度扣减量（误告惩戒）。
	ReputationFrivolousDecrease uint32 = 3

	// ReputationLowPriorityThreshold 声誉低于此值的节点限制接单优先级。
	ReputationLowPriorityThreshold uint32 = 30

	// ReputationDecayBlocks 连续无贡献多少区块后触发声誉衰减（-1）。
	ReputationDecayBlocks int64 = 1000
)
