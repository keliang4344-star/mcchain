package types

const (
	// ModuleName defines the module name
	ModuleName = "depin"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RouterKey defines the module's message routing key
	RouterKey = ModuleName

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_depin"
)

func KeyPrefix(p string) []byte {
	return []byte(p)
}

// AttestationResultKey 返回设备 attestation 结果存储 key。
func AttestationResultKey(deviceID string) []byte {
	return append(KeyPrefix("AttestResult:"), []byte(deviceID)...)
}

// DePINBurnRatioBps defines the burn ratio for DePIN task rewards (基点).
// 500 bps = 5%：每次 DePIN 任务奖励结算时，5% 永久销毁（通缩飞轮），
// 剩余 95% 正常拨付给贡献设备。
// 与 EdgeAI 的 5% 销毁对齐，构成全链统一的通缩压力。
const DePINBurnRatioBps uint32 = 500
