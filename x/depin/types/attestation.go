package types

import "time"

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
func NewAttestationResult(deviceID string, passed bool, reason, oracleAddr string) AttestationResult {
	return AttestationResult{
		DeviceID:      deviceID,
		Timestamp:     time.Now().Unix(),
		Passed:        passed,
		Reason:        reason,
		OracleAddress: oracleAddr,
	}
}
