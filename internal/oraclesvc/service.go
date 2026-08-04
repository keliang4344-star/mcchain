// Package oraclesvc 实现 MobileChain 链下预言机签名服务（T2 生产 attestation 闭环的运营侧）。
//
// 链侧 TeeOracle 约定：对消息 `deviceAddr + "|" + challenge` 用预言机 secp256k1 私钥
// 做签名，base64 后随 AttestDevice 上链验签（见 x/depin/types/oracle.go）。
// 本服务持有预言机私钥，暴露 HTTP 接口供设备/中转层获取公钥与请求签名。
//
// 生产部署：用固定 ORACLE_KEY（32 字节种子 hex）启动，把 /pubkey 返回的 pubkey
// base64 作为 MC_ORACLE_PUBKEY 注入验证人节点（app.go 启动即切换 TeeOracle）。
package oraclesvc

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Run 启动预言机 HTTP 服务，直到出错或收到 SIGINT/SIGTERM 后优雅退出。
func Run(listen string) error {
	SetLogComponent("oracle-signer")

	// 生产加固①：启动即校验配置，任何必填项缺失/非法立刻报错退出，绝不静默继续。
	cfg, err := LoadRunConfig(listen)
	if err != nil {
		Errorf("invalid configuration: %v", err)
		return fmt.Errorf("oracle config: %w", err)
	}

	// 使用 MC 地址前缀，使 /pubkey 返回的地址与链一致（mc... / mcpub...）。
	//
	// 注意：作为 `mcchaind oracle` 子命令运行时，根命令的 initSDKConfig() 已设置并
	// Seal 过同样的前缀，此处再 Set 一次会 panic("Config is sealed")。因此仅在前缀
	// 尚未就位时才设置，保证独立进程与 mcchaind 子命令两种启动方式都能正常运行。
	sdkCfg := sdk.GetConfig()
	if sdkCfg.GetBech32AccountAddrPrefix() != "mc" {
		sdkCfg.SetBech32PrefixForAccount("mc", "mcpub")
		sdkCfg.Seal()
	}

	priv, err := loadOrGenerateKey(cfg.Strict)
	if err != nil {
		Errorf("load oracle key: %v", err)
		return fmt.Errorf("load oracle key: %w", err)
	}

	pub := priv.PubKey().(*secp256k1.PubKey)
	addr := sdk.AccAddress(pub.Address()).String()
	pubB64 := base64.StdEncoding.EncodeToString(pub.Bytes())

	registry := NewRegistryFromEnv()
	configuredRoots := registry.ConfiguredRoots()

	Infof("MC Oracle signer ready (strict=%t)", cfg.Strict)
	Infof("oracle address: %s", addr)
	Infof("oracle pubkey(base64, 33B compressed): %s", pubB64)
	Infof("sign msg format: deviceAddr + \"|\" + challenge (base64 secp256k1 sig)")
	Infof("attestation roots: allowed=[%s] configured=[%s]",
		strings.Join(cfg.AcceptRoots, ","), strings.Join(configuredRoots, ","))

	if cfg.SignToken != "" {
		Infof("/sign auth: Bearer token REQUIRED")
	} else {
		Warnf("ORACLE_SIGN_TOKEN 未设置，/sign 无认证！生产前务必配置（或设置 ORACLE_STRICT=1 强制校验）")
	}
	if cfg.RateLimit > 0 {
		Infof("/sign rate limit: %d req/min", cfg.RateLimit)
	} else {
		Warnf("ORACLE_RATE_LIMIT 未设置，/sign 不限流")
	}
	if len(configuredRoots) == 0 {
		Warnf("未配置任何 attestation 根公钥，/sign 将 fail-closed 拒绝所有请求")
	}

	// 生产加固⑤：根进程上下文，收到 SIGINT/SIGTERM 即取消，周期任务与退避重试随之退出。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 生产加固④：周期自检 goroutine，最近一次成功签名过久即记 WARN/ERROR。
	monitor := NewHealthMonitor("oracle-signer", cfg.HealthInterval, cfg.StaleThreshold)
	go monitor.Run(ctx)

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           NewSecureHandlerWithMonitor(priv, addr, pubB64, cfg.SignToken, cfg.RateLimit, registry, monitor),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// 优雅关闭：上下文取消后给在途请求 15s 收尾时间。
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		Infof("shutdown signal received, draining in-flight requests (timeout 15s)...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if serr := srv.Shutdown(shutdownCtx); serr != nil {
			Errorf("graceful shutdown failed: %v", serr)
		}
	}()

	useTLS := cfg.TLSCert != "" && cfg.TLSKey != ""
	if useTLS {
		Infof("listening on https://%s", cfg.Listen)
		err = srv.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey)
	} else {
		Infof("listening on http://%s", cfg.Listen)
		Warnf("未启用 TLS，生产请将本服务置于 TLS 反向代理之后")
		err = srv.ListenAndServe()
	}

	// ErrServerClosed 表示是主动优雅关闭，不算失败。
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		Errorf("http server terminated: %v", err)
		return err
	}

	<-shutdownDone
	Infof("oracle signer stopped cleanly")
	return nil
}

