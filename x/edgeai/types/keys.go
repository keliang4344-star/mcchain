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

// CheatSlashBps B3.1：争议裁定作弊时对结果提交者的 slash 基点（10%）。
const CheatSlashBps uint32 = 1000

// EdgeAI reward split ratios (基点, 10000 = 100%):
//   80% → submitter (executor node)
//   15% → verifier reserve (verifier 抽检后领取)
//    5% → burn (通缩飞轮, 永久销毁)
const (
	EdgeAISubmitterRatioBps       uint32 = 8000
	EdgeAIVerifierReserveRatioBps uint32 = 1500
	EdgeAIBurnRatioBps            uint32 = 500
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
