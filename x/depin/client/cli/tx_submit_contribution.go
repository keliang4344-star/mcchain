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

func CmdSubmitContribution() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-contribution [task-id] [task-type] [score]",
		Short: "Broadcast message submit-contribution",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			argTaskId := args[0]
			argTaskType := args[1]
			argScore := args[2]

			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := types.NewMsgSubmitContribution(
				clientCtx.GetFromAddress().String(),
				argTaskId,
				argTaskType,
				argScore,
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
