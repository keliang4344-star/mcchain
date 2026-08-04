package oraclesvc

// 生产加固：启动期配置校验。
//
// 原则：宁可启动即失败（fail-fast），也不要带着错误/半截配置静默跑起来。
// 典型的「静默继续」隐患（本文件逐一堵死）：
//   - listen 为空 → net/http 会默默监听 :80
//   - 只配了 ORACLE_TLS_CERT 没配 ORACLE_TLS_KEY → 静默降级成明文 HTTP
//   - ORACLE_RATE_LIMIT="abc" → 旧的 atoiDefault 静默返回 0（= 不限流）
//   - ORACLE_ACCEPT_ROOTS 里写错根名 → 静默变成「拒绝一切设备」
//
// 严格模式（ORACLE_STRICT=1 或 ORACLE_ENV=production）额外强制生产必填项。

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// RunConfig 是经过校验的预言机签名服务启动配置。
type RunConfig struct {
	Listen         string        // 监听地址 host:port
	SignToken      string        // /sign 的 Bearer token（空 = 不鉴权）
	RateLimit      int           // /sign 每分钟请求上限（0 = 不限）
	TLSCert        string        // TLS 证书路径（与 TLSKey 必须同时提供）
	TLSKey         string        // TLS 私钥路径
	AcceptRoots    []string      // 允许的 attestation 根
	HealthInterval time.Duration // 自检周期
	StaleThreshold time.Duration // 多久无成功即视为降级
	Strict         bool          // 严格（生产）模式
	AllowPlaintext bool          // 严格模式下显式允许明文 HTTP（置于 TLS 反代之后时使用）
}

// LoadRunConfig 从环境变量加载并校验启动配置。任何必填项缺失或取值非法都返回
// 明确的错误，由 Run 直接终止启动。
func LoadRunConfig(listen string) (RunConfig, error) {
	cfg := RunConfig{
		Strict:         envBool("ORACLE_STRICT") || strings.EqualFold(os.Getenv("ORACLE_ENV"), "production"),
		AllowPlaintext: envBool("ORACLE_ALLOW_PLAINTEXT"),
		SignToken:      os.Getenv("ORACLE_SIGN_TOKEN"),
		TLSCert:        strings.TrimSpace(os.Getenv("ORACLE_TLS_CERT")),
		TLSKey:         strings.TrimSpace(os.Getenv("ORACLE_TLS_KEY")),
	}

	// 1) 监听地址：必填且必须是合法 host:port。
	addr, err := validateListenAddr(listen)
	if err != nil {
		return RunConfig{}, err
	}
	cfg.Listen = addr

	// 2) 限流：给了就必须是合法非负整数，不能静默当成 0（不限流）。
	cfg.RateLimit, err = envInt("ORACLE_RATE_LIMIT", 0)
	if err != nil {
		return RunConfig{}, err
	}
	if cfg.RateLimit < 0 {
		return RunConfig{}, fmt.Errorf("ORACLE_RATE_LIMIT must be >= 0, got %d", cfg.RateLimit)
	}

	// 3) TLS 证书/私钥必须成对出现，否则会静默降级为明文 HTTP。
	switch {
	case cfg.TLSCert != "" && cfg.TLSKey == "":
		return RunConfig{}, fmt.Errorf("ORACLE_TLS_CERT is set but ORACLE_TLS_KEY is missing; refusing to fall back to plaintext HTTP")
	case cfg.TLSKey != "" && cfg.TLSCert == "":
		return RunConfig{}, fmt.Errorf("ORACLE_TLS_KEY is set but ORACLE_TLS_CERT is missing; refusing to fall back to plaintext HTTP")
	}
	for _, f := range []struct{ env, path string }{
		{"ORACLE_TLS_CERT", cfg.TLSCert},
		{"ORACLE_TLS_KEY", cfg.TLSKey},
	} {
		if f.path == "" {
			continue
		}
		if _, statErr := os.Stat(f.path); statErr != nil {
			return RunConfig{}, fmt.Errorf("%s: cannot read %q: %w", f.env, f.path, statErr)
		}
	}

	// 4) attestation 根白名单：根名写错会导致「拒绝一切设备」，必须显式报错。
	cfg.AcceptRoots, err = validateAcceptRoots(os.Getenv("ORACLE_ACCEPT_ROOTS"))
	if err != nil {
		return RunConfig{}, err
	}

	// 5) 自检周期与降级阈值。
	if cfg.HealthInterval, err = envDuration("ORACLE_HEALTH_INTERVAL", DefaultHealthInterval); err != nil {
		return RunConfig{}, err
	}
	if cfg.StaleThreshold, err = envDuration("ORACLE_HEALTH_STALE", DefaultStaleThreshold); err != nil {
		return RunConfig{}, err
	}

	// 6) 严格（生产）模式下的必填项。
	if cfg.Strict {
		if err := cfg.validateStrict(); err != nil {
			return RunConfig{}, err
		}
	}

	return cfg, nil
}

