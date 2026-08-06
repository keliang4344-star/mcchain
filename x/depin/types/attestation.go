package types

// AttestationResult 预言机提交的单条设备 attestation 验证结果。
type AttestationResult struct {
	DeviceID      string `json:"device_id"`
	Timestamp     int64  `json:"timestamp"`      // Unix 秒时间戳
	Passed        bool   `json:"passed"`          // 验证是否通过
	Reason        string `json:"reason"`          // 通过/失败原因
	OracleAddress string `json:"oracle_address"`  // 提交该结果的预言机地址
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
