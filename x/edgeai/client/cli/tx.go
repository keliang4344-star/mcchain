package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/cosmos/cosmos-sdk/client"
	"mcchain/x/edgeai/types"
)

func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("%s transactions subcommands", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}
	cmd.AddCommand(CmdCreateTask())
	cmd.AddCommand(CmdSubmitResult())
	cmd.AddCommand(CmdOpenDispute())
	return cmd
}

func GetQueryCmd(queryRoute string) *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("Querying commands for the %s module", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}
	cmd.AddCommand(CmdQueryParams())
	cmd.AddCommand(CmdQueryTask())
	cmd.AddCommand(CmdQueryTasks())
	cmd.AddCommand(CmdQueryResults())
	cmd.AddCommand(CmdQueryDisputes())
	return cmd
}
