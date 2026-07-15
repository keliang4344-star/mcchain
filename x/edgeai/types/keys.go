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
	TaskStatusCheated  = "cheated" // B3.1：仲裁裁定作弊，拒绝拨付
)

// CheatSlashBps B3.1：争议裁定作弊时对结果提交者的 slash 基点（10%）。
const CheatSlashBps uint32 = 1000

// Result status values
const (
	ResultStatusPending  = "pending"
	ResultStatusValid    = "valid"
	ResultStatusRejected = "rejected"
)
