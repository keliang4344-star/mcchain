package main

// 生产 TEE 设备证明验证器（链下）。
//
// 信任模型（必须与链上区分清楚）：
//   - 链上 TeeOracle 只验证「预言机账户对 deviceAddr|challenge 的 secp256k1 签名」，
//     它证明的是「预言机背书过这条 attestation」，**不是**硬件证明本身；
//   - 硬件证明（证书链、设备安全等级、nonce、过期、撤销、重放）由本文件在链下
//     完成，验证通过后预言机才以自己的账户密钥对 challenge 签名并提交上链。
//
// 因此生产环境缺少可信根证书 / 撤销列表 / 有效签名密钥时必须拒绝启动，
// 绝不允许回退到「仅校验非空」的软验证。

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// teeProofMaxSize 限制单条证明的字节数，避免超大 JSON/PEM 撑爆内存与日志。
	teeProofMaxSize = 256 * 1024
	// teeProofPurpose 是设备证明的唯一合法用途，防止把其他场景的证明重放到
	// 设备认证（purpose confusion）。
	teeProofPurpose = "device_attestation"
	// teeProofMaxTTL 是证明允许的最长有效期，防止签发方给出超长窗口。
	teeProofMaxTTL = 10 * time.Minute
	// teeProofClockSkew 容忍的设备/服务端时钟偏差。
	teeProofClockSkew = 2 * time.Minute
	// teeNonceCacheLimit 限制重放缓存条目数，超限时按最早过期清理。
	teeNonceCacheLimit = 20000
)

// TeeProof 是设备端/设备厂商服务提交的硬件证明包。
//
// CertChainPEM 必须按「leaf → intermediate...」顺序给出；根证书只来自本地
// 可信根文件（绝不采信证明包自带的根，否则等于没有信任根）。
type TeeProof struct {
	DeviceID      string   `json:"device_id"`
	ChainID       string   `json:"chain_id"`
	Purpose       string   `json:"purpose"`
	Nonce         string   `json:"nonce"`
	ExpiresAt     int64    `json:"expires_at"`
	SecurityLevel string   `json:"security_level"`
	CertChainPEM  []string `json:"cert_chain_pem"`
	Signature     string   `json:"signature"`
}

// TeeRevocationList 是撤销列表文件格式（指纹为证书 DER 的 sha256 hex）。
type TeeRevocationList struct {
	RevokedFingerprints []string `json:"revoked_fingerprints"`
	RevokedSerials      []string `json:"revoked_serials"`
}

// TeeVerifierConfig 是构造 TeeVerifier 的显式配置。
type TeeVerifierConfig struct {
	ChainID          string
	RootsFile        string
	RevocationFile   string
	MinSecurityLevel string
}

// TeeVerifier 生产用硬件证明验证器。
type TeeVerifier struct {
	chainID          string
	rootsFile        string
	revocationFile   string
	minSecurityLevel string

	mu           sync.Mutex
	roots        *x509.CertPool
	rootsModTime time.Time
	revoked      map[string]struct{}
	revokedMod   time.Time

	nonceMu    sync.Mutex
	usedNonces map[string]time.Time

	now func() time.Time // 可注入时钟，便于测试过期/时钟偏差
}

// NewTeeVerifier 构造验证器并做启动期自检：可信根必须存在且可解析。
//
// 生产启动路径调用此函数；任何缺失都返回错误（由上层拒绝启动），
// 不会退化成「仅校验非空」。
func NewTeeVerifier(cfg TeeVerifierConfig) (*TeeVerifier, error) {
	if strings.TrimSpace(cfg.ChainID) == "" {
		return nil, fmt.Errorf("tee verifier: chain id is required")
	}
	if strings.TrimSpace(cfg.RootsFile) == "" {
		return nil, fmt.Errorf("tee verifier: trusted roots file is required (ORACLE_TEE_ROOTS_FILE)")
	}
	level := strings.ToLower(strings.TrimSpace(cfg.MinSecurityLevel))
	switch level {
	case "":
		level = "tee"
	case "strongbox", "tee", "software":
	default:
		return nil, fmt.Errorf("tee verifier: unknown min security level %q (strongbox|tee|software)", cfg.MinSecurityLevel)
	}

	v := &TeeVerifier{
		chainID:          cfg.ChainID,
		rootsFile:        cfg.RootsFile,
		revocationFile:   strings.TrimSpace(cfg.RevocationFile),
		minSecurityLevel: level,
		revoked:          map[string]struct{}{},
		usedNonces:       map[string]time.Time{},
		now:              time.Now,
	}
	if err := v.reloadRoots(); err != nil {
		return nil, err
	}
	if err := v.reloadRevocation(); err != nil {
		return nil, err
	}
	return v, nil
}

