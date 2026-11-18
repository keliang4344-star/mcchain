package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"mcchain/internal/chainprofile"
)

// ProfileCmd 输出 chain-id 的部署档位，供脚本/readiness 复用应用层判定。
//
// 为什么需要它：启动脚本曾用 `*mainnet*` 子串自行判断档位，与应用层的
// chainprofile 规则各说各话——同一份 genesis 在脚本里按主网拦、在应用里
// 按测试网放（或反之）。把判定收敛到编译进二进制的唯一事实源后，
// 脚本只能通过本命令获取档位，不允许再自行匹配字符串。
//
// 退出码：0 = 已登记；1 = 未知/非法（脚本据此 fail-closed）。
func ProfileCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "profile <chain-id>",
		Short: "Print the deployment profile (production|test|development) for a chain-id",
		Long: `Print the deployment profile for an exact chain-id.

The mapping lives in internal/chainprofile and is shared by the application,
CLI and deployment scripts. Unknown or malformed chain-ids exit non-zero so
callers fail closed instead of guessing from substrings.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := chainprofile.Resolve(args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(p))
			return nil
		},
		SilenceUsage: true,
	}
}
