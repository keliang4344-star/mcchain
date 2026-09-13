// MobileChain Oracle 预言机服务：接收钱包 attestation 请求，验证签名，转发给链上 Verifier。
//
// 用法：
//
//	oracle --port 8080 --chain-id mcchain-1 --node tcp://localhost:26657
//
// 环境变量：
//
//	_SIGNER_MNEMONIC  - 预言机签名账户助记词（必填，用于向链上提交 MsgSubmitAttestation）
//	_KEYRING_DIR      - keyring 目录（默认 $HOME/.mcchain-oracle）
//	_CHAIN_ID         - 链 ID（优先级低于 --chain-id）
//	_NODE             - 链 RPC 端点（优先级低于 --node）
//	_LISTEN_PORT      - HTTP 监听端口（优先级低于 --port）
//	_BIND_ADDR        - 监听地址 host:port（未设置时仅绑回环 127.0.0.1:<port>）
//	_API_KEY          - /attest 的 Bearer 密钥；监听非回环地址时必填（fail-closed）
//	_TRUSTED_PROXY    - 受信反向代理 IP/CIDR 列表（逗号分隔）；仅这些来源的
//	                          X-Forwarded-For 才被采信，否则限流一律按 RemoteAddr
//	_STRICT           - 置 1（或 ORACLE_ENV=production）启用严格模式
//
// go.mod 依赖（需在 mcchain 根 go.mod 中）：
//
//	require (
//	    github.com/cosmos/cosmos-sdk v0.47.3
//	    github.com/spf13/cobra v1.6.1
//	)
//
// oracle 作为 cmd 运行在 mcchain 模块内，无需独立 go.mod。
package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/spf13/cobra"

	"mcchain/app"
	"mcchain/internal/oraclesvc"
	depintypes "mcchain/x/depin/types"
	phonetypes "mcchain/x/phonenode/types"
)

// 自检与重试参数：链上提交失败最多重试 4 次（500ms→1s→2s 退避），
// 每 60s 自检一次，超过 10 分钟没有成功提交即视为服务降级。
const (
	submitRetryAttempts = 4
	submitRetryBaseWait = 500 * time.Millisecond
	healthInterval      = 60 * time.Second
	healthStaleAfter    = 10 * time.Minute
)

// OracleService 预言机 HTTP 服务。
type OracleService struct {
	clientCtx  client.Context
	txFactory  tx.Factory
	oracleAddr sdk.AccAddress
	chainID    string
	listenAddr string
	httpServer *http.Server

	// monitor 记录链上提交的成败与最近一次成功时间，供周期自检判定降级。
	monitor *oraclesvc.HealthMonitor

	// apiKey 若非空，/attest 必须携带 Authorization: Bearer <apiKey>（生产加固）。
	// 监听非回环地址时该值保证非空（resolveConfig 已 fail-closed 校验，见 S1）。
	apiKey string
	// rateLimiter 按客户端 IP 限流，防止开放中继被刷（生产加固）。
	rateLimiter *rateLimiter
	// trustedProxies 为受信反向代理网段；只有来自这些来源的请求，其 X-Forwarded-For
	// 才会被当作真实客户端 IP，否则一律按 RemoteAddr 限流（S3 防伪造头绕过限流）。
	trustedProxies []*net.IPNet
	// verifier 是设备 attestation 证据的验证器（生产加固）。
	// 生产环境应注入真实 TEE 出证验签实现（Android Key Attestation /
	// iOS DeviceCheck），开发/测试环境使用 SoftOracleVerifier（SHA256 比对）。
	// 把验证逻辑抽象为接口，使生产可在不改动路由的前提下替换为真实 TEE 验签。
	verifier AttestationVerifier
}

func main() {
	rootCmd := newOracleCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "oracle: %v\n", err)
		os.Exit(1)
	}
}

