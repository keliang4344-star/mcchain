package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type testCA struct {
	cert    *x509.Certificate
	key     *ecdsa.PrivateKey
	pemCert string
}

func newTestCA(t *testing.T) *testCA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "mcchain-test-root"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create CA cert: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse CA cert: %v", err)
	}
	return &testCA{
		cert:    cert,
		key:     key,
		pemCert: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
	}
}

func (ca *testCA) newLeaf(t *testing.T, notBefore, notAfter time.Time) (*x509.Certificate, *ecdsa.PrivateKey, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate leaf key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(4242),
		Subject:      pkix.Name{CommonName: "mcchain-test-device"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		t.Fatalf("create leaf cert: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse leaf cert: %v", err)
	}
	return cert, key, string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func writeFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

// buildProof 构造并签名一条合法的 TEE 证明。
func buildProof(t *testing.T, chainID, deviceID string, leafPEM string, leafKey *ecdsa.PrivateKey, expires time.Time) (string, TeeProof) {
	t.Helper()
	tp := TeeProof{
		DeviceID:      deviceID,
		ChainID:       chainID,
		Purpose:       teeProofPurpose,
		Nonce:         "nonce-" + deviceID,
		ExpiresAt:     expires.Unix(),
		SecurityLevel: "tee",
		CertChainPEM:  []string{leafPEM},
	}
	digest := sha256.Sum256(canonicalProofPayload(&tp))
	sig, err := ecdsa.SignASN1(rand.Reader, leafKey, digest[:])
	if err != nil {
		t.Fatalf("sign proof: %v", err)
	}
	tp.Signature = base64.StdEncoding.EncodeToString(sig)
	raw, err := json.Marshal(tp)
	if err != nil {
		t.Fatalf("marshal proof: %v", err)
	}
	return base64.StdEncoding.EncodeToString(raw), tp
}

func newVerifier(t *testing.T, ca *testCA) *TeeVerifier {
	t.Helper()
	dir := t.TempDir()
	roots := writeFile(t, dir, "roots.pem", []byte(ca.pemCert))
	v, err := NewTeeVerifier(TeeVerifierConfig{ChainID: "mcchain-mainnet-1", RootsFile: roots})
	if err != nil {
		t.Fatalf("NewTeeVerifier: %v", err)
	}
	return v
}

func TestTeeVerifier_AcceptsValidProof(t *testing.T) {
	ca := newTestCA(t)
	leaf, key, leafPEM := ca.newLeaf(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	_ = leaf
	v := newVerifier(t, ca)

	proof, _ := buildProof(t, "mcchain-mainnet-1", "mc-device-1", leafPEM, key, time.Now().Add(3*time.Minute))
	ok, reason := v.Verify("mc-device-1", proof, "device-signature")
	if !ok {
		t.Fatalf("valid proof rejected: %s", reason)
	}
}

func TestTeeVerifier_RejectsBadContext(t *testing.T) {
	ca := newTestCA(t)
	_, key, leafPEM := ca.newLeaf(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))

	cases := []struct {
		name     string
		deviceID string
		mutate   func(*TeeProof)
	}{
		{"device mismatch", "other-device", func(*TeeProof) {}},
		{"chain mismatch", "mc-device-1", func(p *TeeProof) { p.ChainID = "mcchain-testnet-1" }},
		{"wrong purpose", "mc-device-1", func(p *TeeProof) { p.Purpose = "other" }},
		{"low security level", "mc-device-1", func(p *TeeProof) { p.SecurityLevel = "software" }},
		{"expired", "mc-device-1", func(p *TeeProof) {}},
		{"ttl too long", "mc-device-1", func(p *TeeProof) {}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := newVerifier(t, ca)
			expires := time.Now().Add(3 * time.Minute)
			switch tc.name {
			case "expired":
				expires = time.Now().Add(-10 * time.Minute)
			case "ttl too long":
				expires = time.Now().Add(time.Hour)
			}
			proof, tp := buildProof(t, "mcchain-mainnet-1", "mc-device-1", leafPEM, key, expires)
			if tc.mutate != nil && (tc.name == "chain mismatch" || tc.name == "wrong purpose" || tc.name == "low security level") {
				// 重新签名：上下文变更后必须由设备重新出证，这里模拟「已签名的错误上下文」。
				tc.mutate(&tp)
				digest := sha256.Sum256(canonicalProofPayload(&tp))
				sig, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
				if err != nil {
					t.Fatalf("sign: %v", err)
				}
				tp.Signature = base64.StdEncoding.EncodeToString(sig)
				raw, _ := json.Marshal(tp)
				proof = base64.StdEncoding.EncodeToString(raw)
			}
			if ok, _ := v.Verify(tc.deviceID, proof, "sig"); ok {
				t.Fatalf("%s: proof must be rejected", tc.name)
			}
		})
	}
}

