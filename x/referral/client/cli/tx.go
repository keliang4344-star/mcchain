package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"

	"mcchain/x/referral/types"
)

// GetTxCmd returns the transaction commands for the referral module.
func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("%s transactions subcommands", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(CmdCreateReferral())
	cmd.AddCommand(CmdClaimReferralReward())

	return cmd
}

// CmdCreateReferral binds the sender (invitee) to an inviter through an invite code.
//
// 签名方是**被推荐人自己**——「我确认由 X 推荐」。
// 推荐人无法替他人发起绑定，因此不存在抢先占用他人推荐关系的可能。
func CmdCreateReferral() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-referral [inviter] [invite-code]",
		Short: "Bind your address to an inviter via an invite code (signed by you, the invitee)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			msg := &types.MsgCreateReferral{
				Inviter:    args[0],
				Invitee:    clientCtx.GetFromAddress().String(),
				InviteCode: args[1],
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

// CmdClaimReferralReward collects the sender's pending referral rewards.
func CmdClaimReferralReward() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "claim-reward",
		Short: "Claim the sender's pending referral rewards",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			msg := &types.MsgClaimReferralReward{
				Claimer: clientCtx.GetFromAddress().String(),
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