func newOracleCmd() *cobra.Command {
	var (
		port    int
		chainID string
		node    string
	)

	cmd := &cobra.Command{
		Use:   "oracle",
		Short: "MC Chain Oracle - Device Attestation Verification Service",
		Long: `MobileChain 预言机服务：接收钱包设备 attestation 请求，验证设备身份，
通过 Cosmos SDK 客户端将验证结果写入链上 depin 模块。

流程：POST /attest → 解析 attestation_proof → 查询 phonenode 注册状态 →
      校验 SHA256 设备指纹 → 调用 MsgSubmitAttestation 上链 → 返回 pass/fail 结果。`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 生产加固①：解析 + 校验配置，任何必填项缺失或取值非法都立即报错退出。
			cfg, err := resolveConfig(chainID, node, port)
			if err != nil {
				oraclesvc.Errorf("invalid configuration: %v", err)
				return err
			}
			return runOracle(cfg)
		},
	}

	cmd.Flags().IntVar(&port, "port", 8080, "HTTP 监听端口")
	cmd.Flags().StringVar(&chainID, "chain-id", "", "链 ID（例如 mcchain-1）")
	cmd.Flags().StringVar(&node, "node", "", "链 RPC 端点（例如 tcp://localhost:26657）")

	return cmd
}

// oracleConfig 是经过校验的启动配置。
type oracleConfig struct {
	chainID    string
	nodeURI    string
	listenAddr string
	mnemonic   string
	keyringDir string
	apiKey     string
	// trustedProxies 是受信反向代理来源（IP/CIDR）。仅当请求的 RemoteAddr 命中其中之一时，
	// 才采信 X-Forwarded-For 里的客户端 IP 用于限流，否则一律以 RemoteAddr 计。
	trustedProxies []*net.IPNet
	strict         bool
}

