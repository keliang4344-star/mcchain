package edgeai

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"mcchain/internal/kvgenesis"
	"mcchain/x/edgeai/keeper"
	"mcchain/x/edgeai/types"
)

func InitGenesis(ctx sdk.Context, k keeper.Keeper, genState types.GenesisState) {
	// 起链路径必须执行创世校验。edgeai 的参数直接决定
	// 托管金与争议仲裁行为（85/15 拆分、仲裁人地址、验证者预留窗口），带非法值
	// 起链会导致每一笔任务的结算基数都是错的。
	if err := genState.Validate(); err != nil {
		panic(fmt.Sprintf("edgeai: invalid genesis state: %v", err))
	}
	// 先回灌运行期状态快照，再写 params。
	// 顺序刻意如此：快照里也含 params 子空间的键，让 GenesisState.Params
	// 最后落盘才能保证「创世文件里的参数」是权威来源，不会被旧快照覆盖回去
	// （详见 x/referral/keeper/genesis.go 同名注释）。
	kvgenesis.Restore(ctx, k.StoreKey(), kvgenesis.FromProto(genState.StateKv))
	k.SetParams(ctx, genState.Params)
}

// ExportGenesis returns the module's exported genesis.
//
// 导出若只写 params，导出/导入会把全部在途任务（状态机各阶段）、
// 托管金、验证结果与 pending 索引、争议与仲裁记录、验证者预留池、声誉值清零
// —— 在途任务的托管资金会凭空消失。故导出附带模块 KVStore 的全量原始快照。
func ExportGenesis(ctx sdk.Context, k keeper.Keeper) *types.GenesisState {
	genesis := types.DefaultGenesis()
	genesis.Params = k.GetParams(ctx)
	entries := kvgenesis.Dump(ctx, k.StoreKey())
	ctx.Logger().Info("edgeai: exporting genesis state", "kv_entries", len(entries))
	genesis.StateKv = kvgenesis.ToProto(entries, func(key, value []byte) types.StateKV {
		return types.StateKV{Key: key, Value: value}
	})
	return genesis
}
