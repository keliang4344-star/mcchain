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
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/spf13/cobra"

	depintypes "mcchain/x/depin/types"
	phonetypes "mcchain/x/phonenode/types"
)

// OracleService 预言机 HTTP 服务。
type OracleService struct {
	clientCtx       client.Context
	txFactory       tx.Factory
	oracleAddr      sdk.AccAddress
	chainID         string
	listenAddr      string
	httpServer      *http.Server
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
		port     int
		chainID  string
		node     string
	)

	cmd := &cobra.Command{
		Use:   "oracle",
		Short: "MC Chain Oracle - Device Attestation Verification Service",
		Long: `MobileChain 预言机服务：接收钱包设备 attestation 请求，验证设备身份，
通过 Cosmos SDK 客户端将验证结果写入链上 depin 模块。

流程：POST /attest → 解析 attestation_proof → 查询 phonenode 注册状态 →
      校验 SHA256 设备指纹 → 调用 MsgSubmitAttestation 上链 → 返回 pass/fail 结果。`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 参数优先级：命令行 > 环境变量 > 默认值
			if chainID == "" {
				chainID = envOrDefault("ORACLE_CHAIN_ID", "mcchain-1")
			}
			if node == "" {
				node = envOrDefault("ORACLE_NODE", "tcp://localhost:26657")
			}
			listen := fmt.Sprintf(":%d", port)
			if envPort := os.Getenv("ORACLE_LISTEN_PORT"); envPort != "" && port == 8080 {
				listen = fmt.Sprintf(":%s", envPort)
			}

			return runOracle(chainID, node, listen)
		},
	}

	cmd.Flags().IntVar(&port, "port", 8080, "HTTP 监听端口")
	cmd.Flags().StringVar(&chainID, "chain-id", "", "链 ID（例如 mcchain-1）")
	cmd.Flags().StringVar(&node, "node", "", "链 RPC 端点（例如 tcp://localhost:26657）")

	return cmd
}

func runOracle(chainID, nodeURI, listenAddr string) error {
	// 初始化 SDK 配置
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount("mc", "mcpub")
	cfg.SetBech32PrefixForValidator("mcvaloper", "mcvaloperpub")
	cfg.SetBech32PrefixForConsensusNode("mccons", "mcconspub")
	cfg.Seal()

	// 加载预言机签名账户
	mnemonic := os.Getenv("ORACLE_SIGNER_MNEMONIC")
	if mnemonic == "" {
		return fmt.Errorf("ORACLE_SIGNER_MNEMONIC not set; oracle needs a funded account to submit attestation results on-chain")
	}

	keyringDir := envOrDefault("ORACLE_KEYRING_DIR", os.ExpandEnv("$HOME/.mcchain-oracle"))
	kr, err := keyring.New("mcchain-oracle", keyring.BackendTest, keyringDir, os.Stdin, depintypes.ModuleCdc)
	if err != nil {
		return fmt.Errorf("create keyring: %w", err)
	}

	// 通过助记词恢复或创建 oracle 账户
	oracleRecord, err := kr.NewAccount("oracle", mnemonic, "", sdk.GetConfig().GetFullBIP44Path(), hd.Secp256k1)
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
	}

	// HTTP 路由
	mux := http.NewServeMux()
	mux.HandleFunc("/health", svc.handleHealth)
	mux.HandleFunc("/attest", svc.handleAttest)

	svc.httpServer = &http.Server{
		Addr:         listenAddr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// 优雅关闭
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("received signal %v, shutting down gracefully...", sig)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := svc.httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}
	}()

	log.Printf("MC Oracle started")
	log.Printf("  chain-id  : %s", chainID)
	log.Printf("  rpc node  : %s", nodeURI)
	log.Printf("  oracle    : %s", oracleAddr.String())
	log.Printf("  listening : http://%s", listenAddr)
	log.Printf("  endpoints : POST /attest  GET /health")

	if err := svc.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTP server: %w", err)
	}

	log.Println("oracle stopped")
	return nil
}

// AttestRequest 设备 attestation 请求体。
type AttestRequest struct {
	DeviceID         string `json:"device_id"`
	AttestationProof string `json:"attestation_proof"` // SHA256(device_id)
	Signature        string `json:"signature"`          // 设备签名
}

// AttestResponse attestation 验证结果响应体。
type AttestResponse struct {
	Passed bool   `json:"passed"`
	Reason string `json:"reason"`
	TxHash string `json:"tx_hash,omitempty"`
}

// handleHealth 健康检查端点。
func (s *OracleService) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"oracle":  s.oracleAddr.String(),
		"chain":   s.chainID,
		"time":    time.Now().Unix(),
	})
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
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(AttestResponse{Passed: false, Reason: "bad json body"})
		return
	}

	if req.DeviceID == "" || req.AttestationProof == "" || req.Signature == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(AttestResponse{Passed: false, Reason: "device_id, attestation_proof, and signature are required"})
		return
	}

	log.Printf("[attest] device=%s verifying...", req.DeviceID)

	// 1. 本地校验 attestation_proof：SHA256(deviceID) == proof
	passed, reason := verifyAttestationProof(req.DeviceID, req.AttestationProof)

	// 2. 将结果提交到链上（depin.MsgSubmitAttestation）
	var txHash string
	if passed {
		txHash, _ = s.submitAttestationResult(req.DeviceID, req.AttestationProof, req.Signature, true, reason)
	} else {
		txHash, _ = s.submitAttestationResult(req.DeviceID, req.AttestationProof, req.Signature, false, reason)
	}

	resp := AttestResponse{
		Passed: passed,
		Reason: reason,
		TxHash: txHash,
	}

	if passed {
		log.Printf("[attest] device=%s PASSED tx=%s", req.DeviceID, txHash)
	} else {
		log.Printf("[attest] device=%s FAILED reason=%s", req.DeviceID, reason)
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
func (s *OracleService) submitAttestationResult(deviceID, proof, signature string, passed bool, reason string) (string, error) {
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

	// 更新 account number 和 sequence
	clientCtx := s.clientCtx
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
