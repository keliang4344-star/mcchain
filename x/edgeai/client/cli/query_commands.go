package cli

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"
	"mcchain/x/edgeai/types"
)

func CmdQueryParams() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "shows the parameters of the module",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil { return err }
			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.Params(cmd.Context(), &types.QueryParamsRequest{})
			if err != nil { return err }
			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdQueryTask() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task [task_id]",
		Short: "query a task by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil { return err }
			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.Task(cmd.Context(), &types.QueryTaskRequest{TaskId: args[0]})
			if err != nil { return err }
			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdQueryTasks() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-tasks",
		Short: "list all task ids",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil { return err }
			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.Tasks(cmd.Context(), &types.QueryTasksRequest{})
			if err != nil { return err }
			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdQueryResults() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-results",
		Short: "list all submitted results (JSON)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil { return err }
			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.Results(cmd.Context(), &types.QueryResultsRequest{})
			if err != nil { return err }
			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdQueryDisputes() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-disputes",
		Short: "list all disputes (JSON)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil { return err }
			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.Disputes(cmd.Context(), &types.QueryDisputesRequest{})
			if err != nil { return err }
			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
