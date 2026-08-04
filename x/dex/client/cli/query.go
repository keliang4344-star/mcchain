package cli

import (
	"fmt"
	"strconv"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	"mcchain/x/dex/types"
)

// GetQueryCmd returns the query commands for the dex module.
func GetQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("Querying commands for the %s module", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(CmdQueryPool())
	cmd.AddCommand(CmdQueryPools())
	cmd.AddCommand(CmdQueryEstimateSwap())
	cmd.AddCommand(CmdQueryPrice())
	cmd.AddCommand(CmdQueryLiquidityLock())

	return cmd
}

func CmdQueryPool() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pool [pool-id]",
		Short: "Query a single liquidity pool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			poolID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid pool_id: %w", err)
			}
			res, err := types.NewQueryClient(clientCtx).Pool(cmd.Context(), &types.QueryPoolRequest{PoolId: poolID})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdQueryPools() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pools",
		Short: "Query all liquidity pools",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			res, err := types.NewQueryClient(clientCtx).Pools(cmd.Context(), &types.QueryPoolsRequest{})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdQueryEstimateSwap() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "estimate-swap [pool-id] [denom-in] [amount-in] [denom-out]",
		Short: "Estimate the output amount of a swap without broadcasting",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			poolID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid pool_id: %w", err)
			}
			res, err := types.NewQueryClient(clientCtx).EstimateSwap(cmd.Context(), &types.QueryEstimateSwapRequest{
				PoolId:   poolID,
				DenomIn:  args[1],
				AmountIn: args[2],
				DenomOut: args[3],
			})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdQueryPrice() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "price [pool-id] [denom]",
		Short: "Query the spot price of a denom in a pool",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			poolID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid pool_id: %w", err)
			}
			res, err := types.NewQueryClient(clientCtx).Price(cmd.Context(), &types.QueryPriceRequest{
				PoolId: poolID,
				Denom:  args[1],
			})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdQueryLiquidityLock() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "liquidity-lock [lp-address] [pool-id]",
		Short: "Query the liquidity lock state of an LP in a pool",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			poolID, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid pool_id: %w", err)
			}
			res, err := types.NewQueryClient(clientCtx).LiquidityLock(cmd.Context(), &types.QueryLiquidityLockRequest{
				LpAddress: args[0],
				PoolId:    poolID,
			})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
