package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"mcchain/internal/oraclesvc"
)

// oracleCmd 启动链下预言机签名服务（T2 生产 attestation 闭环的运营侧组件）。
// 用法：mcchaind oracle   （可选环境变量 ORACLE_LISTEN / ORACLE_KEY / MC_ORACLE_PUBKEY）
func oracleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "oracle",
		Short: "Run the off-chain attestation oracle signer service",
		Long: "启动预言机签名服务，对 (deviceAddr|challenge) 用预言机私钥签名，供设备 attestation 上链验签。\n" +
			"生产请把 /pubkey 返回的 pubkey_base64 作为 MC_ORACLE_PUBKEY 注入验证人节点以启用 TeeOracle。",
		RunE: func(cmd *cobra.Command, _ []string) error {
			listen := os.Getenv("ORACLE_LISTEN")
			if listen == "" {
				listen = ":8080"
			}
			return oraclesvc.Run(listen)
		},
	}
}
