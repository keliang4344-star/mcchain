package cli

import (
	"strconv"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/spf13/cobra"
	"mcchain/x/edgeai/types"
)

var _ = strconv.Itoa(0)

func CmdCreateTask() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-task [description] [reward]",
		Short: "Create an AI task with reward (umc)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil { return err }
			reward, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil { return err }
			msg := types.NewMsgCreateTask(clientCtx.GetFromAddress().String(), args[0], reward)
			if err := msg.ValidateBasic(); err != nil { return err }
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

func CmdSubmitResult() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-result [task_id] [result_hash] [attestation_nonce]",
		Short: "Submit AI task result with attestation",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil { return err }
			msg := types.NewMsgSubmitResult(clientCtx.GetFromAddress().String(), args[0], args[1], args[2])
			if err := msg.ValidateBasic(); err != nil { return err }
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

func CmdOpenDispute() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "open-dispute [task_id] [reason]",
		Short: "Open a dispute on an AI task result",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil { return err }
			msg := types.NewMsgOpenDispute(clientCtx.GetFromAddress().String(), args[0], args[1])
			if err := msg.ValidateBasic(); err != nil { return err }
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
