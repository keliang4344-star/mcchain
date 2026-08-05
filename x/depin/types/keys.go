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

// 【已撤销】DePINBurnRatioBps（设备任务赏金 5% 销毁）
//
// 早期版本对每笔设备任务赏金抽取 5% 打入黑洞。该设计已按白皮书《优化定稿版》
// §24.6 否决清单撤销：通缩只能来自「协议使用费」（gas 7%、DEX 手续费 0.05%）
// 与「作恶罚没」（40%），绝不侵蚀参与者以真实算力/带宽/在线时长换来的劳动应得。
// 设备完成任务应得多少即足额到手，链上不做任何截留。
//
// 常量已删除而非置零，以从编译层面杜绝该逻辑被重新引用。
