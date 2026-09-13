package depin

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"mcchain/internal/kvgenesis"
	"mcchain/x/depin/keeper"
	"mcchain/x/depin/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func InitGenesis(ctx sdk.Context, k keeper.Keeper, genState types.GenesisState) {
	// 创世校验若只在 `validate-genesis` 子命令里跑，
	// 真正 `start` 起链时各模块直接 MustUnmarshalJSON 后落盘，Validate() 从不执行。
	// 于是「创世文件不合法」这件事在起链路径上完全不可观测 —— 手改过创世文件、
	// 或导出/导入过程中字段被截断，链照样起得来，只是带着一份非法状态开始出块。
	// 起链阶段是唯一能廉价拒绝坏状态的时机（拒绝成本 = 起不来，改完重来即可），
	// 一旦出块，非法状态就固化进 appHash，只能靠治理迁移修。故在这里硬失败。
	if err := genState.Validate(); err != nil {
		panic(fmt.Sprintf("depin: invalid genesis state: %v", err))
	}
	// this line is used by starport scaffolding # genesis/module/init
	// 先回灌运行期状态快照，再写 params（顺序见 referral 同处注释）。
	kvgenesis.Restore(ctx, k.StoreKey(), kvgenesis.FromProto(genState.StateKv))
	k.SetParams(ctx, genState.Params)

	// [2]/[7] 预言机白名单初始化：从 DefaultOracleAddresses 播种 KVStore。
	// 生产链若未配置（空）将在 SubmitAttestation 处 fail-closed。
	k.SetOracleWhitelist(ctx, types.DefaultOracleAddresses)

	// 铸币铁律：depin 绝不自铸，模块账户不持有 Minter 权限。
	// InitialPool（默认 types.DefaultInitialPool = 4.675e14 umc = 4.675 亿 MC）
	// 由 tokenomics 在 InitGenesis 从设备激励池切片 DepinInitialPoolSlice
	// (5.5e14 umc) 扣除 ReferralEcosystemBudget (8.25e13 umc) 后，经模块间
	// 转账注入 depin 模块账户（见 x/tokenomics/keeper/genesis.go）。
	// 顺序铁律：tokenomics.InitGenesis 必须排在 depin 之前（app.go SetOrderInitGenesis）。
}

// ExportGenesis returns the module's exported genesis
//
// 导出若只写 params，导出/导入会把全部设备状态、贡献记录、
// attestation 历史、已消费 challenge 索引、七层防线计数器与日累计、
// 线性释放金库游标清零。故导出附带模块 KVStore 的全量原始快照。
func ExportGenesis(ctx sdk.Context, k keeper.Keeper) *types.GenesisState {
	genesis := types.DefaultGenesis()
	genesis.Params = k.GetParams(ctx)

	// this line is used by starport scaffolding # genesis/module/export
	entries := kvgenesis.Dump(ctx, k.StoreKey())
	ctx.Logger().Info("depin: exporting genesis state", "kv_entries", len(entries))
	genesis.StateKv = kvgenesis.ToProto(entries, func(key, value []byte) types.StateKV {
		return types.StateKV{Key: key, Value: value}
	})

	return genesis
}
