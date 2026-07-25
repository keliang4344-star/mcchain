package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"

	"mcchain/x/tokenomics/types"
)

// GetQueryCmd 返回 tokenomics 模块的查询命令根节点。
func GetQueryCmd(_ string) *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("Querying commands for the %s module", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(CmdQuerySupply())
	cmd.AddCommand(CmdQueryAllocations())
	cmd.AddCommand(CmdQueryRelease())

	return cmd
}
