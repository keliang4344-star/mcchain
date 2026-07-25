package oraclesvc

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"strings"
)

// 本文件实现「预言机真验硬件」生产闭环（P0②）：链下预言机在签发
// `deviceAddr|challenge` 之前，必须先验证设备提交的真实硬件 attestation，杜绝
// 「来者不拒」式签名。支持多根（multi-root）：
//   - play_integrity        ：Google Play Integrity（ES256 JWS，nonce 绑定 device|challenge）
//   - huawei                ：华为 HWAttestation / AppGallery 完整性令牌（ES256 JWS，同结构）
//   - android_key_attestation：Android Key Attestation（X.509 证书链 → 可信根，challenge 嵌入扩展）
//
// 信任模型：预言机是网关，仅对通过真机验证的设备签名；链上 TeeOracle 只需验证
// 预言机签名（见 x/depin/types/oracle.go）。故真机校验强度完全取决于本模块。

var (
	// ErrAttestation 表示 attestation 验证失败（任一环节不通过即拒绝签名）。
	ErrAttestation = errors.New("oraclesvc: attestation verification failed")
	// ErrUnknownRoot 表示 attestation 的 root 不在已知列表。
	ErrUnknownRoot = errors.New("oraclesvc: unknown attestation root")
	// ErrRootNotAllowed 表示 attestation 的 root 未在当前部署允许的列表中。
	ErrRootNotAllowed = errors.New("oraclesvc: attestation root not allowed in this deployment")
)

// AttestationClaim 是设备随 /sign 请求提交的硬件 attestation。
type AttestationClaim struct {
	Root       string `json:"root"`        // play_integrity | huawei | android_key_attestation
	Payload    string `json:"payload"`     // root 特定：JWS 令牌（PI/华为）或 PEM 证书链（AKA）
	Challenge  string `json:"challenge"`   // 预言机下发的 challenge（与链上一致）
	DeviceAddr string `json:"device_addr"` // 设备链上地址（绑定进 attestation nonce）
}

// AttestationVerifier 校验某一种根的硬件 attestation，并确认其绑定到 (deviceAddr, challenge)。
type AttestationVerifier interface {
	// Root 返回该验证器对应的根标识。
	Root() string
	// Verify 校验 attestation 是否来自真实设备且绑定到预期的 device|challenge。
	// 任一环节失败返回 ErrAttestation（或 ErrUnknownRoot / ErrRootNotAllowed）。
	Verify(ctx context.Context, c AttestationClaim) error
}

// boundMessage 返回 attestation 必须绑定的消息：与链上 TeeOracle 验签一致的
// `deviceAddr|challenge`。设备端在生成 attestation 时（nonce / challenge 字段）
// 必须包含此串，预言机据此确认「签名请求来自这台真机、且对应当前 challenge」。
func boundMessage(c AttestationClaim) string {
	return c.DeviceAddr + "|" + c.Challenge
}

// ---------------------------------------------------------------------------
// 1) ES256 JWS 令牌验证器（Play Integrity / 华为共用结构）
// ---------------------------------------------------------------------------

// signedTokenVerifier 验证一个 ES256 签名的 JWS/Payload，并提取 nonce。
// Play Integrity 与华为完整性令牌均为 ES256 签名，payload 为 JSON，
// nonce 字段（PI 在 requestDetails.nonce；华为可直接用 nonce）经 base64 编码后
// 必须等于 boundMessage。
type signedTokenVerifier struct {
	root string
	pub  *ecdsa.PublicKey
}

// NewSignedTokenVerifier 用受信任的根公钥构造一个 JWS 验证器。
func NewSignedTokenVerifier(root string, pub *ecdsa.PublicKey) AttestationVerifier {
	return &signedTokenVerifier{root: root, pub: pub}
}

func (v *signedTokenVerifier) Root() string { return v.root }

func (v *signedTokenVerifier) Verify(ctx context.Context, c AttestationClaim) error {
	claims, err := verifyJWS(c.Payload, v.pub)
	if err != nil {
		return ErrAttestation
	}
	// 提取 nonce：Play Integrity 在 requestDetails.nonce；华为/通用直接用 nonce。
	nonceB64, ok := extractNonce(claims)
	if !ok || nonceB64 == "" {
		return ErrAttestation
	}
	nonce, err := base64.StdEncoding.DecodeString(nonceB64)
	if err != nil {
		// 部分实现用 rawurl；再试一次。
		if n2, e2 := base64.RawURLEncoding.DecodeString(nonceB64); e2 == nil {
			nonce = n2
		} else {
			return ErrAttestation
		}
	}
	if string(nonce) != boundMessage(c) {
		return ErrAttestation
	}
	return nil
}

