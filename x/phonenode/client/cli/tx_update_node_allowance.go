package cli

import (
	"fmt"
	"strconv"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/spf13/cobra"

	"mcchain/x/phonenode/types"
)

const (
	flagAllowanceDeposit = "deposit"
	flagAllowanceTitle   = "title"
	flagAllowanceSummary = "summary"
)

// CmdUpdateNodeAllowance 提交一份治理提案，经链上投票调整节点资本津贴配置。
//
// 节点资本津贴（白皮书 §32「治理参数，不是固定权利」）必须有链上治理入口，否则
// 默认值 30 MC/节点/日不可调——这是主网前必须补齐的治理旋钮。
// 本命令把 MsgUpdateNodeAllowance 包装为 gov v1 提案广播；提案通过后由治理模块
// 账户执行，MsgServer 二次校验 authority == gov 模块账户，确保仅治理可变更。
func CmdUpdateNodeAllowance() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update-node-allowance [enabled] [per-day-umc]",
		Short: "Submit a governance proposal to update the node capital allowance config",
		Long: `Submit a governance proposal that updates the node capital allowance.

The node capital allowance (a per-node, per-day construction premium drawn from the
device-incentive pool) is a governance parameter. This command wraps MsgUpdateNodeAllowance
in a gov v1 proposal; once the proposal passes, the governance module account executes it.
Set per-day-umc to 0 to pause payouts.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			enabled, err := strconv.ParseBool(args[0])
			if err != nil {
				return fmt.Errorf("enabled must be true or false: %w", err)
			}
			perDay, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return fmt.Errorf("per-day-umc must be a non-negative integer: %w", err)
			}

			authority := authtypes.NewModuleAddress(govtypes.ModuleName).String()
			msg := types.NewMsgUpdateNodeAllowance(authority, enabled, perDay)

			depositStr, _ := cmd.Flags().GetString(flagAllowanceDeposit)
			deposit, err := sdk.ParseCoinsNormalized(depositStr)
			if err != nil {
				return fmt.Errorf("invalid deposit %q: %w", depositStr, err)
			}

			title, _ := cmd.Flags().GetString(flagAllowanceTitle)
			if title == "" {
				title = "Update node capital allowance"
			}
			summary, _ := cmd.Flags().GetString(flagAllowanceSummary)
			if summary == "" {
				summary = fmt.Sprintf("Set node capital allowance enabled=%t per_day=%d umc", enabled, perDay)
			}

			proposer := clientCtx.GetFromAddress().String()
			proposal, err := govv1.NewMsgSubmitProposal([]sdk.Msg{msg}, deposit, proposer, "", title, summary)
			if err != nil {
				return err
			}
			if err := proposal.ValidateBasic(); err != nil {
				return err
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), proposal)
		},
	}

	cmd.Flags().String(flagAllowanceDeposit, "10000000umc", "initial deposit for the proposal")
	cmd.Flags().String(flagAllowanceTitle, "", "proposal title")
	cmd.Flags().String(flagAllowanceSummary, "", "proposal summary")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
