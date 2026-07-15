package cli

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	"mcchain/x/tokenomics/types"
)

// CmdQueryAllocations 实现 `mcchaind q tokenomics allocations`。
func CmdQueryAllocations() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "allocations",
		Short: "查询三大池（团队/社区/生态）的分配占比、拨付额与当前余额",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.Allocations(cmd.Context(), &types.QueryAllocationsRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}
