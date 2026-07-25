package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"mcchain/x/depin/types"
)

// GetParams get all parameters as types.Params, read from the param subspace.
func (k Keeper) GetParams(ctx sdk.Context) types.Params {
	var p types.Params
	k.paramstore.GetParamSet(ctx, &p)
	return p
}

// SetParams set the params
func (k Keeper) SetParams(ctx sdk.Context, params types.Params) {
	k.paramstore.SetParamSet(ctx, &params)
}
