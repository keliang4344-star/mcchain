package mcchain

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"mcchain/internal/kvgenesis"
	"mcchain/x/mcchain/keeper"
	"mcchain/x/mcchain/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func InitGenesis(ctx sdk.Context, k keeper.Keeper, genState types.GenesisState) {
	// 起链路径必须执行创世校验。
	if err := genState.Validate(); err != nil {
		panic(fmt.Sprintf("mcchain: invalid genesis state: %v", err))
	}
	// this line is used by starport scaffolding # genesis/module/init
	// 先回灌运行期状态快照，再写 params（顺序理由见
	// x/referral/keeper/genesis.go 同名注释）。
	kvgenesis.Restore(ctx, k.StoreKey(), kvgenesis.FromProto(genState.StateKv))
	k.SetParams(ctx, genState.Params)

	// 显式播种渐进治理移交配置。GenesisState 的 Params 字段不含该配置，
	// 这里写入的是全网确定性默认值（治理主体 = 链上治理模块账户）。
	// 显式播种的价值在于：升级/对账时能直接读到一条真实记录，而不是依赖
	// 「键不存在 ⇒ 返回默认值」这条隐式路径——后者正是死开关长期未被察觉的原因。
	//
	// 只有在快照里没有该配置时才播种（否则会把已在链上生效的移交状态
	// 覆盖回默认值，等于悄悄撤销一次治理移交）。
	if !k.HasGovernanceHandoverConfig(ctx) {
		k.SetGovernanceHandoverConfig(ctx, keeper.DefaultGovernanceHandoverConfig())
	}
}

// ExportGenesis returns the module's exported genesis
//
// 导出若只写 params，渐进治理移交配置（GovernanceHandoverConfig）
// 在导入后被静默重置回默认值。故导出附带模块 KVStore 的全量原始快照。
func ExportGenesis(ctx sdk.Context, k keeper.Keeper) *types.GenesisState {
	genesis := types.DefaultGenesis()
	genesis.Params = k.GetParams(ctx)

	// this line is used by starport scaffolding # genesis/module/export
	entries := kvgenesis.Dump(ctx, k.StoreKey())
	ctx.Logger().Info("mcchain: exporting genesis state", "kv_entries", len(entries))
	genesis.StateKv = kvgenesis.ToProto(entries, func(key, value []byte) types.StateKV {
		return types.StateKV{Key: key, Value: value}
	})

	return genesis
}
