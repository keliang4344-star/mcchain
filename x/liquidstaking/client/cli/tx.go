package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"

	"mcchain/x/liquidstaking/types"
)

// GetTxCmd returns the transaction commands for this module.
func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("%s transactions subcommands", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(CmdLiquidStake())
	cmd.AddCommand(CmdLiquidUnstake())
	cmd.AddCommand(CmdClaimMatured())

	return cmd
}

// CmdLiquidStake bonds MC and mints the receipt denom.
func CmdLiquidStake() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "liquid-stake [validator] [amount-umc]",
		Short: "Bond MC through the liquid staking pool and receive ulmc",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			amount, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid amount: %w", err)
			}
			msg := &types.MsgLiquidStake{
				Delegator: clientCtx.GetFromAddress().String(),
				Validator: args[0],
				AmountUmc: amount,
			}
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdLiquidUnstake burns receipt shares and starts unbonding.
func CmdLiquidUnstake() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "liquid-unstake [shares-ulmc] [validator?]",
		Short: "Redeem ulmc and start the unbonding period",
		Long: "Redeem ulmc and start the unbonding period. " +
			"Omit the validator to unbond from whichever validator holds the largest pool bond.",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			shares, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid shares: %w", err)
			}
			validator := ""
			if len(args) == 2 {
				validator = args[1]
			}
			msg := &types.MsgLiquidUnstake{
				Delegator:  clientCtx.GetFromAddress().String(),
				Validator:  validator,
				SharesUlmc: shares,
			}
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdClaimMatured collects every matured unbonding entry.
func CmdClaimMatured() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "claim",
		Short: "Collect every matured unbonding entry",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			msg := &types.MsgClaimMatured{Delegator: clientCtx.GetFromAddress().String()}
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