func TestTeeVerifier_RejectsUnknownRoot(t *testing.T) {
	trusted := newTestCA(t)
	rogue := newTestCA(t)
	_, key, leafPEM := rogue.newLeaf(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	v := newVerifier(t, trusted)

	proof, _ := buildProof(t, "mcchain-mainnet-1", "mc-device-1", leafPEM, key, time.Now().Add(3*time.Minute))
	if ok, reason := v.Verify("mc-device-1", proof, "sig"); ok {
		t.Fatalf("proof signed by unknown root accepted: %s", reason)
	}
}

func TestTeeVerifier_RejectsRevokedCertificate(t *testing.T) {
	ca := newTestCA(t)
	leaf, key, leafPEM := ca.newLeaf(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	dir := t.TempDir()
	roots := writeFile(t, dir, "roots.pem", []byte(ca.pemCert))
	fp := sha256.Sum256(leaf.Raw)
	rev := writeFile(t, dir, "revoked.json", []byte(`{"revoked_fingerprints":["`+hex.EncodeToString(fp[:])+`"]}`))

	v, err := NewTeeVerifier(TeeVerifierConfig{ChainID: "mcchain-mainnet-1", RootsFile: roots, RevocationFile: rev})
	if err != nil {
		t.Fatalf("NewTeeVerifier: %v", err)
	}
	proof, _ := buildProof(t, "mcchain-mainnet-1", "mc-device-1", leafPEM, key, time.Now().Add(3*time.Minute))
	if ok, reason := v.Verify("mc-device-1", proof, "sig"); ok {
		t.Fatalf("revoked certificate accepted: %s", reason)
	}
}

func TestTeeVerifier_RejectsReplay(t *testing.T) {
	ca := newTestCA(t)
	_, key, leafPEM := ca.newLeaf(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	v := newVerifier(t, ca)

	proof, _ := buildProof(t, "mcchain-mainnet-1", "mc-device-1", leafPEM, key, time.Now().Add(3*time.Minute))
	if ok, reason := v.Verify("mc-device-1", proof, "sig"); !ok {
		t.Fatalf("first verification failed: %s", reason)
	}
	if ok, reason := v.Verify("mc-device-1", proof, "sig"); ok {
		t.Fatalf("replayed proof accepted: %s", reason)
	}
}

func TestTeeVerifier_RejectsTamperedSignature(t *testing.T) {
	ca := newTestCA(t)
	_, key, leafPEM := ca.newLeaf(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	v := newVerifier(t, ca)

	proof, tp := buildProof(t, "mcchain-mainnet-1", "mc-device-1", leafPEM, key, time.Now().Add(3*time.Minute))
	tp.Nonce = "tampered-nonce"
	raw, _ := json.Marshal(tp)
	if ok, _ := v.Verify("mc-device-1", base64.StdEncoding.EncodeToString(raw), "sig"); ok {
		t.Fatal("tampered payload accepted")
	}
	_ = proof
}

func TestNewTeeVerifier_RequiresRoots(t *testing.T) {
	if _, err := NewTeeVerifier(TeeVerifierConfig{ChainID: "mcchain-mainnet-1"}); err == nil {
		t.Fatal("missing roots file must fail")
	}
	if _, err := NewTeeVerifier(TeeVerifierConfig{ChainID: "mcchain-mainnet-1", RootsFile: filepath.Join(t.TempDir(), "nope.pem")}); err == nil {
		t.Fatal("nonexistent roots file must fail")
	}
}
