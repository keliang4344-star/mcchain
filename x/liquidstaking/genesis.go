package liquidstaking

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/liquidstaking/keeper"
	"mcchain/x/liquidstaking/types"
)

// InitGenesis writes the module genesis into the store.
func InitGenesis(ctx sdk.Context, k keeper.Keeper, gs types.GenesisState) {
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
