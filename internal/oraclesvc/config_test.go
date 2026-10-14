package oraclesvc

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 每次测试前清空与配置相关的所有环境变量，避免宿主环境泄漏。
func resetConfigEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"ORACLE_STRICT", "ORACLE_ENV", "ORACLE_ALLOW_PLAINTEXT",
		"ORACLE_SIGN_TOKEN", "ORACLE_TLS_CERT", "ORACLE_TLS_KEY",
		"ORACLE_RATE_LIMIT", "ORACLE_ACCEPT_ROOTS",
		"ORACLE_HEALTH_INTERVAL", "ORACLE_HEALTH_STALE",
		"ORACLE_KEY", "ORACLE_KEY_FILE",
		"ORACLE_GOOGLE_PUBKEY", "ORACLE_GOOGLE_PUBKEY_FILE",
		"ORACLE_HW_PUBKEY", "ORACLE_HW_PUBKEY_FILE",
		"ORACLE_ANDROID_ROOTS_FILE",
	} {
		t.Setenv(k, "")
	}
}

// TestLoadRunConfigBaseline：最简合法配置（非严格模式）可加载。
func TestLoadRunConfigBaseline(t *testing.T) {
	resetConfigEnv(t)
	t.Setenv("ORACLE_SIGN_TOKEN", "tok")

	cfg, err := LoadRunConfig(":8080")
	if err != nil {
		t.Fatalf("LoadRunConfig: %v", err)
	}
	if cfg.Listen != ":8080" {
		t.Fatalf("listen = %q, want :8080", cfg.Listen)
	}
	if cfg.Strict {
		t.Fatal("should not be strict without ORACLE_STRICT/ORACLE_ENV")
	}
	if cfg.HealthInterval <= 0 || cfg.StaleThreshold <= 0 {
		t.Fatalf("health defaults missing: %v / %v", cfg.HealthInterval, cfg.StaleThreshold)
	}
}

// TestLoadRunConfigListenValidation：监听地址必须合法。
func TestLoadRunConfigListenValidation(t *testing.T) {
	resetConfigEnv(t)

	for _, bad := range []string{"", "   ", "8080", "host:port", "127.0.0.1:0", "127.0.0.1:70000"} {
		if _, err := LoadRunConfig(bad); err == nil {
			t.Fatalf("want error for listen %q", bad)
		}
	}

	for _, good := range []string{":8080", "127.0.0.1:8080", "localhost:9090", "0.0.0.0:1317"} {
		if _, err := LoadRunConfig(good); err != nil {
			t.Fatalf("unexpected error for listen %q: %v", good, err)
		}
	}
}

// TestLoadRunConfigRateLimit：非法限流值必须报错，绝不静默回退成不限流。
func TestLoadRunConfigRateLimit(t *testing.T) {
	resetConfigEnv(t)

	t.Setenv("ORACLE_RATE_LIMIT", "abc")
	if _, err := LoadRunConfig(":8080"); err == nil {
		t.Fatal("want error for non-integer rate limit")
	}

	t.Setenv("ORACLE_RATE_LIMIT", "-5")
	if _, err := LoadRunConfig(":8080"); err == nil {
		t.Fatal("want error for negative rate limit")
	}

	t.Setenv("ORACLE_RATE_LIMIT", "60")
	cfg, err := LoadRunConfig(":8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RateLimit != 60 {
		t.Fatalf("rate limit = %d, want 60", cfg.RateLimit)
	}
}