// resolveConfig 按「命令行 > 环境变量 > 默认值」解析配置，并逐项校验。
//
// 严格模式（ORACLE_STRICT=1 或 ORACLE_ENV=production）下，chain-id 与 node 必须显式
// 提供 —— 生产环境静默回落到 mcchain-1 / localhost:26657 会让服务连到错误的链或
// 根本连不上，却看不出任何异常。
func resolveConfig(chainID, node string, port int) (oracleConfig, error) {
	strict := envBool("ORACLE_STRICT") || strings.EqualFold(os.Getenv("ORACLE_ENV"), "production")

	cfg := oracleConfig{strict: strict}

	// 1) 链 ID
	cfg.chainID = strings.TrimSpace(firstNonEmpty(chainID, os.Getenv("ORACLE_CHAIN_ID")))
	if cfg.chainID == "" {
		if strict {
			return oracleConfig{}, errors.New("chain id is required in strict mode: set --chain-id or ORACLE_CHAIN_ID")
		}
		cfg.chainID = "mcchain-1"
	}

	// 2) 链 RPC 端点
	cfg.nodeURI = strings.TrimSpace(firstNonEmpty(node, os.Getenv("ORACLE_NODE")))
	if cfg.nodeURI == "" {
		if strict {
			return oracleConfig{}, errors.New("chain node endpoint is required in strict mode: set --node or ORACLE_NODE (e.g. tcp://rpc.example:26657)")
		}
		cfg.nodeURI = "tcp://localhost:26657"
	}
	if err := validateNodeURI(cfg.nodeURI); err != nil {
		return oracleConfig{}, err
	}

	// 3) 监听端口：命令行未显式指定时才看 ORACLE_LISTEN_PORT；
	//    环境变量非法时必须报错，不能拼出 ":abc" 这种地址再去 Listen。
	listenPort := port
	if envPort := strings.TrimSpace(os.Getenv("ORACLE_LISTEN_PORT")); envPort != "" && port == 8080 {
		p, err := strconv.Atoi(envPort)
		if err != nil {
			return oracleConfig{}, fmt.Errorf("ORACLE_LISTEN_PORT must be an integer, got %q", envPort)
		}
		listenPort = p
	}
	if listenPort < 1 || listenPort > 65535 {
		return oracleConfig{}, fmt.Errorf("listen port must be in 1..65535, got %d", listenPort)
	}
	// 5.5) 绑定地址：生产环境应仅监听内网（前置 Nginx TLS + Bearer 鉴权 + 限流），
	// 通过 ORACLE_BIND_ADDR 显式指定（如 127.0.0.1:8080 或内网 IP）。未显式设置时
	// 「fail-safe 回落到回环 127.0.0.1:<port>」而非全接口 ":port"——避免默认部署把未鉴权的
	// /attest 直接暴露到公网。需对外服务必须显式设置 ORACLE_BIND_ADDR。
	if bind := strings.TrimSpace(os.Getenv("ORACLE_BIND_ADDR")); bind != "" {
		if _, _, err := net.SplitHostPort(bind); err != nil {
			return oracleConfig{}, fmt.Errorf("ORACLE_BIND_ADDR %q is not a valid host:port: %w", bind, err)
		}
		cfg.listenAddr = bind
	} else {
		cfg.listenAddr = fmt.Sprintf("127.0.0.1:%d", listenPort)
	}

	// 4) 签名账户助记词：没有它就无法向链上提交 attestation 结果。
	cfg.mnemonic = strings.TrimSpace(os.Getenv("ORACLE_SIGNER_MNEMONIC"))
	if cfg.mnemonic == "" {
		return oracleConfig{}, errors.New("ORACLE_SIGNER_MNEMONIC is required: oracle needs a funded account to submit attestation results on-chain")
	}
	if n := len(strings.Fields(cfg.mnemonic)); n != 12 && n != 24 {
		return oracleConfig{}, fmt.Errorf("ORACLE_SIGNER_MNEMONIC must be a 12 or 24 word BIP39 mnemonic, got %d words", n)
	}

	// 6) API key：/attest 需携带 Authorization: Bearer <key>，避免开放中继被任意调用
	//    （生产加固）。仅当服务只绑回环（本地开发 / CI）时可省略；一旦监听地址可被
	//    外部访问，缺失即视为配置错误并拒绝启动（S1 fail-closed，见下方校验）。
	cfg.apiKey = strings.TrimSpace(os.Getenv("ORACLE_API_KEY"))

	// 6.1) S1 fail-closed：监听非回环地址（0.0.0.0 / :: / 内网或公网 IP）却没有 Bearer 密钥，
	//      等于把「可直接写链的 attestation 中继」开放给任何能连上该端口的人。这里直接拒绝启动，
	//      而不是仅打一条 Warn 日志——否则运维漏设变量就会静默上线一个开放中继。
	if !isLoopbackListenAddr(cfg.listenAddr) && cfg.apiKey == "" {
		return oracleConfig{}, fmt.Errorf(
			"ORACLE_API_KEY is required when binding a non-loopback address (%s): "+
				"/attest submits on-chain attestations and must not be exposed unauthenticated; "+
				"set ORACLE_API_KEY, or bind loopback only (unset ORACLE_BIND_ADDR) for local development",
			cfg.listenAddr)
	}
	// 6.2) 严格模式（生产）下即使绑回环也要求显式设置密钥，防止「先回环调通、后改绑定忘配密钥」。
	if cfg.strict && cfg.apiKey == "" {
		return oracleConfig{}, errors.New("ORACLE_API_KEY is required in strict/production mode")
	}

	// 7) 受信反向代理：默认为空 —— 任何 X-Forwarded-For 都不被采信，限流一律按 RemoteAddr。
	//    只有部署在受信网关之后时才配置此变量，使限流能识别真实客户端 IP 而不被伪造头绕过。
	trusted, err := parseTrustedProxies(os.Getenv("ORACLE_TRUSTED_PROXY"))
	if err != nil {
		return oracleConfig{}, err
	}
	cfg.trustedProxies = trusted

	// 5) keyring 目录：必须可创建/可写，否则启动后第一次签名才炸。
	cfg.keyringDir = strings.TrimSpace(envOrDefault("ORACLE_KEYRING_DIR", os.ExpandEnv("$HOME/.mcchain-oracle")))
	if cfg.keyringDir == "" {
		return oracleConfig{}, errors.New("ORACLE_KEYRING_DIR is empty and $HOME is not set; set ORACLE_KEYRING_DIR explicitly")
	}
	if err := os.MkdirAll(cfg.keyringDir, 0o700); err != nil {
		return oracleConfig{}, fmt.Errorf("ORACLE_KEYRING_DIR %q is not usable: %w", cfg.keyringDir, err)
	}

	return cfg, nil
}

