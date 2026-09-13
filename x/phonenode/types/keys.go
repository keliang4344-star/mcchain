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

// 安全相关 KV key 前缀与构造器。
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
	// AttestationTierKeyPrefix 是节点地址 → attestation 验证层级 的存储前缀。
	// attestation 证明强度分级（见下方 AttestationTier* 常量）。
	AttestationTierKeyPrefix = []byte("AttTier:")
	// AttestationChallengeKeyPrefix 是节点地址 → 最近一次「预言机已验证」的 challenge，
	// 仅作审计留痕，不参与权限判定。
	AttestationChallengeKeyPrefix = []byte("AttChal:")
	// OracleChallengeUsedKeyPrefix 是「已被消费的预言机 challenge」一次性索引前缀。
	//
	// TeeOracle 验签的是设备地址与 challenge 的拼接串，challenge 本身
	// 只是不透明字符串。没有「用过即废」的记录时，一对 (challenge,
	// signature) 可以在一笔合法交易被广播后**无限次重放**——设备被 slash 进入冷却期、
	// 或 attestation 到期降级后，重放旧签名即可重新拿到 TierOracle 背书。
	// 这里按 challenge 摘要登记消费标记，使每次预言机背书只生效一次。
	OracleChallengeUsedKeyPrefix = []byte("AttChalUsed:")
	// OracleEndorsementAtKeyPrefix 是节点地址 → 最近一次预言机背书的区块时间戳。
	//
	// 即使 challenge 一次性，背书本身也不能永久有效。记录背书时间，
	// 配合 OracleEndorsementValidity 让「预言机已背书」成为一种有期限的断言——
	// 与 attestation 自身的 30 天有效期保持同一语义（到期需重新走真机校验）。
	OracleEndorsementAtKeyPrefix = []byte("AttEndorse:")

	// ---- 节点资本津贴（建设溢价）存储键（2026-08 落地）----
	// NodeAllowanceConfigKey 节点资本津贴配置（Enabled / PerDay），存储于模块 KVStore。
	NodeAllowanceConfigKey = []byte("NodeAllowCfg:")
	// NodeAllowanceDayKeyPrefix 记录各节点最近一次领取津贴的「日序号」：NodeAllowDay:<addr>
	NodeAllowanceDayKeyPrefix = []byte("NodeAllowDay:")
	// GlobalLastAllowanceDayKey 全局「当日已分发」标记，避免同日重复遍历。
	GlobalLastAllowanceDayKey = []byte("NodeAllowGlobalDay:")

	// ---- 有界扫描索引 / FIFO 到期队列 ----
	// HeartbeatIndexKeyPrefix 是「按最近心跳高度排序」的节点索引前缀：
	//   HeartbeatIndexKey(lastProofBlock, addr) = "HbIdx:" + be8(height) + addr
	// 大端编码保证键的字典序等同高度的数值序。离线检测从索引**头部**开始扫，
	// 头部即 LastProofBlock 最小 = 最早心跳 = 最早到期的节点，构成天然的
	// FIFO 到期队列，因此不需要持久化轮转游标：
	//   - 在线节点心跳会把条目重写到更大高度 → 离开头部；
	//   - 离线节点条目沉在头部 → 被扫到即 slash 并出队。
	// 前提是「处理过必出队」，否则僵尸条目会在头部反复被扫、吃掉全部预算。
	HeartbeatIndexKeyPrefix = []byte("HbIdx:")
	// AllowanceScanCursorKey 节点资本津贴分发的持久化轮转游标。
	AllowanceScanCursorKey = []byte("cursor:allowance_scan")

	// OfflineBacklogFlagKey 记录「上一区块离线检测是否出现积压」。
	//
	// 检测到积压后置位，下一区块自动放大扫描预算追赶，批次完成后清除。
	// 这条自愈路径针对的是「区域性断网导致大面积同时离线」这类突发场景：
	// 稳态下每块到期量约为 0，只有突发时才会短时排队，放大预算即可在一个
	// 出块周期内消化掉，不需要人工干预。
	OfflineBacklogFlagKey = []byte("offline_backlog")
)