// TestLoadRunConfigTLS：TLS 必须成对；路径必须可读。
func TestLoadRunConfigTLS(t *testing.T) {
	resetConfigEnv(t)

	t.Setenv("ORACLE_TLS_CERT", "/tmp/cert.pem")
	if _, err := LoadRunConfig(":8080"); err == nil {
		t.Fatal("want error when cert set but key missing")
	}

	t.Setenv("ORACLE_TLS_KEY", "/tmp/key.pem")
	if _, err := LoadRunConfig(":8080"); err == nil {
		t.Fatal("want error when TLS paths do not exist")
	}

	// 两个都存在但路径不可读 → 仍报错
	dir := t.TempDir()
	cert := filepath.Join(dir, "cert.pem")
	key := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(cert, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(key, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ORACLE_TLS_CERT", cert)
	t.Setenv("ORACLE_TLS_KEY", key)
	if _, err := LoadRunConfig(":8080"); err != nil {
		t.Fatalf("unexpected error with valid TLS pair: %v", err)
	}
}

// TestLoadRunConfigAcceptRoots：未知根必须报错，避免静默「拒绝一切设备」。
func TestLoadRunConfigAcceptRoots(t *testing.T) {
	resetConfigEnv(t)

	t.Setenv("ORACLE_ACCEPT_ROOTS", "not_a_real_root")
	if _, err := LoadRunConfig(":8080"); err == nil {
		t.Fatal("want error for unknown attestation root")
	}

	t.Setenv("ORACLE_ACCEPT_ROOTS", "  ,  ")
	if _, err := LoadRunConfig(":8080"); err == nil {
		t.Fatal("want error for empty accept roots")
	}

	// 合法根名：从 allRoots 里取第一个（与产品定义强绑定）
	if len(allRoots) == 0 {
		t.Fatal("allRoots should not be empty")
	}
	t.Setenv("ORACLE_ACCEPT_ROOTS", allRoots[0])
	if _, err := LoadRunConfig(":8080"); err != nil {
		t.Fatalf("unexpected error with known root: %v", err)
	}
}

// TestLoadRunConfigStrict：严格模式缺必填项必须拒绝启动。
func TestLoadRunConfigStrict(t *testing.T) {
	resetConfigEnv(t)
	t.Setenv("ORACLE_STRICT", "1")

	// 全部缺失 → 报错列出必填项
	if _, err := LoadRunConfig(":8080"); err == nil {
		t.Fatal("want error in strict mode with missing requirements")
	}

	// 补齐全部 → 通过
	t.Setenv("ORACLE_KEY", "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	t.Setenv("ORACLE_SIGN_TOKEN", "tok")
	t.Setenv("ORACLE_GOOGLE_PUBKEY", "MEkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE_placeholder")
	t.Setenv("ORACLE_ALLOW_PLAINTEXT", "1")
	cfg, err := LoadRunConfig(":8080")
	if err != nil {
		t.Fatalf("strict mode should pass with all requirements: %v", err)
	}
	if !cfg.Strict {
		t.Fatal("strict flag not propagated")
	}

	// ORACLE_ENV=production 同样触发严格模式
	resetConfigEnv(t)
	t.Setenv("ORACLE_ENV", "production")
	t.Setenv("ORACLE_KEY", "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	t.Setenv("ORACLE_SIGN_TOKEN", "tok")
	t.Setenv("ORACLE_GOOGLE_PUBKEY", "MEkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE_placeholder")
	t.Setenv("ORACLE_ALLOW_PLAINTEXT", "1")
	cfg2, err := LoadRunConfig(":8080")
	if err != nil {
		t.Fatalf("production env should imply strict: %v", err)
	}
	if !cfg2.Strict {
		t.Fatal("ORACLE_ENV=production should set Strict")
	}
}

// TestConfigHelpers：envBool / envInt / envDuration 的边界行为。
func TestConfigHelpers(t *testing.T) {
	resetConfigEnv(t)

	t.Setenv("B", "TRUE")
	if !envBool("B") {
		t.Fatal("envBool(TRUE) should be true")
	}
	t.Setenv("B", "0")
	if envBool("B") {
		t.Fatal("envBool(0) should be false")
	}

	t.Setenv("I", "42")
	if n, err := envInt("I", 0); err != nil || n != 42 {
		t.Fatalf("envInt = %d, %v", n, err)
	}
	t.Setenv("I", "x")
	if _, err := envInt("I", 0); err == nil {
		t.Fatal("envInt should fail on non-integer")
	}
	if n, err := envInt("UNSET", 7); err != nil || n != 7 {
		t.Fatalf("envInt default = %d, %v", n, err)
	}

	t.Setenv("D", "90s")
	if d, err := envDuration("D", 0); err != nil || d != 90*time.Second {
		t.Fatalf("envDuration = %v, %v", d, err)
	}
	t.Setenv("D", "-1s")
	if _, err := envDuration("D", 0); err == nil {
		t.Fatal("envDuration should reject negative duration")
	}
	t.Setenv("D", "nope")
	if _, err := envDuration("D", 0); err == nil {
		t.Fatal("envDuration should reject garbage")
	}
}
