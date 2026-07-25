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
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
)

// Run 启动预言机 HTTP 服务，直到出错返回。
func Run(listen string) error {
	// 使用 MC 地址前缀，使 /pubkey 返回的地址与链一致（mc... / mcpub...）
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount("mc", "mcpub")
	cfg.Seal()

	priv, err := loadOrGenerateKey()
	if err != nil {
		return fmt.Errorf("load oracle key: %w", err)
	}

	pub := priv.PubKey().(*secp256k1.PubKey)
	addr := sdk.AccAddress(pub.Address()).String()
	pubB64 := base64.StdEncoding.EncodeToString(pub.Bytes())

	// S1 生产加固：
	//   - ORACLE_SIGN_TOKEN：非空时 /sign 需 Bearer 认证（防止任意客户端滥用签名）。
	//   - ORACLE_RATE_LIMIT：/sign 每分钟最大请求数（0 = 不限）。
	//   - ORACLE_TLS_CERT / ORACLE_TLS_KEY：配置后启用 HTTPS（生产强烈建议）。
	signToken := os.Getenv("ORACLE_SIGN_TOKEN")
	rateLimit := atoiDefault(os.Getenv("ORACLE_RATE_LIMIT"), 0)

	log.Printf("MC Oracle signer ready")
	log.Printf("  oracle address : %s", addr)
	log.Printf("  oracle pubkey(base64, 33B compressed): %s", pubB64)
	log.Printf("  sign msg format: deviceAddr + \"|\" + challenge  (base64 secp256k1 sig)")
	if signToken != "" {
		log.Printf("  /sign auth: Bearer token REQUIRED")
	} else {
		log.Printf("  [WARN] ORACLE_SIGN_TOKEN 未设置，/sign 无认证！生产前务必配置。")
	}
	if rateLimit > 0 {
		log.Printf("  /sign rate limit: %d req/min", rateLimit)
	}

	h := NewSecureHandler(priv, addr, pubB64, signToken, rateLimit)

	cert, key := os.Getenv("ORACLE_TLS_CERT"), os.Getenv("ORACLE_TLS_KEY")
	if cert != "" && key != "" {
		log.Printf("listening on https://%s", listen)
		return http.ListenAndServeTLS(listen, cert, key, h)
	}
	log.Printf("listening on http://%s", listen)
	log.Printf("[WARN] 未启用 TLS，生产请将本服务置于 TLS 反向代理之后。")
	return http.ListenAndServe(listen, h)
}

// NewHandler 构造预言机 HTTP 处理器（无认证 / 无限流，便于单测）。
func NewHandler(priv *secp256k1.PrivKey, addr, pubB64 string) http.Handler {
	return NewSecureHandler(priv, addr, pubB64, "", 0)
}

// NewSecureHandler 构造带认证的预言机 HTTP 处理器。
// signToken 非空 → /sign 需 `Authorization: Bearer <token>`；ratePerMin>0 → 每分钟限流。
func NewSecureHandler(priv *secp256k1.PrivKey, addr, pubB64, signToken string, ratePerMin int) http.Handler {
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
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		// S1：限流
		if !rl.allow() {
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		var req struct {
			DeviceAddr string `json:"device_addr"`
			Challenge  string `json:"challenge"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if req.DeviceAddr == "" || req.Challenge == "" {
			http.Error(w, "device_addr and challenge required", http.StatusBadRequest)
			return
		}
		// 与链上 TeeOracle.VerifyDeviceAttestation 完全一致的待签消息
		msg := []byte(req.DeviceAddr + "|" + req.Challenge)
		sig, serr := priv.Sign(msg)
		if serr != nil {
			http.Error(w, "sign failed", http.StatusInternalServerError)
			return
		}
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

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return def
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// loadOrGenerateKey 优先从 ORACLE_KEY（32 字节 hex）或 ORACLE_KEY_FILE 读取固定密钥；
// 都不存在时生成临时密钥（仅开发演示，重启即变，生产务必固定）。
func loadOrGenerateKey() (*secp256k1.PrivKey, error) {
	if hexKey := os.Getenv("ORACLE_KEY"); hexKey != "" {
		bz, err := hex.DecodeString(strings.TrimSpace(hexKey))
		if err != nil || len(bz) != 32 {
			return nil, fmt.Errorf("ORACLE_KEY must be 32-byte hex, got %d bytes", len(bz))
		}
		return &secp256k1.PrivKey{Key: bz}, nil
	}
	if f := os.Getenv("ORACLE_KEY_FILE"); f != "" {
		bz, rerr := os.ReadFile(f)
		if rerr != nil {
			return nil, rerr
		}
		seed, herr := hex.DecodeString(strings.TrimSpace(string(bz)))
		if herr != nil || len(seed) != 32 {
			return nil, fmt.Errorf("ORACLE_KEY_FILE must contain 32-byte hex")
		}
		return &secp256k1.PrivKey{Key: seed}, nil
	}
	priv := secp256k1.GenPrivKey()
	log.Printf("[WARN] 未配置 ORACLE_KEY，已生成临时密钥（重启即变）。生产请把下面这串作为 ORACLE_KEY 固定下来：")
	log.Printf("[WARN] ORACLE_KEY=%s", hex.EncodeToString(priv.Key))
	return priv, nil
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