// BeginBlock 有界扫描预算。
// 硬约束：BeginBlock 内不得出现 O(全量节点) 的遍历，否则节点规模上量后直接停块。
//
// 容量模型（设计离线检测时请按此推导，不要凭直觉调数值）：
//
//	索引键 = be8(LastProofBlock) + addr，按高度升序排列。
//	在线节点每次心跳都会把条目重写到更大的高度，因此条目会不断向索引尾部迁移；
//	索引头部只可能积累「已经停止心跳、真正离线」的节点。配合「处理过必出队」
//	（见 keeper.DetectOffline / SlashIfBad），可得：
//
//	    稳态每区块处理量 ≈ 该区块真正离线的节点数 ≈ 0
//
//	也就是容量与「已注册设备总数」解耦 —— 不存在「N 台设备上限」这回事。
//	唯一会短暂积压的场景是大面积同时离线，由 OfflineBacklogFlagKey 的自愈
//	路径 + phonenode.OfflineBacklog 事件覆盖。
//
// 心跳节奏与宽限期的关系由产品口径给定（白皮书 / RUNBOOK 保持一致）：
// 设备必须在 OfflineGraceBlocks（100 块 ≈ 400s）内至少心跳一次，否则判离线。
const (
	// MaxOfflineScanPerBlock 稳态下每区块检查的心跳索引条目数。
	// 稳态只需处理「本区块真正离线」的少数节点（正常接近 0），128 有充裕余量。
	MaxOfflineScanPerBlock int = 128
	// MaxOfflineScanPerBlockBacklog 检出积压时的放大预算（自愈追赶）。
	// 取 4 倍：单区块最多 512 次「读 attestation + 读 node + 少量写」。
	// 4s 出块间隔下这一量级的 KV 操作留有数量级余量，不会把 BeginBlock 顶穿。
	MaxOfflineScanPerBlockBacklog int = 512
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

// SlashCooldownKey 返回某节点 slash 冷却截止高度 key。
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

// AttestationTierKey 返回某节点 attestation 验证层级 key。
func AttestationTierKey(addr string) []byte {
	return append(AttestationTierKeyPrefix, []byte(addr)...)
}

// AttestationChallengeKey 返回某节点最近一次预言机已验证 challenge 的留痕 key。
func AttestationChallengeKey(addr string) []byte {
	return append(AttestationChallengeKeyPrefix, []byte(addr)...)
}

// OracleChallengeUsedKey 返回「某节点已消费的预言机 challenge 摘要」一次性索引 key。
//
// challengeDigest 固定为 SHA-256 的 hex（64 字符），因此键长恒定，
// 不受外部输入长度影响（键长度可控 + 一次性消费）。
func OracleChallengeUsedKey(addr, challengeDigest string) []byte {
	key := make([]byte, 0, len(OracleChallengeUsedKeyPrefix)+len(addr)+1+len(challengeDigest))
	key = append(key, OracleChallengeUsedKeyPrefix...)
	key = append(key, []byte(addr)...)
	key = append(key, '/')
	key = append(key, []byte(challengeDigest)...)
	return key
}

// OracleEndorsementAtKey 返回某节点最近一次预言机背书的区块时间戳 key。
func OracleEndorsementAtKey(addr string) []byte {
	return append(OracleEndorsementAtKeyPrefix, []byte(addr)...)
}

// ---------------------------------------------------------------------------
// attestation 验证层级（分级信任）
//
// 冷启动阶段刻意不设准入门槛——任何注册设备都能以「设备自证」完成认证并开始挖矿，
// 这保证了获客速度（分享 / 推荐 / 组队不受阻）。但自证材料在链上是可伪造的
// （只要自己生成一对密钥即可），因此不能作为高经济权重的唯一依据。
//
// 于是把「能否参与」与「按什么权重参与」拆开：
//   - TierSelf   ：设备自签证明，参与不受限（冷启动不变）；
//   - TierOracle ：在自签之外，额外持有受信预言机对同一 challenge 的背书签名，
//     由 depin 的 TeeOracle 链上验签后回写本层级。
//
// 需要「真实设备」才能获得的经济动作（DePIN 贡献拨付的独立校验）走 TierOracle；
// 纯参与类动作继续走 TierSelf。这样既不抬高参与门槛，又让伪造成本从「零」变成
// 「必须拿到一台真机 + 通过预言机真机校验」。
// ---------------------------------------------------------------------------
const (
	// AttestationTierSelf 设备自签（基础层，冷启动默认）。
	AttestationTierSelf uint8 = 0
	// AttestationTierOracle 预言机已背书（增强层）。
	AttestationTierOracle uint8 = 1
)

// Attestation 输入字段硬约束。
//
// 这些字段中的 nonce / device_id_hash 会直接进入 KVStore 键，
// 无长度上限意味着攻击者可以用超长字符串把状态膨胀成无法容忍的体积
// （每次 attest 一个 1 MiB 的 nonce ⇒ 每个节点都能推动一次状态膨胀）。
// 因此在写库之前必须先做长度与字符集闸门。
const (
	// MaxNonceLength nonce 最大字节数（bech32 地址之外的重放索引段）。
	MaxNonceLength = 64
	// MinNonceLength nonce 最小字节数，避免空/退化 nonce 让重放防护失去意义。
	MinNonceLength = 8
	// MaxDeviceIDHashLength device_id_hash 最大长度（SHA-256 hex = 64）。
	MaxDeviceIDHashLength = 64
	// MaxRootHashLength root_hash 最大长度（base64 编码的 32 字节摘要 = 44）。
	MaxRootHashLength = 128
)

// 预言机背书的有效期与 challenge 输入上限。
const (
	// MaxOracleChallengeLength 预言机 challenge 的最大字节数。
	//
	// challenge 由链下预言机服务生成（通常是真机校验结果摘要或随机 nonce），
	// 链上既不解析也不回显，但必须给长度一个上界：它会被拼进留痕键与事件。
	MaxOracleChallengeLength = 256
	// OracleEndorsementValidity 预言机背书的有效期（秒），默认 7 天。
	//
	// 与 AttestationValidity（30 天）同语义但更短：真机校验是有时效的断言，
	// 设备可在有效期内更换硬件/环境，背书越长越容易「一次校验、长期受益」。
	// 7 天意味着 DePIN 贡献拨付所需的 TierOracle 每周至少重新走一次真机校验，
	// 伪造成本从「一次性」变为「持续可验证」。
	OracleEndorsementValidity int64 = 86400 * 7
)