// isLoopbackListenAddr 判断监听地址是否仅绑定回环（因而不可被外部主机直接访问）。
//
// 判定规则（保守：任何无法确证为回环的写法都按「可被外部访问」处理）：
//   - "127.0.0.1:8080" / "[::1]:8080" / "localhost:8080" → 回环
//   - ":8080" / "0.0.0.0:8080" / "[::]:8080"             → 全接口，非回环
//   - "10.0.0.5:8080" 等具体 IP                          → 非回环
//   - 无法解析的主机名（如内网 DNS 名）                   → 非回环（不做 DNS 解析，宁可从严）
func isLoopbackListenAddr(addr string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(addr))
	if err != nil {
		return false
	}
	host = strings.TrimSpace(host)
	if host == "" { // ":8080" —— 监听全部接口
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// parseTrustedProxies 把 "10.0.0.1, 172.16.0.0/12" 这样的列表解析为网段集合。
// 单个 IP 视为 /32（IPv4）或 /128（IPv6）。空输入返回 nil —— 即不信任任何代理头。
func parseTrustedProxies(raw string) ([]*net.IPNet, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var out []*net.IPNet
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.Contains(item, "/") {
			_, network, err := net.ParseCIDR(item)
			if err != nil {
				return nil, fmt.Errorf("ORACLE_TRUSTED_PROXY entry %q is not a valid CIDR: %w", item, err)
			}
			out = append(out, network)
			continue
		}
		ip := net.ParseIP(item)
		if ip == nil {
			return nil, fmt.Errorf("ORACLE_TRUSTED_PROXY entry %q is not a valid IP or CIDR", item)
		}
		bits := 32
		if ip.To4() == nil {
			bits = 128
		}
		out = append(out, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
	}
	return out, nil
}

// validateNodeURI 校验链 RPC 端点格式，避免把明显错误的地址带进运行期。
func validateNodeURI(uri string) error {
	schemes := []string{"tcp://", "http://", "https://", "unix://"}
	for _, s := range schemes {
		if strings.HasPrefix(uri, s) {
			if strings.TrimPrefix(uri, s) == "" {
				return fmt.Errorf("invalid node endpoint %q: missing host after %q", uri, s)
			}
			return nil
		}
	}
	return fmt.Errorf("invalid node endpoint %q: must start with one of tcp:// http:// https:// unix://", uri)
}

// firstNonEmpty 返回第一个非空字符串。
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// envBool 判断环境变量是否为真（1/true/yes/on）。
func envBool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func runOracle(cfg oracleConfig) error {
	oraclesvc.SetLogComponent("oracle-attestor")

	// 初始化 SDK 配置
	sdkCfg := sdk.GetConfig()
	sdkCfg.SetBech32PrefixForAccount("mc", "mcpub")
	sdkCfg.SetBech32PrefixForValidator("mcvaloper", "mcvaloperpub")
	sdkCfg.SetBech32PrefixForConsensusNode("mccons", "mcconspub")
	sdkCfg.Seal()

	chainID, nodeURI, listenAddr := cfg.chainID, cfg.nodeURI, cfg.listenAddr

	kr, err := keyring.New("mcchain-oracle", keyring.BackendTest, cfg.keyringDir, os.Stdin, depintypes.ModuleCdc)
	if err != nil {
		return fmt.Errorf("create keyring: %w", err)
	}

	// 接入 RPC client 与 SDK 编码配置。
	// client.Context{} 只 With Keyring/FromName/BroadcastMode 是不够的 ——
	// 缺 WithClient / WithTxConfig / WithAccountRetriever，/attest 的链上提交
	// 路径整体不可用（签名通过后重试 4 次全失败，健康监控永远 DEGRADED）。
	encCfg := app.MakeEncodingConfig()
	rpcClient, err := rpchttp.New(nodeURI, "/websocket")
	if err != nil {
		return fmt.Errorf("connect to rpc node %s: %w", nodeURI, err)
	}

	// 通过助记词恢复或创建 oracle 账户
	oracleRecord, err := kr.NewAccount("oracle", cfg.mnemonic, "", sdk.GetConfig().GetFullBIP44Path(), hd.Secp256k1)
	if err != nil {
		// 账户可能已存在，尝试获取
		var getErr error
		oracleRecord, getErr = kr.Key("oracle")
		if getErr != nil {
			return fmt.Errorf("load oracle key: %w (original: %w)", getErr, err)
		}
	}

	oracleAddr, err := oracleRecord.GetAddress()
	if err != nil {
		return fmt.Errorf("get oracle address: %w", err)
	}

	// 构造 Cosmos SDK 客户端上下文（RPC client + TxConfig + AccountRetriever 全接线）
	clientCtx := client.Context{}.
		WithChainID(chainID).
		WithNodeURI(nodeURI).
		WithClient(rpcClient).
		WithInterfaceRegistry(encCfg.InterfaceRegistry).
		WithCodec(encCfg.Marshaler).
		WithLegacyAmino(encCfg.Amino).
		WithTxConfig(encCfg.TxConfig).
		WithAccountRetriever(authtypes.AccountRetriever{}).
		WithKeyring(kr).
		WithFromName("oracle").
		WithFromAddress(oracleAddr).
		WithBroadcastMode("sync").
		WithSkipConfirmation(true)

	txFactory := tx.Factory{}.
		WithChainID(chainID).
		WithKeybase(kr).
		WithTxConfig(clientCtx.TxConfig).
		WithAccountRetriever(clientCtx.AccountRetriever).
		WithGasAdjustment(1.5).
		WithGasPrices("0.001umc").
		WithSignMode(signing.SignMode_SIGN_MODE_DIRECT)

	// verifier 模式闸门（fail-closed）。
	// _VERIFIER=soft（默认）仅限开发/测试；strict 模式必须显式选择
	// verifier，而当前 TEE 验签尚未接线 —— strict + 未就绪 = 拒绝启动，
	// 而不是静默接受「仅凭 SHA256(deviceID) 即可伪造」的软验证上链。
	switch verifierMode := strings.ToLower(strings.TrimSpace(os.Getenv("ORACLE_VERIFIER"))); verifierMode {
	case "soft":
		if cfg.strict {
			return fmt.Errorf("ORACLE_VERIFIER=soft 不可用于 strict 模式（attestation 不构成抗女巫证据）；生产部署须接入 TEE 验签后再以 strict 启动")
		}
		oraclesvc.Warnf("使用 SoftOracleVerifier（SHA256 比对）——仅供开发/测试，不构成抗女巫证据")
	case "tee":
		return fmt.Errorf("ORACLE_VERIFIER=tee：真实 TEE 验签尚未接入 cmd/oracle；请实现 AttestationVerifier 接口后替换，或以非 strict 模式明确接受软验证风险")
	default:
		return fmt.Errorf("未知 ORACLE_VERIFIER=%q（可选：soft | tee）", verifierMode)
	}

	svc := &OracleService{
		clientCtx:  clientCtx,
		txFactory:  txFactory,
		oracleAddr: oracleAddr,
		chainID:    chainID,
		listenAddr: listenAddr,
		// 生产加固④：监控链上提交的最近成功时间。
		monitor:     oraclesvc.NewHealthMonitor("chain-submit", healthInterval, healthStaleAfter),
		apiKey:      cfg.apiKey,
		rateLimiter: newRateLimiter(20, 10*time.Second), // 单 IP 每 10s 最多 20 次 /attest
		// 受信代理为空时，X-Forwarded-For 一律不采信（限流按 RemoteAddr）。
		trustedProxies: cfg.trustedProxies,
		// 经上方 ORACLE_VERIFIER 闸门放行后注入（当前仅 soft 可达）。
		verifier: SoftOracleVerifier{},
	}

	// HTTP 路由
	mux := http.NewServeMux()
	mux.HandleFunc("/health", svc.handleHealth)
	mux.HandleFunc("/attest", svc.authAndRateLimit(svc.handleAttest))

	svc.httpServer = &http.Server{
		Addr:              listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// 生产加固⑤：根上下文，收到 SIGINT/SIGTERM 即取消，退避重试与周期自检随之退出。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 生产加固④：周期自检 goroutine（仅记日志，不引入 HTTP/指标依赖）。
	go svc.monitor.Run(ctx)

	// 优雅关闭
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		oraclesvc.Infof("shutdown signal received, draining in-flight requests (timeout 15s)...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if serr := svc.httpServer.Shutdown(shutdownCtx); serr != nil {
			oraclesvc.Errorf("HTTP server shutdown error: %v", serr)
		}
	}()

	oraclesvc.Infof("MC Oracle started (strict=%t)", cfg.strict)
	oraclesvc.Infof("chain-id : %s", chainID)
	oraclesvc.Infof("rpc node : %s", nodeURI)
	oraclesvc.Infof("oracle   : %s", oracleAddr.String())
	oraclesvc.Infof("listening: http://%s", listenAddr)
	oraclesvc.Infof("endpoints: POST /attest  GET /health")
	if svc.apiKey != "" {
		oraclesvc.Infof("attest auth: Bearer required (ORACLE_API_KEY set)")
	} else {
		// 走到这里说明监听地址是回环（非回环 + 无密钥已在 resolveConfig 被拒绝启动），
		// 属本地开发 / CI 场景，仍明确提示暴露面，避免误改绑定后带着开放中继上线。
		oraclesvc.Warnf("attest auth: ORACLE_API_KEY not set — allowed only because listener is loopback-only (%s)", listenAddr)
	}
	if len(svc.trustedProxies) > 0 {
		oraclesvc.Infof("rate limit: X-Forwarded-For honored for %d trusted proxy range(s)", len(svc.trustedProxies))
	} else {
		oraclesvc.Infof("rate limit: keyed by RemoteAddr (X-Forwarded-For ignored; set ORACLE_TRUSTED_PROXY behind a gateway)")
	}

	if err := svc.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		oraclesvc.Errorf("HTTP server terminated: %v", err)
		return fmt.Errorf("HTTP server: %w", err)
	}

	<-shutdownDone
	oraclesvc.Infof("oracle stopped cleanly")
	return nil
}

