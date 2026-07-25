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

func CmdAttestDevice() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attest-device [address] [challenge] [signature]",
		Short: "Broadcast message attest-device",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			argAddress := args[0]
			argChallenge := args[1]
			argSignature := args[2]

			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := types.NewMsgAttestDevice(
				clientCtx.GetFromAddress().String(),
				argAddress,
				argChallenge,
				argSignature,
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
