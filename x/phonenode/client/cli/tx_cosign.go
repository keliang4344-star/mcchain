package cli

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/spf13/cobra"

	"mcchain/x/phonenode/types"
)

// CmdRegisterCloudSigner 绑定云端共签方公钥（Path C）。
func CmdRegisterCloudSigner() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "register-cloud-signer [cloud-pub-key-hex]",
		Short: "Bind a cloud co-signer public key (33-byte hex secp256k1) to this node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			msg := types.NewMsgRegisterCloudSigner(clientCtx.GetFromAddress().String(), args[0])
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdSubmitCosign 提交云端共签，链上验签。
func CmdSubmitCosign() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-cosign [payload-hash-hex] [cloud-signature-hex]",
		Short: "Submit the cloud co-signature for a payload hash and verify it on chain",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			msg := types.NewMsgSubmitCosign(clientCtx.GetFromAddress().String(), args[0], args[1])
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