// validateStrict 检查生产模式下不可缺省的安全项，缺一即拒绝启动。
func (c RunConfig) validateStrict() error {
	var missing []string

	// 签名私钥必须固定，否则重启后 pubkey 变化，链上 TeeOracle 立刻验签失败。
	if os.Getenv("ORACLE_KEY") == "" && os.Getenv("ORACLE_KEY_FILE") == "" {
		missing = append(missing, "ORACLE_KEY or ORACLE_KEY_FILE (fixed oracle signing key)")
	}
	// /sign 无鉴权 = 任何人都能拿到预言机签名。
	if c.SignToken == "" {
		missing = append(missing, "ORACLE_SIGN_TOKEN (/sign bearer auth)")
	}
	// 明文 HTTP 会让 Bearer token 与签名结果裸奔，必须显式豁免。
	if c.TLSCert == "" && !c.AllowPlaintext {
		missing = append(missing, "ORACLE_TLS_CERT/ORACLE_TLS_KEY (or set ORACLE_ALLOW_PLAINTEXT=1 when behind a TLS proxy)")
	}
	// 一个 attestation 根都没配 → /sign 恒久 fail-closed，等于服务不可用。
	if !hasAnyAttestationRootConfigured() {
		missing = append(missing, "at least one attestation root (ORACLE_GOOGLE_PUBKEY[_FILE] / ORACLE_HW_PUBKEY[_FILE] / ORACLE_ANDROID_ROOTS_FILE)")
	}

	if len(missing) > 0 {
		return fmt.Errorf("strict mode (ORACLE_STRICT/ORACLE_ENV=production) requires the following configuration:\n  - %s",
			strings.Join(missing, "\n  - "))
	}
	return nil
}

// hasAnyAttestationRootConfigured 判断是否至少配置了一个 attestation 根来源。
func hasAnyAttestationRootConfigured() bool {
	for _, k := range []string{
		"ORACLE_GOOGLE_PUBKEY", "ORACLE_GOOGLE_PUBKEY_FILE",
		"ORACLE_HW_PUBKEY", "ORACLE_HW_PUBKEY_FILE",
		"ORACLE_ANDROID_ROOTS_FILE",
	} {
		if os.Getenv(k) != "" {
			return true
		}
	}
	return false
}

// validateListenAddr 校验监听地址为合法的 host:port（host 可省略）。
func validateListenAddr(listen string) (string, error) {
	listen = strings.TrimSpace(listen)
	if listen == "" {
		return "", fmt.Errorf("listen address is required (set ORACLE_LISTEN, e.g. \":8080\"); refusing to start on the net/http default port")
	}
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "", fmt.Errorf("invalid listen address %q (want host:port, e.g. \":8080\"): %w", listen, err)
	}
	p, err := strconv.Atoi(port)
	if err != nil || p < 1 || p > 65535 {
		return "", fmt.Errorf("invalid listen port %q in %q: must be an integer in 1..65535", port, listen)
	}
	if host != "" {
		if ip := net.ParseIP(host); ip == nil && host != "localhost" {
			// 允许主机名，但明显不是 IP/localhost 时给出可解析性检查。
			if _, lerr := net.LookupPort("tcp", port); lerr != nil {
				return "", fmt.Errorf("invalid listen address %q: %w", listen, lerr)
			}
		}
	}
	return listen, nil
}

// validateAcceptRoots 解析 ORACLE_ACCEPT_ROOTS，未知根名直接报错（避免静默全拒）。
func validateAcceptRoots(env string) ([]string, error) {
	if strings.TrimSpace(env) == "" {
		return allRoots, nil
	}
	known := make(map[string]bool, len(allRoots))
	for _, r := range allRoots {
		known[r] = true
	}
	var out []string
	for _, raw := range strings.Split(env, ",") {
		r := strings.TrimSpace(raw)
		if r == "" {
			continue
		}
		if !known[r] {
			return nil, fmt.Errorf("ORACLE_ACCEPT_ROOTS contains unknown root %q; supported roots: %s",
				r, strings.Join(allRoots, ", "))
		}
		out = append(out, r)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("ORACLE_ACCEPT_ROOTS is set but contains no valid root; supported roots: %s",
			strings.Join(allRoots, ", "))
	}
	return out, nil
}

// envBool 判断环境变量是否为真（1/true/yes/on，大小写不敏感）。
func envBool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// envInt 读取整数环境变量；未设置返回 def，取值非法返回错误（不静默回退）。
func envInt(key string, def int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer, got %q", key, raw)
	}
	return n, nil
}

// envDuration 读取时长环境变量（如 "30s" / "5m"）；非法值返回错误。
func envDuration(key string, def time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration (e.g. \"30s\", \"5m\"), got %q", key, raw)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s must be positive, got %q", key, raw)
	}
	return d, nil
}
