package oraclesvc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// --- 工具：生成 ES256 密钥并构造 JWS 令牌（nonce = base64(device|challenge)） ---

func genES256(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return k
}

// makeJWS 用 priv 对 claims 做 ES256 紧凑签名（R||S，64 字节）。
func makeJWS(t *testing.T, priv *ecdsa.PrivateKey, claims map[string]interface{}) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"ES256","typ":"JWT"}`))
	payload, err := json.Marshal(claims)
	require.NoError(t, err)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
	msg := []byte(header + "." + payloadB64)
	h := sha256.Sum256(msg)
	r, s, err := ecdsa.Sign(rand.Reader, priv, h[:])
	require.NoError(t, err)
	sig := append(intToBytes(r, 32), intToBytes(s, 32)...)
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)
	return header + "." + payloadB64 + "." + sigB64
}

func intToBytes(v *big.Int, size int) []byte {
	b := v.Bytes()
	if len(b) >= size {
		return b[len(b)-size:]
	}
	out := make([]byte, size)
	copy(out[size-len(b):], b)
	return out
}

func jwsAttestation(t *testing.T, root string, priv *ecdsa.PrivateKey, deviceAddr, challenge string) AttestationClaim {
	t.Helper()
	nonce := base64.StdEncoding.EncodeToString([]byte(deviceAddr + "|" + challenge))
	claims := map[string]interface{}{
		"requestDetails": map[string]interface{}{"nonce": nonce},
		"deviceIntegrity": map[string]interface{}{"deviceRecognition": "MEETS_DEVICE_INTEGRITY"},
	}
	return AttestationClaim{
		Root:       root,
		Payload:    makeJWS(t, priv, claims),
		Challenge:  challenge,
		DeviceAddr: deviceAddr,
	}
}

// --- Play Integrity / 华为：ES256 JWS 验证 ---

func TestSignedTokenVerifier_Accept(t *testing.T) {
	priv := genES256(t)
	v := NewSignedTokenVerifier("play_integrity", &priv.PublicKey)
	c := jwsAttestation(t, "play_integrity", priv, "mcDeviceA", "chal-1")
	require.NoError(t, v.Verify(context.Background(), c))
}

func TestSignedTokenVerifier_WrongNonce(t *testing.T) {
	priv := genES256(t)
	v := NewSignedTokenVerifier("play_integrity", &priv.PublicKey)
	c := jwsAttestation(t, "play_integrity", priv, "mcDeviceA", "chal-1")
	// 篡改 challenge：nonce 绑定不匹配
	c.Challenge = "chal-TAMPERED"
	require.ErrorIs(t, v.Verify(context.Background(), c), ErrAttestation)
}

func TestSignedTokenVerifier_BadSignature(t *testing.T) {
	priv := genES256(t)
	other := genES256(t) // 用另一把私钥签名 → 验签失败
	v := NewSignedTokenVerifier("play_integrity", &priv.PublicKey)
	c := jwsAttestation(t, "play_integrity", other, "mcDeviceA", "chal-1")
	require.ErrorIs(t, v.Verify(context.Background(), c), ErrAttestation)
}

func TestSignedTokenVerifier_HuaweiRoot(t *testing.T) {
	priv := genES256(t)
	v := NewSignedTokenVerifier("huawei", &priv.PublicKey)
	// 华为用顶层 nonce 字段同样结构
	nonce := base64.StdEncoding.EncodeToString([]byte("mcDeviceB|chal-2"))
	claims := map[string]interface{}{"nonce": nonce}
	c := AttestationClaim{Root: "huawei", Payload: makeJWS(t, priv, claims), Challenge: "chal-2", DeviceAddr: "mcDeviceB"}
	require.NoError(t, v.Verify(context.Background(), c))
}

// --- Android Key Attestation：X.509 链 + challenge 扩展 ---

// makeAndroidKACertChain 生成「可信根 → 叶」两级链，叶证书携带 KeyDescription 扩展，
// 其 challenge 字段 = device|challenge。用于验证 androidKAVerifier。
func makeAndroidKACertChain(t *testing.T, deviceAddr, challenge string) (leafPEM string, rootPEM string) {
	t.Helper()
	// 根 CA
	rootKey := genES256(t)
	rootTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "MC Test Root CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTmpl, rootTmpl, &rootKey.PublicKey, rootKey)
	require.NoError(t, err)
	rootCert, err := x509.ParseCertificate(rootDER)
	require.NoError(t, err)

	// 叶（设备硬件密钥）
	leafKey := genES256(t)
	// KeyDescription ::= SEQUENCE { challenge OCTET STRING, ... }
	challengeBytes := []byte(deviceAddr + "|" + challenge)
	kd, err := asn1.Marshal(struct {
		Challenge []byte `asn1:"octet"`
	}{Challenge: challengeBytes})
	require.NoError(t, err)
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "MC Device Attestation"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtraExtensions: []pkix.Extension{
			{Id: androidKeyAttestationOID, Critical: false, Value: kd},
		},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, rootCert, &leafKey.PublicKey, rootKey)
	require.NoError(t, err)

	leafPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}))
	rootPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootDER}))
	return
}

func TestAndroidKAVerifier_Accept(t *testing.T) {
	leafPEM, rootPEM := makeAndroidKACertChain(t, "mcDeviceC", "chal-3")
	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM([]byte(rootPEM)))
	v := NewAndroidKAVerifier(pool)
	c := AttestationClaim{Root: "android_key_attestation", Payload: leafPEM, Challenge: "chal-3", DeviceAddr: "mcDeviceC"}
	require.NoError(t, v.Verify(context.Background(), c))
}

func TestAndroidKAVerifier_BadChain(t *testing.T) {
	leafPEM, _ := makeAndroidKACertChain(t, "mcDeviceC", "chal-3")
	// 用一把无关根构造池（不匹配叶证书的签发者）→ 链校验失败
	otherKey := genES256(t)
	otherTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(9),
		Subject:               pkix.Name{CommonName: "Untrusted CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	otherDER, err := x509.CreateCertificate(rand.Reader, otherTmpl, otherTmpl, &otherKey.PublicKey, otherKey)
	require.NoError(t, err)
	otherPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: otherDER})
	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM(otherPEM))

	v := NewAndroidKAVerifier(pool)
	c := AttestationClaim{Root: "android_key_attestation", Payload: leafPEM, Challenge: "chal-3", DeviceAddr: "mcDeviceC"}
	require.ErrorIs(t, v.Verify(context.Background(), c), ErrAttestation)
}

func TestAndroidKAVerifier_WrongChallenge(t *testing.T) {
	leafPEM, rootPEM := makeAndroidKACertChain(t, "mcDeviceC", "chal-3")
	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM([]byte(rootPEM)))
	v := NewAndroidKAVerifier(pool)
	// challenge 不匹配嵌入扩展 → 拒绝
	c := AttestationClaim{Root: "android_key_attestation", Payload: leafPEM, Challenge: "chal-WRONG", DeviceAddr: "mcDeviceC"}
	require.ErrorIs(t, v.Verify(context.Background(), c), ErrAttestation)
}

// --- Registry 分发 ---

func TestRegistry_DispatchAndReject(t *testing.T) {
	priv := genES256(t)
	vs := []AttestationVerifier{
		NewSignedTokenVerifier("play_integrity", &priv.PublicKey),
	}
	reg := NewRegistry(vs, nil) // 全部允许

	// 已知 root 且验证通过
	good := jwsAttestation(t, "play_integrity", priv, "mcD", "chal-4")
	require.NoError(t, reg.Verify(context.Background(), good))

	// 未知 root
	unknown := jwsAttestation(t, "no_such_root", priv, "mcD", "chal-4")
	require.ErrorIs(t, reg.Verify(context.Background(), unknown), ErrUnknownRoot)

	// 允许列表排除 play_integrity
	regLimited := NewRegistry(vs, []string{"huawei"})
	require.ErrorIs(t, regLimited.Verify(context.Background(), good), ErrRootNotAllowed)
}