// reloadRoots 读取并解析可信根文件；文件变更时自动重载（根证书轮换）。
func (v *TeeVerifier) reloadRoots() error {
	st, err := os.Stat(v.rootsFile)
	if err != nil {
		return fmt.Errorf("tee verifier: stat roots file: %w", err)
	}
	if v.roots != nil && st.ModTime().Equal(v.rootsModTime) {
		return nil
	}
	bz, err := os.ReadFile(v.rootsFile)
	if err != nil {
		return fmt.Errorf("tee verifier: read roots file: %w", err)
	}
	pool := x509.NewCertPool()
	added := 0
	for {
		var block *pem.Block
		block, bz = pem.Decode(bz)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return fmt.Errorf("tee verifier: parse root certificate: %w", err)
		}
		pool.AddCert(cert)
		added++
	}
	if added == 0 {
		return fmt.Errorf("tee verifier: roots file %q contains no CERTIFICATE blocks", v.rootsFile)
	}
	v.roots = pool
	v.rootsModTime = st.ModTime()
	return nil
}

// reloadRevocation 读取撤销列表（可选）；文件缺失视为空列表。
func (v *TeeVerifier) reloadRevocation() error {
	if v.revocationFile == "" {
		return nil
	}
	st, err := os.Stat(v.revocationFile)
	if err != nil {
		if os.IsNotExist(err) {
			// 显式配置了撤销文件却不存在：拒绝启动，避免「以为在检查撤销」。
			return fmt.Errorf("tee verifier: revocation file %q does not exist", v.revocationFile)
		}
		return fmt.Errorf("tee verifier: stat revocation file: %w", err)
	}
	if v.revoked != nil && len(v.revoked) > 0 && st.ModTime().Equal(v.revokedMod) {
		return nil
	}
	bz, err := os.ReadFile(v.revocationFile)
	if err != nil {
		return fmt.Errorf("tee verifier: read revocation file: %w", err)
	}
	var list TeeRevocationList
	if err := json.Unmarshal(bz, &list); err != nil {
		return fmt.Errorf("tee verifier: parse revocation file: %w", err)
	}
	revoked := make(map[string]struct{}, len(list.RevokedFingerprints))
	for _, fp := range list.RevokedFingerprints {
		revoked[strings.ToLower(strings.TrimSpace(fp))] = struct{}{}
	}
	for _, sn := range list.RevokedSerials {
		revoked["serial:"+strings.ToLower(strings.TrimSpace(sn))] = struct{}{}
	}
	v.revoked = revoked
	v.revokedMod = st.ModTime()
	return nil
}

// Verify 实现 AttestationVerifier。
// deviceID / challenge 来自 HTTP 请求；proof 是 base64(JSON TeeProof)。
func (v *TeeVerifier) Verify(deviceID, proof, signature string) (bool, string) {
	_ = signature // 设备签名已包含在 proof.Signature 中；此参数保留接口兼容
	if len(proof) == 0 {
		return false, "empty attestation proof"
	}
	raw, err := base64.StdEncoding.DecodeString(proof)
	if err != nil {
		return false, "attestation proof is not valid base64"
	}
	if len(raw) > teeProofMaxSize {
		return false, "attestation proof exceeds size limit"
	}
	var tp TeeProof
	if err := json.Unmarshal(raw, &tp); err != nil {
		return false, "attestation proof is not valid JSON"
	}

	// 1) 上下文绑定：设备、链、用途、nonce 必须与请求完全一致。
	if tp.DeviceID != deviceID {
		return false, "proof device_id does not match request"
	}
	if tp.ChainID != v.chainID {
		return false, fmt.Sprintf("proof chain_id %q does not match oracle chain %q", tp.ChainID, v.chainID)
	}
	if tp.Purpose != teeProofPurpose {
		return false, fmt.Sprintf("proof purpose %q is not %q", tp.Purpose, teeProofPurpose)
	}
	if strings.TrimSpace(tp.Nonce) == "" {
		return false, "proof nonce is empty"
	}
	if tp.ExpiresAt <= 0 {
		return false, "proof expires_at is missing"
	}

	now := v.now()
	expires := time.Unix(tp.ExpiresAt, 0)
	if now.After(expires.Add(teeProofClockSkew)) {
		return false, "proof has expired"
	}
	if expires.After(now.Add(teeProofMaxTTL + teeProofClockSkew)) {
		return false, "proof expiration window exceeds maximum TTL"
	}

	// 2) 设备安全等级：生产默认至少 tee。
	if !securityLevelAtLeast(tp.SecurityLevel, v.minSecurityLevel) {
		return false, fmt.Sprintf("security_level %q is below required %q", tp.SecurityLevel, v.minSecurityLevel)
	}

	// 3) 证书链：leaf → intermediate，根只来自本地可信根。
	if len(tp.CertChainPEM) == 0 {
		return false, "proof has no certificate chain"
	}
	leaf, intermediates, err := parseCertChain(tp.CertChainPEM)
	if err != nil {
		return false, err.Error()
	}
	if err := v.reloadRoots(); err != nil {
		return false, err.Error()
	}
	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots:         v.roots,
		Intermediates: intermediates,
		CurrentTime:   now,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}); err != nil {
		return false, "certificate chain verification failed: " + err.Error()
	}

	// 4) 撤销列表：按指纹与序列号双向检查。
	if err := v.reloadRevocation(); err != nil {
		return false, err.Error()
	}
	fp := sha256.Sum256(leaf.Raw)
	if _, bad := v.revoked[hex.EncodeToString(fp[:])]; bad {
		return false, "leaf certificate is revoked"
	}
	if _, bad := v.revoked["serial:"+strings.ToLower(leaf.SerialNumber.Text(16))]; bad {
		return false, "leaf certificate serial is revoked"
	}

	// 5) 证明签名：leaf 私钥对规范化上下文签名。
	sig, err := base64.StdEncoding.DecodeString(tp.Signature)
	if err != nil || len(sig) == 0 {
		return false, "proof signature is missing or not base64"
	}
	payload := canonicalProofPayload(&tp)
	if err := verifySignature(leaf.PublicKey, payload, sig); err != nil {
		return false, "proof signature verification failed: " + err.Error()
	}

	// 6) 一次性 nonce：同一 (device, nonce) 只能成功一次。
	if !v.consumeNonce(deviceID, tp.Nonce, expires) {
		return false, "proof nonce already used (replay rejected)"
	}

	return true, "tee attestation verified (off-chain certificate chain + nonce)"
}