// verifyJWS 解析并验证 ES256 紧凑 JWS（header.payload.signature），返回 payload 的 JSON claims。
func verifyJWS(token string, pub *ecdsa.PublicKey) (map[string]interface{}, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return nil, ErrAttestation
	}
	// 解析 header，校验 alg == ES256。
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrAttestation
	}
	var header struct {
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil || header.Alg != "ES256" {
		return nil, ErrAttestation
	}
	// 解码签名（ES256 over P-256 → 64 字节 R||S）。
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(sig) != 64 {
		return nil, ErrAttestation
	}
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:])
	msg := []byte(parts[0] + "." + parts[1])
	h := sha256.Sum256(msg)
	if !ecdsa.Verify(pub, h[:], r, s) {
		return nil, ErrAttestation
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrAttestation
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, ErrAttestation
	}
	return claims, nil
}

// extractNonce 从 claims 取 nonce：优先 Play Integrity 的 requestDetails.nonce，
// 回退到顶层 nonce。
func extractNonce(claims map[string]interface{}) (string, bool) {
	if rd, ok := claims["requestDetails"].(map[string]interface{}); ok {
		if n, ok := rd["nonce"].(string); ok {
			return n, true
		}
	}
	if n, ok := claims["nonce"].(string); ok {
		return n, true
	}
	return "", false
}

// ---------------------------------------------------------------------------
// 2) Android Key Attestation（X.509 证书链 → 可信根 + challenge 扩展）
// ---------------------------------------------------------------------------

// androidKeyAttestationOID 为 Android Key Attestation 证书中 KeyDescription 扩展的 OID。
var androidKeyAttestationOID = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 11129, 2, 1, 17}

// androidKAVerifier 验证 Android Key Attestation 证书链到可信根，并确认 challenge 扩展
// 绑定的消息 == boundMessage。
type androidKAVerifier struct {
	roots *x509.CertPool
}

// NewAndroidKAVerifier 用受信任的根 CA 池构造验证器（生产注入 Google/Huawei 等设备根）。
func NewAndroidKAVerifier(roots *x509.CertPool) AttestationVerifier {
	return &androidKAVerifier{roots: roots}
}

func (v *androidKAVerifier) Root() string { return "android_key_attestation" }

func (v *androidKAVerifier) Verify(ctx context.Context, c AttestationClaim) error {
	if v.roots == nil {
		return ErrAttestation
	}
	certs, err := parsePEMCertChain(c.Payload)
	if err != nil || len(certs) == 0 {
		return ErrAttestation
	}
	leaf := certs[0]
	// 验证整条链到可信根（中间证书可由 leaf 携带）。
	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots:       v.roots,
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
		Intermediates: intermediatePool(certs[1:]),
	}); err != nil {
		return ErrAttestation
	}
	challenge, err := extractAttestationChallenge(leaf)
	if err != nil {
		return ErrAttestation
	}
	if string(challenge) != boundMessage(c) {
		return ErrAttestation
	}
	return nil
}

// parsePEMCertChain 从 PEM 文本解析出有序证书列表（leaf 在前）。
func parsePEMCertChain(pemText string) ([]*x509.Certificate, error) {
	var certs []*x509.Certificate
	rest := []byte(pemText)
	for {
		block, rem := pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" {
			c, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, err
			}
			certs = append(certs, c)
		}
		rest = rem
	}
	if len(certs) == 0 {
		return nil, ErrAttestation
	}
	return certs, nil
}

// intermediatePool 把中间证书组装成 CertPool 供链校验使用。
func intermediatePool(certs []*x509.Certificate) *x509.CertPool {
	pool := x509.NewCertPool()
	for _, c := range certs {
		pool.AddCert(c)
	}
	return pool
}

// extractAttestationChallenge 从证书 KeyDescription 扩展中提取 challenge（首段 OCTET STRING）。
// KeyDescription ::= SEQUENCE { challenge OCTET STRING, ... }。
func extractAttestationChallenge(cert *x509.Certificate) ([]byte, error) {
	for _, ext := range cert.Extensions {
		if ext.Id.Equal(androidKeyAttestationOID) {
			return firstOctetString(ext.Value)
		}
	}
	return nil, ErrAttestation
}

// firstOctetString 在 DER 中递归查找首个 OCTET STRING 的内容。
func firstOctetString(der []byte) ([]byte, error) {
	var rv asn1.RawValue
	if _, err := asn1.Unmarshal(der, &rv); err != nil {
		return nil, err
	}
	rest := rv.Bytes
	for len(rest) > 0 {
		var f asn1.RawValue
		var err error
		rest, err = asn1.Unmarshal(rest, &f)
		if err != nil {
			return nil, err
		}
		if f.Class == asn1.ClassUniversal && f.Tag == asn1.TagOctetString {
			return f.Bytes, nil
		}
		if f.IsCompound {
			if inner, ierr := firstOctetString(f.FullBytes); ierr == nil {
				return inner, nil
			}
		}
	}
	return nil, ErrAttestation
}

// ---------------------------------------------------------------------------
// 3) 验证器注册表（按 root 分发 + 部署白名单）
// ---------------------------------------------------------------------------

