package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"

	"mcchain/x/referral/types"
)

// GetQueryCmd returns the query commands for the referral module.
func GetQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("%s query subcommands", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(CmdReferral())
	cmd.AddCommand(CmdReferralsByInviter())
	cmd.AddCommand(CmdPendingRewards())

	return cmd
}

// CmdReferral fetches a single referral by its id.
func CmdReferral() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "referral [referral-id]",
		Short: "Query a single referral by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			id, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid referral-id: %w", err)
			}
			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.Referral(cmd.Context(), &types.QueryReferralRequest{ReferralId: id})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdReferralsByInviter lists every referral created by an inviter.
func CmdReferralsByInviter() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "referrals-by-inviter [inviter]",
		Short: "List all referrals created by an inviter address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.ReferralsByInviter(cmd.Context(), &types.QueryReferralsByInviterRequest{Inviter: args[0]})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdPendingRewards shows the pending referral rewards for a claimer.
func CmdPendingRewards() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pending-rewards [claimer]",
		Short: "Show pending referral rewards for a claimer address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.PendingRewards(cmd.Context(), &types.QueryPendingRewardsRequest{Claimer: args[0]})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
