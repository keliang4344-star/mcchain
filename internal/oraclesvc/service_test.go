package oraclesvc

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/stretchr/testify/require"
)

// 固定预言机私钥（32 字节 hex），保证测试可复现且与链侧验签一致。
const testOracleKeyHex = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

// allowAllVerifier 仅用于 service 层联调：放行任意 attestation，使 /sign 能走到签名逻辑。
// 真实的验证强度由 attestation_test.go 中 signedTokenVerifier / androidKAVerifier 覆盖。
type allowAllVerifier struct{}

func (allowAllVerifier) Root() string { return "test" }
func (allowAllVerifier) Verify(_ context.Context, _ AttestationClaim) error { return nil }

// rejectAllVerifier 用于验证 /sign 在 attestation 不通过时拒绝（fail-closed）。
type rejectAllVerifier struct{}

func (rejectAllVerifier) Root() string { return "test" }
func (rejectAllVerifier) Verify(_ context.Context, _ AttestationClaim) error { return ErrAttestation }

func newTestHandler(t *testing.T) (http.Handler, *secp256k1.PrivKey) {
	t.Helper()
	// 注意：SDK 全局 Bech32 前缀配置在进程内只能 Seal 一次，此处不再设置（验签不依赖前缀）。
	bz := mustHexDecode(t, testOracleKeyHex)
	priv := &secp256k1.PrivKey{Key: bz}
	pub := priv.PubKey().(*secp256k1.PubKey)
	addr := sdk.AccAddress(pub.Address()).String()
	pubB64 := base64.StdEncoding.EncodeToString(pub.Bytes())
	return NewHandler(priv, addr, pubB64, allowAllVerifier{}), priv
}

func mustHexDecode(t *testing.T, h string) []byte {
	t.Helper()
	b, err := hex.DecodeString(h)
	require.NoError(t, err)
	require.Len(t, b, 32)
	return b
}

// signBody 构造带 attestation 的 /sign 请求体（attestation 由 allowAllVerifier 放行）。
func signBody(deviceAddr, challenge string) string {
	return `{"device_addr":"` + deviceAddr + `","challenge":"` + challenge + `","attestation":{"root":"play_integrity","payload":"test"}}`
}

func TestHealthz(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "ok", rec.Body.String())
}

func TestPubkey(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/pubkey", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.NotEmpty(t, body["address"])
	require.NotEmpty(t, body["pubkey_base64"])
	// 公钥 base64 解码后应为 33 字节压缩 secp256k1 公钥
	pk, err := base64.StdEncoding.DecodeString(body["pubkey_base64"])
	require.NoError(t, err)
	require.Len(t, pk, 33)
}

func TestSignAndVerifyConsistencyWithChain(t *testing.T) {
	h, priv := newTestHandler(t)

	deviceAddr := "mc1abc123def456"
	challenge := "challenge-xyz-987"

	// 请求签名（含 attestation，经 allowAllVerifier 放行）
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/sign", strings.NewReader(signBody(deviceAddr, challenge))))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		SignatureBase64 string `json:"signature_base64"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.SignatureBase64)

	// 链侧一致：对 `deviceAddr|challenge` 验签，必须为真
	sig, err := base64.StdEncoding.DecodeString(resp.SignatureBase64)
	require.NoError(t, err)
	msg := []byte(deviceAddr + "|" + challenge)
	require.True(t, priv.PubKey().VerifySignature(msg, sig), "oracle 签名必须能被链侧 TeeOracle 验签通过")

	// 负向：篡改消息后验签必须失败（防伪造）
	require.False(t, priv.PubKey().VerifySignature([]byte(deviceAddr+"|"+challenge+"tampered"), sig))
}

func TestSignRejectsMissingFields(t *testing.T) {
	h, _ := newTestHandler(t)

	// 缺 challenge → 400（与 attestation 无关，参数校验在前）
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/sign", strings.NewReader(`{"device_addr":"x"}`)))
	require.Equal(t, http.StatusBadRequest, rec.Code)

	// GET 方法 → 405
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/sign", nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec2.Code)
}

// TestSignRejectsBadAttestation 验证 P0② fail-closed：attestation 不通过时 /sign 返回 403。
func TestSignRejectsBadAttestation(t *testing.T) {
	priv := &secp256k1.PrivKey{Key: mustHexDecode(t, testOracleKeyHex)}
	pub := priv.PubKey().(*secp256k1.PubKey)
	addr := sdk.AccAddress(pub.Address()).String()
	pubB64 := base64.StdEncoding.EncodeToString(pub.Bytes())
	// 用 rejectAllVerifier 模拟「未通过真机验证」
	h := NewSecureHandler(priv, addr, pubB64, "", 0, rejectAllVerifier{})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/sign", strings.NewReader(signBody("mc1abc", "c1"))))
	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestSignRequiresBearer(t *testing.T) {
	priv := &secp256k1.PrivKey{Key: mustHexDecode(t, testOracleKeyHex)}
	pub := priv.PubKey().(*secp256k1.PubKey)
	addr := sdk.AccAddress(pub.Address()).String()
	pubB64 := base64.StdEncoding.EncodeToString(pub.Bytes())
	h := NewSecureHandler(priv, addr, pubB64, "topsecret", 0, allowAllVerifier{})

	body := strings.NewReader(signBody("mc1abc", "c1"))

	// 无 token → 401
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/sign", body))
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	// 错误 token → 401
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/sign", strings.NewReader(signBody("mc1abc", "c1")))
	req2.Header.Set("Authorization", "Bearer wrong")
	h.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusUnauthorized, rec2.Code)

	// 正确 token → 200 且签名可验
	rec3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodPost, "/sign", strings.NewReader(signBody("mc1abc", "c1")))
	req3.Header.Set("Authorization", "Bearer topsecret")
	h.ServeHTTP(rec3, req3)
	require.Equal(t, http.StatusOK, rec3.Code)
	var resp struct {
		SignatureBase64 string `json:"signature_base64"`
	}
	require.NoError(t, json.Unmarshal(rec3.Body.Bytes(), &resp))
	sig, err := base64.StdEncoding.DecodeString(resp.SignatureBase64)
	require.NoError(t, err)
	require.True(t, priv.PubKey().VerifySignature([]byte("mc1abc|"+"c1"), sig))
}

func TestRateLimit(t *testing.T) {
	priv := &secp256k1.PrivKey{Key: mustHexDecode(t, testOracleKeyHex)}
	pub := priv.PubKey().(*secp256k1.PubKey)
	addr := sdk.AccAddress(pub.Address()).String()
	pubB64 := base64.StdEncoding.EncodeToString(pub.Bytes())
	h := NewSecureHandler(priv, addr, pubB64, "", 2, allowAllVerifier{}) // 每分钟 2 次

	doSign := func() int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/sign", strings.NewReader(signBody("mc1abc", "c1"))))
		return rec.Code
	}
	require.Equal(t, http.StatusOK, doSign())
	require.Equal(t, http.StatusOK, doSign())
	// 第 3 次触发限流
	require.Equal(t, http.StatusTooManyRequests, doSign())
}