// Registry 按 root 分发 attestation 验证，并 enforce 部署允许列表。
type Registry struct {
	verifiers map[string]AttestationVerifier
	allowed   map[string]bool
}

// NewRegistry 用注入的验证器构造注册表；allowed 为允许的根集合（空=全部允许）。
func NewRegistry(verifiers []AttestationVerifier, allowed []string) *Registry {
	m := make(map[string]AttestationVerifier, len(verifiers))
	for _, v := range verifiers {
		m[v.Root()] = v
	}
	allow := make(map[string]bool, len(allowed))
	for _, r := range allowed {
		allow[r] = true
	}
	return &Registry{verifiers: m, allowed: allow}
}

// Root 仅用于满足 AttestationVerifier 接口（注册表本身按 root 分发）。
func (r *Registry) Root() string { return "registry" }

// Verify 校验 attestation：未知 root / 未允许 root / 验证失败均拒绝。
func (r *Registry) Verify(ctx context.Context, c AttestationClaim) error {
	v, ok := r.verifiers[c.Root]
	if !ok {
		return ErrUnknownRoot
	}
	if len(r.allowed) > 0 && !r.allowed[c.Root] {
		return ErrRootNotAllowed
	}
	return v.Verify(ctx, c)
}

// ---------------------------------------------------------------------------
// 4) 生产构造：从环境变量加载根公钥（PEM），默认允许全部三根。
// ---------------------------------------------------------------------------

// allRoots 为所有支持的根标识。
var allRoots = []string{"play_integrity", "huawei", "android_key_attestation"}

// NewRegistryFromEnv 按部署配置构造注册表：
//   - ORACLE_ACCEPT_ROOTS：逗号分隔的允许根（默认全部）。
//   - ORACLE_GOOGLE_PUBKEY[_FILE]：Play Integrity 根公钥（PEM，ES256）。
//   - ORACLE_HW_PUBKEY[_FILE]：华为根公钥（PEM，ES256）。
//   - ORACLE_ANDROID_ROOTS_FILE：Android Key Attestation 可信根 CA（PEM 包）。
//
// 某根被允许但未配置对应公钥时，其验证器会直接拒绝（fail-closed）。
func NewRegistryFromEnv() *Registry {
	allowed := allRoots
	if env := os.Getenv("ORACLE_ACCEPT_ROOTS"); env != "" {
		allowed = strings.Split(env, ",")
		for i := range allowed {
			allowed[i] = strings.TrimSpace(allowed[i])
		}
	}
	var vs []AttestationVerifier
	if pub := loadECDSAPubFromEnv("ORACLE_GOOGLE_PUBKEY", "ORACLE_GOOGLE_PUBKEY_FILE"); pub != nil {
		vs = append(vs, NewSignedTokenVerifier("play_integrity", pub))
	}
	if pub := loadECDSAPubFromEnv("ORACLE_HW_PUBKEY", "ORACLE_HW_PUBKEY_FILE"); pub != nil {
		vs = append(vs, NewSignedTokenVerifier("huawei", pub))
	}
	if roots := loadCertPoolFromFile(os.Getenv("ORACLE_ANDROID_ROOTS_FILE")); roots != nil {
		vs = append(vs, NewAndroidKAVerifier(roots))
	}
	return NewRegistry(vs, allowed)
}

// loadECDSAPubFromEnv 优先读取 *FILE，回退到直接 PEM 字符串。
func loadECDSAPubFromEnv(envKey, fileKey string) *ecdsa.PublicKey {
	var pemBytes []byte
	if f := os.Getenv(fileKey); f != "" {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil
		}
		pemBytes = b
	} else if s := os.Getenv(envKey); s != "" {
		pemBytes = []byte(s)
	} else {
		return nil
	}
	return parseECDSAPublicPEM(pemBytes)
}

// parseECDSAPublicPEM 从 PEM 解析 ECDSA 公钥（支持 PUBLIC KEY 与 CERTIFICATE）。
func parseECDSAPublicPEM(pemBytes []byte) *ecdsa.PublicKey {
	for {
		block, rest := pem.Decode(pemBytes)
		if block == nil {
			break
		}
		if block.Type == "PUBLIC KEY" {
			pub, err := x509.ParsePKIXPublicKey(block.Bytes)
			if err == nil {
				if k, ok := pub.(*ecdsa.PublicKey); ok {
					return k
				}
			}
		}
		if block.Type == "CERTIFICATE" {
			c, err := x509.ParseCertificate(block.Bytes)
			if err == nil {
				if k, ok := c.PublicKey.(*ecdsa.PublicKey); ok {
					return k
				}
			}
		}
		pemBytes = rest
	}
	return nil
}

// loadCertPoolFromFile 从 PEM 文件加载 CA 池（支持多证书）。
func loadCertPoolFromFile(path string) *x509.CertPool {
	if path == "" {
		return nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	pool := x509.NewCertPool()
	if pool.AppendCertsFromPEM(b) {
		return pool
	}
	return nil
}
