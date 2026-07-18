package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/referral/types"
)

// InitGenesis initializes the referral module's state from genesis data.
func InitGenesis(ctx sdk.Context, k Keeper, genState types.GenesisState) {
	k.SetParams(ctx, genState.Params)
}

// ExportGenesis returns the referral module's genesis state.
func ExportGenesis(ctx sdk.Context, k Keeper) *types.GenesisState {
	return &types.GenesisState{
		Params: k.GetParams(ctx),
	}
}
