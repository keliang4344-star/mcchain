package keeper

import storetypes "github.com/cosmos/cosmos-sdk/store/types"

// StoreKey 暴露模块主 KVStore 的 store key。
//
// 唯一用途：让模块的 ExportGenesis / InitGenesis 能通过 internal/kvgenesis
// 对运行期状态做全量快照与回灌（导出/导入必须无损）。
// 除创世路径外，任何业务逻辑都不应直接操作裸 KVStore —— 请使用具体访问器。
func (k Keeper) StoreKey() storetypes.StoreKey {
	return k.storeKey
}