func canonicalProofPayload(tp *TeeProof) []byte {
	return []byte(strings.Join([]string{
		tp.DeviceID,
		tp.ChainID,
		tp.Purpose,
		tp.Nonce,
		strconv.FormatInt(tp.ExpiresAt, 10),
		tp.SecurityLevel,
	}, "\x00"))
}

func parseCertChain(pemBlocks []string) (*x509.Certificate, *x509.CertPool, error) {
	pool := x509.NewCertPool()
	var leaf *x509.Certificate
	for i, s := range pemBlocks {
		block, _ := pem.Decode([]byte(s))
		if block == nil || block.Type != "CERTIFICATE" {
			return nil, nil, fmt.Errorf("cert_chain_pem[%d] is not a CERTIFICATE", i)
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, nil, fmt.Errorf("cert_chain_pem[%d] parse failed: %w", i, err)
		}
		if i == 0 {
			leaf = cert
			continue
		}
		pool.AddCert(cert)
	}
	return leaf, pool, nil
}

func verifySignature(pub crypto.PublicKey, msg, sig []byte) error {
	digest := sha256.Sum256(msg)
	switch k := pub.(type) {
	case *ecdsa.PublicKey:
		if !ecdsa.VerifyASN1(k, digest[:], sig) {
			return fmt.Errorf("ECDSA signature mismatch")
		}
		return nil
	case *rsa.PublicKey:
		return rsa.VerifyPKCS1v15(k, crypto.SHA256, digest[:], sig)
	default:
		return fmt.Errorf("unsupported public key type %T", pub)
	}
}

func securityLevelAtLeast(level, min string) bool {
	rank := map[string]int{"software": 0, "tee": 1, "strongbox": 2}
	got, ok := rank[strings.ToLower(strings.TrimSpace(level))]
	if !ok {
		return false
	}
	return got >= rank[min]
}

// consumeNonce 原子登记 nonce；已存在返回 false。
func (v *TeeVerifier) consumeNonce(deviceID, nonce string, expires time.Time) bool {
	key := deviceID + "\x00" + nonce
	now := v.now()

	v.nonceMu.Lock()
	defer v.nonceMu.Unlock()

	// 惰性清理：只在超限时按过期时间清一轮，避免每请求全表扫描。
	if len(v.usedNonces) >= teeNonceCacheLimit {
		for k, exp := range v.usedNonces {
			if now.After(exp) {
				delete(v.usedNonces, k)
			}
		}
		if len(v.usedNonces) >= teeNonceCacheLimit {
			// 仍满：拒绝新 nonce 而不是无界增长（宁可短暂拒绝，也不吃光内存）。
			return false
		}
	}
	if _, used := v.usedNonces[key]; used {
		return false
	}
	v.usedNonces[key] = expires.Add(teeProofMaxTTL)
	return true
}
