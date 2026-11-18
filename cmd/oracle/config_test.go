package main

import (
	"os"
	"path/filepath"
	"testing"
)

// clearOracleEnv 清空所有可能影响 resolveConfig 的环境变量。
func clearOracleEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"ORACLE_STRICT", "ORACLE_ENV", "ORACLE_CHAIN_ID", "ORACLE_NODE",
		"ORACLE_SIGNER_MNEMONIC", "ORACLE_KEYRING_BACKEND", "ORACLE_KEYRING_DIR",
		"ORACLE_KEYRING_PASSWORD_FILE", "ORACLE_VERIFIER", "ORACLE_TEE_ROOTS_FILE",
		"ORACLE_TEE_REVOCATION_FILE", "ORACLE_TEE_MIN_SECURITY_LEVEL",
		"ORACLE_API_KEY", "ORACLE_BIND_ADDR", "ORACLE_LISTEN_PORT",
		"ORACLE_TRUSTED_PROXY", "ORACLE_WAIT_COMMIT", "ORACLE_WAIT_COMMIT_TIMEOUT",
		"ORACLE_GAS_PRICES",
	} {
		t.Setenv(k, "")
	}
}

// P0-03/P1-09：生产（strict）启动闸门的必填项与禁止项。
func TestResolveConfig_StrictGates(t *testing.T) {
	dir := t.TempDir()
	ca := newTestCA(t)
	roots := filepath.Join(dir, "roots.pem")
	if err := os.WriteFile(roots, []byte(ca.pemCert), 0o600); err != nil {
		t.Fatal(err)
	}
	password := filepath.Join(dir, "keyring.pass")
	if err := os.WriteFile(password, []byte("test-pass\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	baseStrict := func(t *testing.T) {
		clearOracleEnv(t)
		t.Setenv("ORACLE_STRICT", "1")
		t.Setenv("ORACLE_CHAIN_ID", "mcchain-mainnet-1")
		t.Setenv("ORACLE_NODE", "tcp://127.0.0.1:26657")
		t.Setenv("ORACLE_API_KEY", "test-api-key")
		t.Setenv("ORACLE_KEYRING_DIR", dir)
	}

	t.Run("strict 完整配置通过", func(t *testing.T) {
		baseStrict(t)
		t.Setenv("ORACLE_VERIFIER", "tee")
		t.Setenv("ORACLE_TEE_ROOTS_FILE", roots)
		t.Setenv("ORACLE_KEYRING_BACKEND", "file")
		t.Setenv("ORACLE_KEYRING_PASSWORD_FILE", password)

		cfg, err := resolveConfig("", "", 8080)
		if err != nil {
			t.Fatalf("valid strict config rejected: %v", err)
		}
		if cfg.verifierMode != "tee" || cfg.keyringBackend != "file" || !cfg.strict {
			t.Fatalf("unexpected config: %+v", cfg)
		}
		if cfg.gasPrices != "0.0025umc" {
			t.Fatalf("default gas price must match production docs, got %q", cfg.gasPrices)
		}
	})

	t.Run("非法 gas price 拒绝", func(t *testing.T) {
		baseStrict(t)
		t.Setenv("ORACLE_VERIFIER", "tee")
		t.Setenv("ORACLE_TEE_ROOTS_FILE", roots)
		t.Setenv("ORACLE_KEYRING_BACKEND", "file")
		t.Setenv("ORACLE_KEYRING_PASSWORD_FILE", password)
		t.Setenv("ORACLE_GAS_PRICES", "not-a-coin")
		if _, err := resolveConfig("", "", 8080); err == nil {
			t.Fatal("invalid ORACLE_GAS_PRICES must be rejected")
		}
	})

	t.Run("strict 禁止 soft verifier", func(t *testing.T) {
		baseStrict(t)
		t.Setenv("ORACLE_VERIFIER", "soft")
		t.Setenv("ORACLE_KEYRING_BACKEND", "file")
		t.Setenv("ORACLE_KEYRING_PASSWORD_FILE", password)
		if _, err := resolveConfig("", "", 8080); err == nil {
			t.Fatal("strict + soft must be rejected")
		}
	})

	t.Run("strict tee 缺可信根", func(t *testing.T) {
		baseStrict(t)
		t.Setenv("ORACLE_VERIFIER", "tee")
		t.Setenv("ORACLE_KEYRING_BACKEND", "file")
		t.Setenv("ORACLE_KEYRING_PASSWORD_FILE", password)
		if _, err := resolveConfig("", "", 8080); err == nil {
			t.Fatal("strict + tee without roots must be rejected")
		}
	})

	t.Run("strict 禁止助记词常驻", func(t *testing.T) {
		baseStrict(t)
		t.Setenv("ORACLE_VERIFIER", "tee")
		t.Setenv("ORACLE_TEE_ROOTS_FILE", roots)
		t.Setenv("ORACLE_KEYRING_BACKEND", "file")
		t.Setenv("ORACLE_KEYRING_PASSWORD_FILE", password)
		t.Setenv("ORACLE_SIGNER_MNEMONIC", "a b c d e f g h i j k l")
		if _, err := resolveConfig("", "", 8080); err == nil {
			t.Fatal("strict + mnemonic must be rejected")
		}
	})

	t.Run("strict 禁止 test keyring 后端", func(t *testing.T) {
		baseStrict(t)
		t.Setenv("ORACLE_VERIFIER", "tee")
		t.Setenv("ORACLE_TEE_ROOTS_FILE", roots)
		t.Setenv("ORACLE_KEYRING_BACKEND", "test")
		if _, err := resolveConfig("", "", 8080); err == nil {
			t.Fatal("strict + test keyring must be rejected")
		}
	})

	t.Run("strict file 后端要求口令文件", func(t *testing.T) {
		baseStrict(t)
		t.Setenv("ORACLE_VERIFIER", "tee")
		t.Setenv("ORACLE_TEE_ROOTS_FILE", roots)
		t.Setenv("ORACLE_KEYRING_BACKEND", "file")
		if _, err := resolveConfig("", "", 8080); err == nil {
			t.Fatal("strict + file keyring without password file must be rejected")
		}
	})
}

// 非生产：保留助记词便捷路径，但缺助记词仍拒绝。
func TestResolveConfig_Development(t *testing.T) {
	clearOracleEnv(t)
	t.Setenv("ORACLE_CHAIN_ID", "mcchain-dev-1")
	t.Setenv("ORACLE_NODE", "tcp://127.0.0.1:26657")
	t.Setenv("ORACLE_KEYRING_DIR", t.TempDir())

	if _, err := resolveConfig("", "", 8080); err == nil {
		t.Fatal("development without mnemonic must be rejected")
	}

	t.Setenv("ORACLE_SIGNER_MNEMONIC", "a b c d e f g h i j k l")
	if _, err := resolveConfig("", "", 8080); err != nil {
		t.Fatalf("development with mnemonic should pass: %v", err)
	}
}
