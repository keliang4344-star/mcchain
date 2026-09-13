package types

// AttestationResult 预言机提交的单条设备 attestation 验证结果。
type AttestationResult struct {
	DeviceID      string `json:"device_id"`
	Timestamp     int64  `json:"timestamp"`      // Unix 秒时间戳
	Passed        bool   `json:"passed"`         // 验证是否通过
	Reason        string `json:"reason"`         // 通过/失败原因
	OracleAddress string `json:"oracle_address"` // 提交该结果的预言机地址
}

// AttestationHistory 设备的历史 attestation 记录列表。
type AttestationHistory struct {
	Results []AttestationResult `json:"results"`
}

// NewAttestationResult 构造一条验证结果。
// timestamp 必须传入 ctx.BlockTime().Unix()，由各验证人从同一区块时间推导，
// 保证链上状态确定性（可复现、不出分叉）。切勿在此使用 time.Now()。
func NewAttestationResult(deviceID string, passed bool, reason, oracleAddr string, timestamp int64) AttestationResult {
	return AttestationResult{
		DeviceID:      deviceID,
		Timestamp:     timestamp,
		Passed:        passed,
		Reason:        reason,
		OracleAddress: oracleAddr,
	}
}

// 设备认证（AttestDevice）的输入约束与有效期。
const (
	// MaxAttestChallengeLength 设备认证 challenge 的最大字节数。
	//
	// challenge 会拼进「已消费 challenge」索引键与 attestation 历史；
	// 无长度上限即等于把状态体积的控制权交给提交者。
	MaxAttestChallengeLength = 256

	// AttestationValiditySeconds 一次成功设备认证的有效期（秒），默认 7 天。
	//
	// DeviceState.Attested 若为**永不过期的布尔位**：设备只要成功认证过一次，
	// 无论其后 attestation 是否过期、是否被 slash、硬件是否更换，都能持续领取
	// DePIN 贡献奖励。这里给认证结果一个明确期限，到期需重新走一次真机校验，
	// 与 phonenode 的 OracleEndorsementValidity 保持同一量级。
	AttestationValiditySeconds int64 = 86400 * 7
)
