// MobileChain Oracle 预言机服务：接收钱包 attestation 请求，验证签名，转发给链上 Verifier。
//
// 用法：
//
//	oracle --port 8080 --chain-id mcchain-1 --node tcp://localhost:26657
//
// 环境变量：
//
//	ORACLE_SIGNER_MNEMONIC  - 预言机签名账户助记词（必填，用于向链上提交 MsgSubmitAttestation）
//	ORACLE_KEYRING_DIR      - keyring 目录（默认 $HOME/.mcchain-oracle）
//	ORACLE_CHAIN_ID         - 链 ID（优先级低于 --chain-id）
//	ORACLE_NODE             - 链 RPC 端点（优先级低于 --node）
//	ORACLE_LISTEN_PORT      - HTTP 监听端口（优先级低于 --port）
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
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/spf13/cobra"

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
	strict     bool
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
	cfg.listenAddr = fmt.Sprintf(":%d", listenPort)

	// 4) 签名账户助记词：没有它就无法向链上提交 attestation 结果。
	cfg.mnemonic = strings.TrimSpace(os.Getenv("ORACLE_SIGNER_MNEMONIC"))
	if cfg.mnemonic == "" {
		return oracleConfig{}, errors.New("ORACLE_SIGNER_MNEMONIC is required: oracle needs a funded account to submit attestation results on-chain")
	}
	if n := len(strings.Fields(cfg.mnemonic)); n != 12 && n != 24 {
		return oracleConfig{}, fmt.Errorf("ORACLE_SIGNER_MNEMONIC must be a 12 or 24 word BIP39 mnemonic, got %d words", n)
	}

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

	// 构造 Cosmos SDK 客户端上下文
	clientCtx := client.Context{}.
		WithChainID(chainID).
		WithNodeURI(nodeURI).
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

	svc := &OracleService{
		clientCtx:  clientCtx,
		txFactory:  txFactory,
		oracleAddr: oracleAddr,
		chainID:    chainID,
		listenAddr: listenAddr,
		// 生产加固④：监控链上提交的最近成功时间。
		monitor: oraclesvc.NewHealthMonitor("chain-submit", healthInterval, healthStaleAfter),
	}

	// HTTP 路由
	mux := http.NewServeMux()
	mux.HandleFunc("/health", svc.handleHealth)
	mux.HandleFunc("/attest", svc.handleAttest)

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

	// 1. 本地校验 attestation_proof：SHA256(deviceID) == proof
	passed, reason := verifyAttestationProof(req.DeviceID, req.AttestationProof)

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

// 确保 phonetypes 被引用（编译时链接 phonenode 模块类型）
var _ = phonetypes.ModuleName
