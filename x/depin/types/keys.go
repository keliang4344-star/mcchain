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
