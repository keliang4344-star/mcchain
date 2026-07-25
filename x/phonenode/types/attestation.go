package types

// Attestation 链上 attestation 状态机状态值（与 query.proto 的 Attestation.status 对齐）。
// 状态流转：valid（提交有效证明）→ invalid/revoked（过期/伪造/被 slash）。
const (
	AttestationStatusPending = "pending"
	AttestationStatusValid   = "valid"
	AttestationStatusInvalid = "invalid"
	AttestationStatusRevoked = "revoked"
)

// NewValidAttestation 构造一条有效 attestation（status=valid）。
func NewValidAttestation(rootHash, nonce, deviceIDHash string, expiry int64) *Attestation {
	return &Attestation{
		RootHash:     rootHash,
		Nonce:        nonce,
		DeviceIdHash: deviceIDHash,
		Expiry:       expiry,
		Status:       AttestationStatusValid,
	}
}

// IsExpired 判断 attestation 是否已超过 expiry（基于链上当前时间 now）。
func (a *Attestation) IsExpired(now int64) bool {
	return a.Expiry > 0 && now > a.Expiry
}
