package types

const (
	// ModuleName defines the module name
	ModuleName = "phonenode"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RouterKey defines the module's message routing key
	RouterKey = ModuleName

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_phonenode"
)

func KeyPrefix(p string) []byte {
	return []byte(p)
}

// B2 安全相关 KV key 前缀与构造器。
var (
	// AttestationKeyPrefix 是 attestation 状态存储前缀：AttestationKey(addr) = "Attestation:"+addr
	AttestationKeyPrefix = []byte("Attestation:")
	// SlashRecordKeyPrefix 是某地址 slash 记录列表存储前缀：SlashRecordKey(addr) = "Slash:"+addr
	SlashRecordKeyPrefix = []byte("Slash:")
	// DeviceHashKeyPrefix 是 device_id_hash → 节点地址 的反查索引前缀（防女巫设备绑定）。
	DeviceHashKeyPrefix = []byte("DeviceHash:")
	// NonceKeyPrefix 是 attestation nonce 重放防护索引前缀：NonceKey(addr,nonce) = "Nonce:"+addr+"/"+nonce
	NonceKeyPrefix = []byte("Nonce:")
	// SlashCooldownKeyPrefix 是 slash 后再认证冷却截止高度前缀：SlashCooldownKey(addr) = "SlashCD:"+addr
	SlashCooldownKeyPrefix = []byte("SlashCD:")
)

// AttestationKey 返回某节点的 attestation 状态 key。
func AttestationKey(addr string) []byte {
	return append(AttestationKeyPrefix, []byte(addr)...)
}

// SlashRecordKey 返回某地址的 slash 记录列表 key。
func SlashRecordKey(addr string) []byte {
	return append(SlashRecordKeyPrefix, []byte(addr)...)
}

// DeviceHashKey 返回 device_id_hash 反查索引 key。
func DeviceHashKey(deviceIDHash string) []byte {
	return append(DeviceHashKeyPrefix, []byte(deviceIDHash)...)
}

// NonceKey 返回某节点某 nonce 的重放防护索引 key。bech32 地址不含 "/"，故用 "/" 作分隔安全。
func NonceKey(addr, nonce string) []byte {
	return append(append(NonceKeyPrefix, []byte(addr)...), []byte("/"+nonce)...)
}

// SlashCooldownKey 返回某节点 slash 冷却截止高度 key（B2 非验证人细则）。
func SlashCooldownKey(addr string) []byte {
	return append(SlashCooldownKeyPrefix, []byte(addr)...)
}

// VerifierStatusKey 返回节点验证者状态 key。
func VerifierStatusKey(nodeID string) []byte {
	return append(VerifierStatusKeyPrefix, []byte(nodeID)...)
}
