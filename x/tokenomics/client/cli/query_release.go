package cli

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	"mcchain/x/tokenomics/types"
)

// CmdQueryRelease 实现 `mcchaind q tokenomics release`。
func CmdQueryRelease() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release",
		Short: "查询团队池释放进度（已释放/未释放/进度），基于当前区块时间实时计算",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.Release(cmd.Context(), &types.QueryReleaseRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}
