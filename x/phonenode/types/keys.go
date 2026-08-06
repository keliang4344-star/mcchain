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
	// VerifierStatusKeyPrefix 是节点验证者状态存储前缀。
	VerifierStatusKeyPrefix = []byte("VerifierStatus:")
	// DevicePubKeyKeyPrefix 是节点地址 → 设备公钥 的存储前缀（attestation 验签绑定）。
	DevicePubKeyKeyPrefix = []byte("DevPub:")

	// ---- 节点资本津贴（建设溢价）存储键（2026-08 落地）----
	// NodeAllowanceConfigKey 节点资本津贴配置（Enabled / PerDay），存储于模块 KVStore。
	NodeAllowanceConfigKey = []byte("NodeAllowCfg:")
	// NodeAllowanceDayKeyPrefix 记录各节点最近一次领取津贴的「日序号」：NodeAllowDay:<addr>
	NodeAllowanceDayKeyPrefix = []byte("NodeAllowDay:")
	// GlobalLastAllowanceDayKey 全局「当日已分发」标记，避免同日重复遍历。
	GlobalLastAllowanceDayKey = []byte("NodeAllowGlobalDay:")

	// ---- SCALE-1：有界扫描索引与游标 ----
	// HeartbeatIndexKeyPrefix 是「按最近心跳高度排序」的节点索引前缀：
	//   HeartbeatIndexKey(lastProofBlock, addr) = "HbIdx:" + be8(height) + addr
	// 大端编码保证键的字典序等同高度的数值序，离线检测因此只需从索引头部扫到
	// 「高度 < 当前高度 - 宽限期」的边界，命中的都是真正超时的节点，
	// 不必像原实现那样每个区块把全量节点读进内存（5.5 亿设备下即为停块）。
	HeartbeatIndexKeyPrefix = []byte("HbIdx:")
	// OfflineScanCursorKey 离线检测的持久化轮转游标。
	OfflineScanCursorKey = []byte("cursor:offline_scan")
	// AllowanceScanCursorKey 节点资本津贴分发的持久化轮转游标。
	AllowanceScanCursorKey = []byte("cursor:allowance_scan")
)

// SCALE-1：BeginBlock 有界扫描预算。
// 硬约束：BeginBlock 内不得出现 O(全量节点) 的遍历，否则节点规模上量后直接停块。
const (
	// MaxOfflineScanPerBlock 每区块最多检查的心跳索引条目数。
	MaxOfflineScanPerBlock int = 128
	// MaxAllowancePerBlock 每区块最多处理的津贴发放节点数。
	MaxAllowancePerBlock int = 128
)

// HeartbeatIndexKey 构造心跳索引键：前缀 + 8 字节大端高度 + 地址。
// 负高度（不应出现）统一归零，保证编码单调。
func HeartbeatIndexKey(height int64, addr string) []byte {
	out := make([]byte, 0, len(HeartbeatIndexKeyPrefix)+8+len(addr))
	out = append(out, HeartbeatIndexKeyPrefix...)
	out = append(out, heightBE(height)...)
	return append(out, []byte(addr)...)
}

// HeartbeatIndexBound 返回「高度 < height」在索引内（去前缀后）的排他上界。
func HeartbeatIndexBound(height int64) []byte {
	return heightBE(height)
}

func heightBE(height int64) []byte {
	h := uint64(0)
	if height > 0 {
		h = uint64(height)
	}
	return []byte{
		byte(h >> 56), byte(h >> 48), byte(h >> 40), byte(h >> 32),
		byte(h >> 24), byte(h >> 16), byte(h >> 8), byte(h),
	}
}

// NodeAllowanceDayKey 返回某节点最近领取日序号的存储 key。
func NodeAllowanceDayKey(addr string) []byte {
	return append(NodeAllowanceDayKeyPrefix, []byte(addr)...)
}

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

// DevicePubKeyKey 返回某节点绑定的设备公钥 key。
func DevicePubKeyKey(addr string) []byte {
	return append(DevicePubKeyKeyPrefix, []byte(addr)...)
}
