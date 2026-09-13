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

// AttestChallengeUsedKey 返回「已被消费的设备认证 challenge 摘要」一次性索引 key。
//
// AttestDevice 只验签不记消费，同一对 (challenge, signature)
// 可以无限重放把 DeviceState.Attested 反复置真（并被用于刷新有效期）。
// 按 SHA-256 摘要登记消费标记，键长恒定（64 hex 字符），不受输入长度影响。
func AttestChallengeUsedKey(addr, challengeDigest string) []byte {
	key := make([]byte, 0, len("AttChalUsed:")+len(addr)+1+len(challengeDigest))
	key = append(key, []byte("AttChalUsed:")...)
	key = append(key, []byte(addr)...)
	key = append(key, '/')
	key = append(key, []byte(challengeDigest)...)
	return key
}

// MaxContributionTaskIDLength 贡献记录 task_id 的最大字节数。
//
// ContributionKeyPrefix + taskID 直接构成 KVStore 键，
// 长度不设限时——每次提交一个 1 MiB 的 task_id 就是一次确定性的
// 状态膨胀（写入按字节计费，但状态永久驻留，gas 拦不住）。
// 128 字节足以容纳 UUID/哈希/业务编号三类常见形态。
const MaxContributionTaskIDLength = 128

// 【已撤销】DePINBurnRatioBps（设备任务赏金 5% 销毁）
//
// 早期版本对每笔设备任务赏金抽取 5% 打入黑洞。该设计已按白皮书《优化定稿版》
// §24.6 否决清单撤销：通缩只能来自「协议使用费」（gas 7%、DEX 手续费的 50%）
// 与「作恶罚没」（40%），绝不侵蚀参与者以真实算力/带宽/在线时长换来的劳动应得。
// 设备完成任务应得多少即足额到手，链上不做任何截留。
//
// 常量已删除而非置零，以从编译层面杜绝该逻辑被重新引用。