// AttestRequest 设备 attestation 请求体。
type AttestRequest struct {
	DeviceID         string `json:"device_id"`
	AttestationProof string `json:"attestation_proof"` // SHA256(device_id)
	Signature        string `json:"signature"`         // 设备签名
}

// AttestResponse attestation 验证结果响应体。
type AttestResponse struct {
	Passed bool   `json:"passed"`
	Reason string `json:"reason"`
	TxHash string `json:"tx_hash,omitempty"`
}

// handleHealth 健康检查端点。除基本信息外，一并暴露自检统计，
// 便于外部探针判断服务是否已降级（最近一次成功上链时间）。
func (s *OracleService) handleHealth(w http.ResponseWriter, _ *http.Request) {
	lastSuccess, ok, failed := s.monitor.Stats()

	body := map[string]interface{}{
		"status":     "ok",
		"oracle":     s.oracleAddr.String(),
		"chain":      s.chainID,
		"time":       time.Now().Unix(),
		"submit_ok":  ok,
		"submit_err": failed,
	}
	if lastSuccess.IsZero() {
		body["last_submit_success"] = nil
		body["degraded"] = ok == 0 && failed > 0
	} else {
		body["last_submit_success"] = lastSuccess.UTC().Format(time.RFC3339)
		body["degraded"] = time.Since(lastSuccess) > healthStaleAfter
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(body)
}

// handleAttest 处理设备 attestation 请求。
func (s *OracleService) handleAttest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// 请求体上限 1MiB。nginx 网关有限制，但 oracle
	// 可绕过网关直连，需自我保护，防止超大 body 消耗内存/日志。
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req AttestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		oraclesvc.Warnf("[attest] bad json body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(AttestResponse{Passed: false, Reason: "bad json body"})
		return
	}

	if req.DeviceID == "" || req.AttestationProof == "" || req.Signature == "" {
		oraclesvc.Warnf("[attest] missing required fields")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(AttestResponse{Passed: false, Reason: "device_id, attestation_proof, and signature are required"})
		return
	}

	oraclesvc.Infof("[attest] device=%s verifying...", req.DeviceID)

	// 1. 本地校验 attestation 证据（经可插拔 verifier，生产为真实 TEE 验签）。
	passed, reason := s.verifier.Verify(req.DeviceID, req.AttestationProof, req.Signature)

	// 2. 将结果提交到链上（depin.MsgSubmitAttestation），失败自动指数退避重试。
	//    使用请求上下文：客户端断开或进程退出时可及时中止重试。
	txHash, submitErr := s.submitAttestationResult(r.Context(), req.DeviceID, req.AttestationProof, req.Signature, passed, reason)
	if submitErr != nil {
		oraclesvc.Errorf("[attest] device=%s on-chain submission failed: %v", req.DeviceID, submitErr)
	}

	resp := AttestResponse{
		Passed: passed,
		Reason: reason,
		TxHash: txHash,
	}

	switch {
	case passed && submitErr == nil:
		oraclesvc.Infof("[attest] device=%s PASSED tx=%s", req.DeviceID, txHash)
	case passed:
		oraclesvc.Warnf("[attest] device=%s PASSED locally but result was NOT recorded on-chain", req.DeviceID)
	default:
		oraclesvc.Warnf("[attest] device=%s FAILED reason=%s", req.DeviceID, reason)
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// verifyAttestationProof 本地验证设备身份证明。
// 对 deviceID 做 SHA256，与 proof 比对。
func verifyAttestationProof(deviceID, proof string) (bool, string) {
	hash := sha256.Sum256([]byte(deviceID))
	expectedProof := hex.EncodeToString(hash[:])

	if expectedProof != proof {
		return false, fmt.Sprintf("attestation proof mismatch for device %s", deviceID)
	}

	return true, "attestation proof verified (SHA256 match)"
}

// ---------------------------------------------------------------------------
// 生产加固：可插拔的 attestation 验证器
// ---------------------------------------------------------------------------

// AttestationVerifier 验证设备 attestation 证据的接口。
//
// 生产环境必须注入真实 TEE 出证验签实现（Android Key Attestation /
// iOS DeviceCheck）：预言机服务应只监听内网（ORACLE_BIND_ADDR），由前置网关
// 做 Bearer 鉴权 + 限流，并对「证明」做真实的硬件出证验签——任何人仅凭
// SHA256(deviceID) 即可伪造的软验证不得用于生产。
//
// SoftOracleVerifier 仅用于开发/测试与 CI，不构成抗女巫证据。
type AttestationVerifier interface {
	// Verify 返回 (是否通过, 失败原因)。
	Verify(deviceID, proof, signature string) (bool, string)
}

// SoftOracleVerifier 开发/测试用软验证：仅校验 SHA256(deviceID) == proof。
type SoftOracleVerifier struct{}

// Verify 实现 AttestationVerifier（软验证，仅供非生产环境）。
func (SoftOracleVerifier) Verify(deviceID, proof, _ string) (bool, string) {
	return verifyAttestationProof(deviceID, proof)
}

// submitAttestationResult 通过 Cosmos SDK 客户端向链上提交验证结果。
//
// 生产加固③：整个「查账户 → 构建 → 签名 → 广播」流程包一层指数退避重试
// （最多 submitRetryAttempts 次，间隔 500ms 起翻倍），每次失败记 WARN 日志；
// 全部失败记 ERROR 并计入健康统计。重试过程尊重 ctx 取消。
func (s *OracleService) submitAttestationResult(ctx context.Context, deviceID, proof, signature string, passed bool, reason string) (string, error) {
	var txHash string

	err := oraclesvc.Retry(ctx, "submit-attestation", oraclesvc.RetryPolicy{
		Attempts: submitRetryAttempts,
		BaseWait: submitRetryBaseWait,
		MaxWait:  oraclesvc.DefaultRetryMaxWait,
	}, func(context.Context) error {
		h, serr := s.broadcastAttestation(deviceID, proof, signature)
		txHash = h
		return serr
	})

	if err != nil {
		s.monitor.MarkFailure()
		return txHash, err
	}
	s.monitor.MarkSuccess()
	return txHash, nil
}

// broadcastAttestation 执行单次链上提交（供退避重试调用）。
func (s *OracleService) broadcastAttestation(deviceID, proof, signature string) (string, error) {
	// 构造 MsgSubmitAttestation
	msg := depintypes.NewMsgSubmitAttestation(
		deviceID,
		proof,
		signature,
		s.oracleAddr.String(),
	)

	// 通过 TxFactory 构建并广播交易
	txf := s.txFactory.
		WithFromName("oracle")

	clientCtx := s.clientCtx

	// 生产加固：client.Context 的 AccountRetriever / TxConfig 若未接线（当前构造方式
	// 未注入 RPC client 与编码配置），直接调用会触发 nil 解引用 panic，把整条连接打断。
	// 这里显式转成可观测的错误：由退避重试记 WARN、最终记 ERROR 并计入健康统计。
	if txf.AccountRetriever() == nil {
		return "", fmt.Errorf("client context has no AccountRetriever wired; on-chain submission is unavailable")
	}
	if clientCtx.TxConfig == nil {
		return "", fmt.Errorf("client context has no TxConfig wired; on-chain submission is unavailable")
	}

	// 更新 account number 和 sequence
	if err := txf.AccountRetriever().EnsureExists(clientCtx, s.oracleAddr); err != nil {
		return "", fmt.Errorf("oracle account not found on chain: %w", err)
	}

	// 构建未签名的交易
	unsignedTx, err := txf.BuildUnsignedTx(msg)
	if err != nil {
		return "", fmt.Errorf("build unsigned tx: %w", err)
	}

	// 签名
	if err := tx.Sign(txf, "oracle", unsignedTx, true); err != nil {
		return "", fmt.Errorf("sign tx: %w", err)
	}

	// 编码并广播
	txBytes, err := clientCtx.TxConfig.TxEncoder()(unsignedTx.GetTx())
	if err != nil {
		return "", fmt.Errorf("encode tx: %w", err)
	}

	resp, err := clientCtx.BroadcastTx(txBytes)
	if err != nil {
		return "", fmt.Errorf("broadcast tx: %w", err)
	}

	if resp.Code != 0 {
		return resp.TxHash, fmt.Errorf("tx failed (code=%d): %s", resp.Code, resp.RawLog)
	}

	return resp.TxHash, nil
}

func envOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// ---------------------------------------------------------------------------
// 生产加固：/attest 端点 Bearer 鉴权 + 按 IP 限流
// ---------------------------------------------------------------------------

// rateLimiter 是极简的固定窗口按 IP 限流器（进程内，重启清零）。
// 足以挡住开放中继被刷的初级 DoS；跨实例部署应前置反向代理/网关限流。
type rateLimiter struct {
	mu        sync.Mutex
	visits    map[string]int
	window    time.Duration
	maxReqs   int
	lastReset time.Time
}

func newRateLimiter(maxReqs int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		visits:    make(map[string]int),
		window:    window,
		maxReqs:   maxReqs,
		lastReset: time.Now(),
	}
}

// allow 返回该 IP 当前窗口内是否仍可放行一次请求。
func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	if now.Sub(rl.lastReset) > rl.window {
		rl.visits = make(map[string]int)
		rl.lastReset = now
	}
	if rl.visits[ip] >= rl.maxReqs {
		return false
	}
	rl.visits[ip]++
	return true
}

