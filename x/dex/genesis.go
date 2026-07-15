package dex

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/dex/keeper"
	"mcchain/x/dex/types"
)

func InitGenesis(ctx sdk.Context, k keeper.Keeper, genState types.GenesisState) {
	k.SetParams(ctx, genState.Params)

	for _, pool := range genState.Pools {
		k.SetPool(ctx, pool)
	}

	k.SetNextPoolID(ctx, genState.NextPoolId)
}

func ExportGenesis(ctx sdk.Context, k keeper.Keeper) *types.GenesisState {
	pools := k.GetAllPools(ctx)
	nextPoolID := k.GetNextPoolID(ctx)
	params := k.GetParams(ctx)

	return &types.GenesisState{
		Pools:      pools,
		NextPoolId: nextPoolID,
		Params:     params,
	}
}
