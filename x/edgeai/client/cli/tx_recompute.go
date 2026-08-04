package cli

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/spf13/cobra"

	"mcchain/x/edgeai/types"
)

// CmdSubmitRecompute 第二验证层：对争议任务提交链上重算结果指纹。
func CmdSubmitRecompute() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-recompute [task_id] [recompute_hash]",
		Short: "Submit an on-chain recomputation fingerprint for a disputed task (second verification layer)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			msg := types.NewMsgSubmitRecompute(clientCtx.GetFromAddress().String(), args[0], args[1])
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
