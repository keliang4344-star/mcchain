package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/spf13/cobra"

	"mcchain/x/dex/types"
)

// GetTxCmd returns the transaction commands for the dex module.
func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("%s transactions subcommands", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(CmdCreatePool())
	cmd.AddCommand(CmdAddLiquidity())
	cmd.AddCommand(CmdRemoveLiquidity())
	cmd.AddCommand(CmdSwapExactIn())
	cmd.AddCommand(CmdSubmitSettlementBatch())
	cmd.AddCommand(CmdFinalizeSettlementBatch())

	return cmd
}

func CmdCreatePool() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-pool [denom-a] [denom-b] [amount-a] [amount-b] [fee-rate-bps]",
		Short: "Create a constant-product liquidity pool",
		Args:  cobra.ExactArgs(5),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			feeBps, err := strconv.ParseUint(args[4], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid fee_rate_bps: %w", err)
			}
			msg := &types.MsgCreatePool{
				Creator:    clientCtx.GetFromAddress().String(),
				DenomA:     args[0],
				DenomB:     args[1],
				AmountA:    args[2],
				AmountB:    args[3],
				FeeRateBps: uint32(feeBps),
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

func CmdAddLiquidity() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-liquidity [pool-id] [amount-a-max] [amount-b-max] [min-lp-out]",
		Short: "Add liquidity to a pool",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			poolID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid pool_id: %w", err)
			}
			msg := &types.MsgAddLiquidity{
				Creator:    clientCtx.GetFromAddress().String(),
				PoolId:     poolID,
				AmountAMax: args[1],
				AmountBMax: args[2],
				MinLpOut:   args[3],
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

func CmdRemoveLiquidity() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove-liquidity [pool-id] [lp-amount] [min-a-out] [min-b-out]",
		Short: "Remove liquidity from a pool",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			poolID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid pool_id: %w", err)
			}
			msg := &types.MsgRemoveLiquidity{
				Creator:  clientCtx.GetFromAddress().String(),
				PoolId:   poolID,
				LpAmount: args[1],
				MinAOut:  args[2],
				MinBOut:  args[3],
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

func CmdSwapExactIn() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "swap-exact-in [pool-id] [denom-in] [amount-in] [denom-out] [min-amount-out]",
		Short: "Swap an exact input amount for the paired denom",
		Args:  cobra.ExactArgs(5),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			poolID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid pool_id: %w", err)
			}
			msg := &types.MsgSwapExactIn{
				Creator:      clientCtx.GetFromAddress().String(),
				PoolId:       poolID,
				DenomIn:      args[1],
				AmountIn:     args[2],
				DenomOut:     args[3],
				MinAmountOut: args[4],
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

// CmdSubmitSettlementBatch 提交离链聚合微结算批次。
// entries 格式：addr:amount,addr:amount（amount 单位 umc）。
func CmdSubmitSettlementBatch() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-settlement-batch [batch-id] [merkle-root-hex] [entries]",
		Short: "Submit an off-chain aggregated micro-settlement batch (entries: addr:amount,addr:amount)",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			entries, err := parseSettlementEntries(args[2])
			if err != nil {
				return err
			}
			msg := &types.MsgSubmitSettlementBatch{
				Creator:    clientCtx.GetFromAddress().String(),
				BatchId:    args[0],
				MerkleRoot: args[1],
				Entries:    entries,
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

func CmdFinalizeSettlementBatch() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "finalize-settlement-batch [batch-id]",
		Short: "Finalize a pending settlement batch and pay out all recipients",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			msg := &types.MsgFinalizeSettlementBatch{
				Creator: clientCtx.GetFromAddress().String(),
				BatchId: args[0],
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

// parseSettlementEntries 解析 "addr:amount,addr:amount" 形式的结算条目列表。
func parseSettlementEntries(raw string) ([]*types.SettlementEntry, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("entries must not be empty")
	}
	parts := strings.Split(raw, ",")
	entries := make([]*types.SettlementEntry, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		kv := strings.SplitN(p, ":", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid entry %q, expected format addr:amount", p)
		}
		amount, err := strconv.ParseUint(strings.TrimSpace(kv[1]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid amount in entry %q: %w", p, err)
		}
		entries = append(entries, &types.SettlementEntry{
			Recipient: strings.TrimSpace(kv[0]),
			AmountUmc: amount,
		})
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("entries must contain at least one addr:amount pair")
	}
	return entries, nil
}
