package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/liquidstaking/types"
)

// ImportValidatorBonds restores per-validator bond tracking from genesis.
func (k Keeper) ImportValidatorBonds(ctx sdk.Context, bonds []types.ValidatorBond) {
	for _, vb := range bonds {
		k.setValidatorBond(ctx, vb.Validator, vb.AmountUmc)
	}
}

// ImportNextUnbondingID restores the redemption receipt counter from genesis.
func (k Keeper) ImportNextUnbondingID(ctx sdk.Context, id uint64) {
	if id == 0 {
		id = 1
	}
	k.setNextUnbondingID(ctx, id)
}
