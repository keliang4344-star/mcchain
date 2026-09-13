package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/internal/kvgenesis"
	"mcchain/x/referral/types"
)

// InitGenesis initializes the referral module's state from genesis data.
func InitGenesis(ctx sdk.Context, k Keeper, genState types.GenesisState) {
	// 起链路径必须执行创世校验。referral 参数直接决定
	// 返佣代数与费率，带非法值起链会让每一笔返佣都算错。
	if err := genState.Validate(); err != nil {
		panic(fmt.Sprintf("referral: invalid genesis state: %v", err))
	}

	// 先回灌运行期状态快照，再写 params。
	//
	// 顺序是刻意的：快照里也包含 params 子空间写入的键（params 直接存于模块
	// KVStore），因此必须让 GenesisState.Params 最后落盘，保证「创世文件里
	// 的参数」永远是权威来源——人工改过参数的创世文件不会被旧快照里的参数
	// 悄悄覆盖回去。
	//
	// 快照为空时行为与历史完全一致（仅按 params 初始化），对既有创世文件
	// 向后兼容。
	kvgenesis.Restore(ctx, k.StoreKey(), kvgenesis.FromProto(genState.StateKv))

	k.SetParams(ctx, genState.Params)
}

// ExportGenesis returns the referral module's genesis state.
//
// 导出必须无损：只导出 Params 时，「导出旧链 → 作为新链创世」
// 这条迁移路径会把推荐关系树（value/count/inviter/invitee 四套索引）、待发返佣、
// 日熔断计数器、清理游标、延后账本、释放节奏配置全部清零，而生成出来的创世文件
// 语法完全合法、validate-genesis 也会 PASS——属于最难被发现的一类故障。
//
// 现在改为「Params + 模块 KVStore 全量原始快照」：完整性由构造保证，
// 新增任何索引/计数器都不需要再改这里。
func ExportGenesis(ctx sdk.Context, k Keeper) *types.GenesisState {
	entries := kvgenesis.Dump(ctx, k.StoreKey())
	ctx.Logger().Info("referral: exporting genesis state",
		"kv_entries", len(entries))
	return &types.GenesisState{
		Params: k.GetParams(ctx),
		StateKv: kvgenesis.ToProto(entries, func(key, value []byte) types.StateKV {
			return types.StateKV{Key: key, Value: value}
		}),
	}
}
