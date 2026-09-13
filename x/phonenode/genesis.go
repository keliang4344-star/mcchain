package phonenode

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"mcchain/internal/kvgenesis"
	"mcchain/x/phonenode/keeper"
	"mcchain/x/phonenode/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func InitGenesis(ctx sdk.Context, k keeper.Keeper, genState types.GenesisState) {
	// 起链路径必须执行创世校验。
	if err := genState.Validate(); err != nil {
		panic(fmt.Sprintf("phonenode: invalid genesis state: %v", err))
	}
	// this line is used by starport scaffolding # genesis/module/init
	// 先回灌运行期状态快照，再写 params（顺序理由见
	// x/referral/keeper/genesis.go 同名注释）。
	kvgenesis.Restore(ctx, k.StoreKey(), kvgenesis.FromProto(genState.StateKv))
	k.SetParams(ctx, genState.Params)
}

// ExportGenesis returns the module's exported genesis
//
// 导出若只写 params，导出/导入会把全部节点注册表、attestation
// 状态与层级、心跳 FIFO 索引、slash 记录与冷却、设备绑定与公钥、nonce 重放索引、
// 津贴发放日标记清零 —— 迁移后所有已注册设备凭空消失。故导出附带全量原始快照。
func ExportGenesis(ctx sdk.Context, k keeper.Keeper) *types.GenesisState {
	genesis := types.DefaultGenesis()
	genesis.Params = k.GetParams(ctx)

	// this line is used by starport scaffolding # genesis/module/export
	entries := kvgenesis.Dump(ctx, k.StoreKey())
	ctx.Logger().Info("phonenode: exporting genesis state", "kv_entries", len(entries))
	genesis.StateKv = kvgenesis.ToProto(entries, func(key, value []byte) types.StateKV {
		return types.StateKV{Key: key, Value: value}
	})

	return genesis
}
