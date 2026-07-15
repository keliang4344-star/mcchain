package cli

import (
	"strconv"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/spf13/cobra"
	"mcchain/x/phonenode/types"
)

var _ = strconv.Itoa(0)

func CmdSubmitStateProof() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-state-proof [root] [leaf] [index] [proof]",
		Short: "Broadcast message submit-state-proof",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			argRoot := args[0]
			argLeaf := args[1]
			argIndex := args[2]
			argProof := args[3]

			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := types.NewMsgSubmitStateProof(
				clientCtx.GetFromAddress().String(),
				argRoot,
				argLeaf,
				argIndex,
				argProof,
			)
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}
