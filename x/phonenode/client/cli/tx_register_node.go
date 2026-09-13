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

func CmdRegisterNode() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "register-node [address] [model] [os] [role]",
		Short: "Broadcast message register-node",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			argAddress := args[0]
			argModel := args[1]
			argOs := args[2]
			argRole := args[3]

			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := types.NewMsgRegisterNode(
				clientCtx.GetFromAddress().String(),
				argAddress,
				argModel,
				argOs,
				argRole,
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
