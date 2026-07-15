package cli

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	"mcchain/x/tokenomics/types"
)

// CmdQuerySupply 实现 `mcchaind q tokenomics supply`。
func CmdQuerySupply() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "supply",
		Short: "查询总量上限（cap）与已发行量（minted_supply）",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.Supply(cmd.Context(), &types.QuerySupplyRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}
