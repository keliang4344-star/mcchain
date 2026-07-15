package cli

import (
	"strconv"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/spf13/cobra"
	"mcchain/x/depin/types"
)

var _ = strconv.Itoa(0)

func CmdRegisterDevice() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "register-device [address] [model] [os]",
		Short: "Broadcast message register-device",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			argAddress := args[0]
			argModel := args[1]
			argOs := args[2]

			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := types.NewMsgRegisterDevice(
				clientCtx.GetFromAddress().String(),
				argAddress,
				argModel,
				argOs,
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
