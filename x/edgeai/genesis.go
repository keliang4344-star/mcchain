package edgeai

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"mcchain/x/edgeai/keeper"
	"mcchain/x/edgeai/types"
)

func InitGenesis(ctx sdk.Context, k keeper.Keeper, genState types.GenesisState) {
	k.SetParams(ctx, genState.Params)
}

func ExportGenesis(ctx sdk.Context, k keeper.Keeper) *types.GenesisState {
	genesis := types.DefaultGenesis()
	genesis.Params = k.GetParams(ctx)
	return genesis
}
