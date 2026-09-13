package dex

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/internal/daywindow"
	"mcchain/internal/kvgenesis"
	"mcchain/x/dex/keeper"
	"mcchain/x/dex/types"
)

func InitGenesis(ctx sdk.Context, k keeper.Keeper, genState types.GenesisState) {
	// 起链路径必须执行创世校验（原只在 validate-genesis
	// 子命令里跑）。DEX 的池不变量（储备非负、LP 总量与储备自洽、手续费率区间）
	// 若带着非法值落盘，后续每一笔 swap / add_liquidity 都在错误基数上计算，
	// 且只有靠治理迁移才能纠正。起链是拒绝坏状态最廉价的时机。
	if err := genState.Validate(); err != nil {
		panic(fmt.Sprintf("dex: invalid genesis state: %v", err))
	}
	// 先回灌运行期状态快照（含 params 子空间的键），
	// 随后由类型化字段与「仅在缺失时」的默认值接管。顺序保证：
	//   1) Params 以创世文件为准（人工可编辑）；
	//   2) pools / next_pool_id 以创世文件为准（人工可编辑）；
	//   3) LP 激励日标记、结算配置等纯运行期状态以快照为准，不被默认值倒灌。
	kvgenesis.Restore(ctx, k.StoreKey(), kvgenesis.FromProto(genState.StateKv))

	k.SetParams(ctx, genState.Params)

	for _, pool := range genState.Pools {
		k.SetPool(ctx, pool)
	}

	k.SetNextPoolID(ctx, genState.NextPoolId)

	// 把 LP 激励的「上次发放日」初始化为创世当日，使首个发放落在下一个
	// UTC 日界，而不是创世后的第一个区块（否则会比设计多发放一天）。
	//
	// 仅当快照未提供时才播种 —— 否则一次导出/导入会把已经推进的
	// 发放日倒回「导出当时的日界之前」，造成重复发放。
	if _, ok := k.GetLastLPIncentiveDay(ctx); !ok {
		k.SetLastLPIncentiveDay(ctx, daywindow.DayIndex(ctx))
	}

	// 显式落盘结算配置的初始状态。
	//
	// 这份配置若从未被任何代码写过（SetSettlementConfig 全仓零调用者），于是
	// 它永远停在默认值 —— Authority = 治理模块账户。而治理模块账户无法签署交易，
	// 结算通道对任何人都是 unauthorized：链上状态显示「未熔断（已启用）」，功能却
	// 永久不可用。这属于最难排查的一类故障（不是不安全，而是不可用且不自知）。
	//
	// 现在创世显式写入：未配置运营地址 → Halted = true，让链上状态说真话。
	// 开启路径见 keeper.EnableSettlement（由升级 Handler 调用），该函数会校验目标
	// 地址必须是可签名的普通账户，杜绝把同一个死锁换个地址再复现一遍。
	//
	// 仅当快照未提供时才写熔断默认态 —— 否则已在链上开启的结算通道
	// 会因一次导出/导入被重新熔断（运营侧表现为「结算突然全部不可用」）。
	if !k.HasSettlementConfig(ctx) {
		k.SetSettlementConfig(ctx, keeper.SettlementConfig{
			Authority: keeper.DefaultSettlementConfig().Authority,
			Halted:    true,
		})
		ctx.EventManager().EmitEvent(sdk.NewEvent(
			types.EventTypeSettlementDefaulted,
			sdk.NewAttribute(types.AttrReason, "no settlement operator configured at genesis"),
		))
	}

	// Whitepaper lines 504-505: create the initial MC/USDT pool at genesis
	// if it doesn't already exist.
	k.InitGenesisPool(ctx)
}

func ExportGenesis(ctx sdk.Context, k keeper.Keeper) *types.GenesisState {
	pools := k.GetAllPools(ctx)
	nextPoolID := k.GetNextPoolID(ctx)
	params := k.GetParams(ctx)

	// 导出若只写 pools / next_pool_id / params，
	// 遗漏了结算配置（SettlementConfig：Authority + Halted）与 LP 锁仓记录
	// （LiquidityLock），以及 LP 激励日标记。后果：
	//   - 导入后 Halted 被重置为 true，已开启的结算通道重新熔断；
	//   - LP 锁仓记录丢失，锁定期形同虚设（可立即撤池）。
	// 故导出附带模块 KVStore 的全量原始快照；pools/next_pool_id/params 仍由
	// 上面的类型化字段承载（它们是允许人工编辑的字段，导入时以类型化字段为准）。
	entries := kvgenesis.Dump(ctx, k.StoreKey())
	ctx.Logger().Info("dex: exporting genesis state", "kv_entries", len(entries))

	return &types.GenesisState{
		Pools:      pools,
		NextPoolId: nextPoolID,
		Params:     params,
		StateKv: kvgenesis.ToProto(entries, func(key, value []byte) types.StateKV {
			return types.StateKV{Key: key, Value: value}
		}),
	}
}
