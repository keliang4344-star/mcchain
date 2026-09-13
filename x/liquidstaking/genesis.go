package liquidstaking

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/liquidstaking/keeper"
	"mcchain/x/liquidstaking/types"
)

// InitGenesis writes the module genesis into the store.
func InitGenesis(ctx sdk.Context, k keeper.Keeper, gs types.GenesisState) {
	// 起链路径必须执行创世校验。liquidstaking 的池状态与
	// 解绑队列若带非法值落盘，兑换率会在错误基数上累积，且必须靠治理迁移纠正。
	if err := gs.Validate(); err != nil {
		panic(fmt.Sprintf("liquidstaking: invalid genesis state: %v", err))
	}
	if err := k.SetParams(ctx, gs.Params); err != nil {
		panic(err)
	}
	k.SetPoolState(ctx, gs.PoolState)
	for _, e := range gs.UnbondingQueue {
		k.SetUnbondingEntry(ctx, e)
	}
	k.ImportValidatorBonds(ctx, gs.ValidatorBonds)
	k.ImportNextUnbondingID(ctx, gs.NextUnbondingID)
}

// ExportGenesis dumps the module state.
func ExportGenesis(ctx sdk.Context, k keeper.Keeper) types.GenesisState {
	return types.GenesisState{
		Params:          k.GetParams(ctx),
		PoolState:       k.GetPoolState(ctx),
		UnbondingQueue:  k.AllUnbondings(ctx),
		ValidatorBonds:  k.AllValidatorBonds(ctx),
		NextUnbondingID: k.NextUnbondingID(ctx),
	}
}
