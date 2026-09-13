package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Pools:      []Pool{},
		NextPoolId: 1,
		Params:     DefaultParams(),
	}
}

// Validate performs a full structural check of the exported/imported DEX state.
//
// Genesis is the one place where pool state is accepted verbatim without going
// through the keeper's message path, so every invariant the AMM relies on has
// to be re-asserted here: sorted denom pairs, parseable non-negative reserves,
// a bounded fee rate, unique pool IDs and pairs, and a NextPoolId that is
// actually ahead of every stored pool.
func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return err
	}
	if gs.NextPoolId == 0 {
		return sdkerrors.Wrapf(ErrInvalidPoolID, "next_pool_id must be >= 1")
	}

	seenIDs := make(map[uint64]struct{}, len(gs.Pools))
	seenPairs := make(map[string]struct{}, len(gs.Pools))

	for _, pool := range gs.Pools {
		if pool.Id == 0 {
			return sdkerrors.Wrapf(ErrInvalidPoolID, "pool id must be >= 1")
		}
		if pool.Id >= gs.NextPoolId {
			return sdkerrors.Wrapf(ErrInvalidPoolID,
				"pool id %d must be below next_pool_id %d", pool.Id, gs.NextPoolId)
		}
		if _, dup := seenIDs[pool.Id]; dup {
			return sdkerrors.Wrapf(ErrInvalidPoolID, "duplicate pool id %d", pool.Id)
		}
		seenIDs[pool.Id] = struct{}{}

		if pool.DenomA == "" || pool.DenomB == "" {
			return ErrInvalidDenom
		}
		if err := sdk.ValidateDenom(pool.DenomA); err != nil {
			return sdkerrors.Wrapf(ErrInvalidDenom, "pool %d denom_a: %s", pool.Id, err)
		}
		if err := sdk.ValidateDenom(pool.DenomB); err != nil {
			return sdkerrors.Wrapf(ErrInvalidDenom, "pool %d denom_b: %s", pool.Id, err)
		}
		if pool.DenomA == pool.DenomB {
			return ErrDuplicateDenom
		}
		if pool.DenomA > pool.DenomB {
			return sdkerrors.Wrapf(ErrDenomSortRequired, "pool %d: %s > %s", pool.Id, pool.DenomA, pool.DenomB)
		}
		pair := pool.DenomA + "/" + pool.DenomB
		if _, dup := seenPairs[pair]; dup {
			return sdkerrors.Wrapf(ErrDuplicateDenom, "duplicate pool for pair %s", pair)
		}
		seenPairs[pair] = struct{}{}

		if pool.FeeRateBps > MaxPoolFeeRateBps {
			return sdkerrors.Wrapf(ErrInvalidFeeRate,
				"pool %d fee_rate_bps must be <= %d, got %d", pool.Id, MaxPoolFeeRateBps, pool.FeeRateBps)
		}

		reserveA, okA := sdk.NewIntFromString(pool.ReserveA)
		reserveB, okB := sdk.NewIntFromString(pool.ReserveB)
		totalLP, okLP := sdk.NewIntFromString(pool.TotalLp)
		if !okA || !okB || !okLP {
			return sdkerrors.Wrapf(ErrInvalidDenom, "pool %d has unparseable reserve or LP amount", pool.Id)
		}
		if reserveA.IsNegative() || reserveB.IsNegative() || totalLP.IsNegative() {
			return sdkerrors.Wrapf(ErrInvalidDenom, "pool %d has a negative reserve or LP supply", pool.Id)
		}
		// A live LP supply must be backed by reserves on both legs, otherwise
		// the AMM divides by a zero reserve on the next deposit.
		if totalLP.IsPositive() && (!reserveA.IsPositive() || !reserveB.IsPositive()) {
			return sdkerrors.Wrapf(ErrPoolEmpty, "pool %d has LP supply but an empty reserve", pool.Id)
		}
		// Reserves with no LP supply are unredeemable — the assets would be
		// permanently stranded in the module account.
		if !totalLP.IsPositive() && (reserveA.IsPositive() || reserveB.IsPositive()) {
			return sdkerrors.Wrapf(ErrInsufficientLiquidity, "pool %d has reserves but no LP supply", pool.Id)
		}
	}
	return nil
}