// authAndRateLimit 在调用 handleAttest 前执行 Bearer 鉴权与按 IP 限流。
func (s *OracleService) authAndRateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1) Bearer 鉴权：仅当配置了 ORACLE_API_KEY 时强制。
		// 常量时间比较，消除逐字节计时侧信道。
		if s.apiKey != "" {
			got := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
			if subtle.ConstantTimeCompare([]byte(got), []byte(s.apiKey)) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		// 2) 按客户端 IP 限流，防开放中继被刷。
		if !s.rateLimiter.allow(s.clientIP(r)) {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

// clientIP 取出用于限流的客户端标识。
//
// 安全约束：X-Forwarded-For 是客户端可任意伪造的请求头，若无条件采信，攻击者只需
// 每次请求换一个伪造的首段 IP，就能让固定窗口限流器为每个「新 IP」重新计数，从而完全绕过限流。
// 因此这里改为：只有当直连对端（RemoteAddr）本身命中受信代理网段时，才采信它转发来的
// X-Forwarded-For；其余情况一律以 RemoteAddr 为准（伪造头会被直接忽略）。
func (s *OracleService) clientIP(r *http.Request) string {
	remote := remoteHost(r)
	if len(s.trustedProxies) == 0 || !s.isTrustedProxy(remote) {
		return remote
	}
	// 受信代理链：XFF 首段是代理记录的原始客户端 IP。
	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		return remote
	}
	first := xff
	if idx := strings.IndexByte(xff, ','); idx >= 0 {
		first = xff[:idx]
	}
	first = strings.TrimSpace(first)
	// 代理也可能转发垃圾值，无法解析成 IP 时退回直连对端，避免限流键被任意字符串污染。
	if net.ParseIP(first) == nil {
		return remote
	}
	return first
}

// isTrustedProxy 判断直连对端是否位于受信代理网段内。
func (s *OracleService) isTrustedProxy(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, network := range s.trustedProxies {
		if network != nil && network.Contains(ip) {
			return true
		}
	}
	return false
}

// remoteHost 从 RemoteAddr 中剥离端口，得到直连对端 IP。
func remoteHost(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// 确保 phonetypes 被引用（编译时链接 phonenode 模块类型）
var _ = phonetypes.ModuleName