// NewHandler 构造预言机 HTTP 处理器（无认证 / 无限流，便于单测），verifier 为
// attestation 注册表（nil 时拒绝一切 /sign —— fail-closed）。
func NewHandler(priv *secp256k1.PrivKey, addr, pubB64 string, verifier AttestationVerifier) http.Handler {
	return NewSecureHandler(priv, addr, pubB64, "", 0, verifier)
}

// NewSecureHandler 构造带认证的预言机 HTTP 处理器。
// signToken 非空 → /sign 需 `Authorization: Bearer <token>`；ratePerMin>0 → 每分钟限流；
// verifier 为 attestation 注册表，/sign 在签名前必须先通过真机验证（P0②）。
func NewSecureHandler(priv *secp256k1.PrivKey, addr, pubB64, signToken string, ratePerMin int, verifier AttestationVerifier) http.Handler {
	return NewSecureHandlerWithMonitor(priv, addr, pubB64, signToken, ratePerMin, verifier, nil)
}

// NewSecureHandlerWithMonitor 与 NewSecureHandler 相同，但额外接入健康监控：
// 每次成功签名会刷新「最近成功时间」，失败则计入失败数，供周期自检判定服务是否降级。
// monitor 可为 nil（HealthMonitor 的方法对 nil 接收者安全）。
func NewSecureHandlerWithMonitor(priv *secp256k1.PrivKey, addr, pubB64, signToken string, ratePerMin int, verifier AttestationVerifier, monitor *HealthMonitor) http.Handler {
	rl := &rateLimiter{limit: ratePerMin}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/pubkey", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]string{
			"address":         addr,
			"pubkey_base64":   pubB64,
			"sign_msg_format": "deviceAddr|\"|\"|challenge",
		})
	})

	signFn := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		// S1：Bearer 认证
		if signToken != "" {
			got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if got != signToken {
				monitor.MarkFailure()
				Warnf("/sign unauthorized from %s", clientIP(r))
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		// S1：限流
		if !rl.allow() {
			monitor.MarkFailure()
			Warnf("/sign rate limited from %s", clientIP(r))
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		var req struct {
			DeviceAddr  string           `json:"device_addr"`
			Challenge   string           `json:"challenge"`
			Attestation AttestationClaim `json:"attestation"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			monitor.MarkFailure()
			Warnf("/sign bad json from %s: %v", clientIP(r), err)
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if req.DeviceAddr == "" || req.Challenge == "" {
			monitor.MarkFailure()
			Warnf("/sign missing required fields from %s", clientIP(r))
			http.Error(w, "device_addr and challenge required", http.StatusBadRequest)
			return
		}
		// P0②：签名前必须先验证真实设备 attestation（fail-closed：无注册表或验证失败即拒绝）。
		if verifier == nil {
			monitor.MarkFailure()
			Errorf("/sign rejected: attestation verifier not configured (fail-closed)")
			http.Error(w, "attestation verifier not configured", http.StatusServiceUnavailable)
			return
		}
		// 尊重请求上下文：客户端断开或服务关闭时不再做无谓的验证与签名。
		if err := r.Context().Err(); err != nil {
			monitor.MarkFailure()
			Warnf("/sign aborted for device=%s: %v", req.DeviceAddr, err)
			http.Error(w, "request canceled", http.StatusRequestTimeout)
			return
		}
		if err := verifier.Verify(r.Context(), req.Attestation); err != nil {
			monitor.MarkFailure()
			Warnf("/sign attestation rejected device=%s root=%s: %v", req.DeviceAddr, req.Attestation.Root, err)
			http.Error(w, "attestation verification failed: "+err.Error(), http.StatusForbidden)
			return
		}
		// 与链上 TeeOracle.VerifyDeviceAttestation 完全一致的待签消息
		msg := []byte(req.DeviceAddr + "|" + req.Challenge)
		sig, serr := priv.Sign(msg)
		if serr != nil {
			monitor.MarkFailure()
			Errorf("/sign signing failed device=%s: %v", req.DeviceAddr, serr)
			http.Error(w, "sign failed", http.StatusInternalServerError)
			return
		}
		monitor.MarkSuccess()
		Infof("/sign issued signature device=%s root=%s", req.DeviceAddr, req.Attestation.Root)
		writeJSON(w, map[string]string{
			"device_addr":      req.DeviceAddr,
			"challenge":        req.Challenge,
			"signature_base64": base64.StdEncoding.EncodeToString(sig),
		})
	}
	mux.HandleFunc("/sign", signFn)
	return mux
}

// rateLimiter 极简固定窗口限流器（按分钟重置）。limit<=0 表示不限流。
type rateLimiter struct {
	mu     sync.Mutex
	limit  int
	count  int
	window time.Time
}

func (r *rateLimiter) allow() bool {
	if r == nil || r.limit <= 0 {
		return true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	if now.Sub(r.window) > time.Minute {
		r.count = 0
		r.window = now
	}
	if r.count >= r.limit {
		return false
	}
	r.count++
	return true
}

// clientIP 提取请求来源 IP（仅用于日志，不作为安全判定依据）。
func clientIP(r *http.Request) string {
	if r == nil {
		return "unknown"
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	if r.RemoteAddr == "" {
		return "unknown"
	}
	return r.RemoteAddr
}

// loadOrGenerateKey 优先从 ORACLE_KEY（32 字节 hex）或 ORACLE_KEY_FILE 读取固定密钥；
// 都不存在时生成临时密钥（仅开发演示，重启即变）。strict=true（生产模式）下拒绝
// 使用临时密钥并直接报错退出 —— 临时密钥会让链上 MC_ORACLE_PUBKEY 立刻失配。
func loadOrGenerateKey(strict bool) (*secp256k1.PrivKey, error) {
	if hexKey := os.Getenv("ORACLE_KEY"); hexKey != "" {
		bz, err := hex.DecodeString(strings.TrimSpace(hexKey))
		if err != nil {
			return nil, fmt.Errorf("ORACLE_KEY must be 32-byte hex: %w", err)
		}
		if len(bz) != 32 {
			return nil, fmt.Errorf("ORACLE_KEY must be 32-byte hex, got %d bytes", len(bz))
		}
		Infof("oracle signing key loaded from ORACLE_KEY")
		return &secp256k1.PrivKey{Key: bz}, nil
	}
	if f := os.Getenv("ORACLE_KEY_FILE"); f != "" {
		bz, rerr := os.ReadFile(f)
		if rerr != nil {
			return nil, fmt.Errorf("ORACLE_KEY_FILE %q: %w", f, rerr)
		}
		seed, herr := hex.DecodeString(strings.TrimSpace(string(bz)))
		if herr != nil || len(seed) != 32 {
			return nil, fmt.Errorf("ORACLE_KEY_FILE %q must contain 32-byte hex", f)
		}
		Infof("oracle signing key loaded from ORACLE_KEY_FILE=%s", f)
		return &secp256k1.PrivKey{Key: seed}, nil
	}
	if strict {
		return nil, fmt.Errorf("ORACLE_KEY or ORACLE_KEY_FILE is required in strict mode; refusing to start with an ephemeral key that would break on-chain MC_ORACLE_PUBKEY verification")
	}
	priv := secp256k1.GenPrivKey()
	Warnf("未配置 ORACLE_KEY，已生成临时密钥（重启即变）。生产请把下面这串作为 ORACLE_KEY 固定下来：")
	Warnf("ORACLE_KEY=%s", hex.EncodeToString(priv.Key))
	return priv, nil
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
