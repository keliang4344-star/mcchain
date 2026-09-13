package keeper

import (
	"strconv"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"mcchain/x/liquidstaking/types"
)

// Hooks wires x/liquidstaking into the x/staking validator lifecycle.
//
// Only one event matters to this module: a validator being slashed. The pooled
// stake is delegated through the module account, so a slash silently reduces
// the umc backing every outstanding ulmc share. Without this hook the module's
// own accounting (TotalBondedUmc) would keep the pre-slash figure, the
// ulmc/umc exchange rate would stay artificially high and the first redeemers
// would be paid at a rate the pool can no longer honour — a classic bank run
// on the last holders. The hook writes the loss down immediately so the rate
// always reflects the real backing.
type Hooks struct {
	k Keeper
}

var _ stakingtypes.StakingHooks = Hooks{}

// Hooks returns the staking hook receiver for this module.
func (k Keeper) Hooks() Hooks { return Hooks{k: k} }

// BeforeValidatorSlashed writes the pool's share of a validator slash down.
//
// x/staking passes the *effective* fraction, i.e. tokensToBurn / validator.Tokens.
// Because the module's delegation is denominated in shares of that validator,
// its bond loses exactly the same fraction.
func (h Hooks) BeforeValidatorSlashed(ctx sdk.Context, valAddr sdk.ValAddress, fraction sdk.Dec) error {
	return h.k.ApplyValidatorSlash(ctx, valAddr.String(), fraction)
}

// --- remaining StakingHooks methods are intentional no-ops -----------------

func (Hooks) AfterValidatorCreated(sdk.Context, sdk.ValAddress) error   { return nil }
func (Hooks) BeforeValidatorModified(sdk.Context, sdk.ValAddress) error { return nil }
func (Hooks) AfterValidatorRemoved(sdk.Context, sdk.ConsAddress, sdk.ValAddress) error {
	return nil
}

func (Hooks) AfterValidatorBonded(sdk.Context, sdk.ConsAddress, sdk.ValAddress) error { return nil }
func (Hooks) AfterValidatorBeginUnbonding(sdk.Context, sdk.ConsAddress, sdk.ValAddress) error {
	return nil
}

func (Hooks) BeforeDelegationCreated(sdk.Context, sdk.AccAddress, sdk.ValAddress) error { return nil }
func (Hooks) BeforeDelegationSharesModified(sdk.Context, sdk.AccAddress, sdk.ValAddress) error {
	return nil
}
func (Hooks) BeforeDelegationRemoved(sdk.Context, sdk.AccAddress, sdk.ValAddress) error { return nil }
func (Hooks) AfterDelegationModified(sdk.Context, sdk.AccAddress, sdk.ValAddress) error { return nil }
func (Hooks) AfterUnbondingInitiated(sdk.Context, uint64) error                         { return nil }

// ---------------------------------------------------------------------------
// Slash accounting
// ---------------------------------------------------------------------------

// ApplyValidatorSlash reduces the tracked bond of `valAddr` and the aggregate
// pool by `fraction`, then re-emits the resulting exchange rate.
//
// The loss is rounded *up*: the recorded bond may understate the real one by at
// most 1 umc, never overstate it. Understating keeps the pool provably solvent;
// overstating would let a redeemer withdraw stake that no longer exists.
func (k Keeper) ApplyValidatorSlash(ctx sdk.Context, valAddr string, fraction sdk.Dec) error {
	if fraction.IsNil() || !fraction.IsPositive() {
		return nil
	}
	if fraction.GT(sdk.OneDec()) {
		fraction = sdk.OneDec()
	}

	bond := k.GetValidatorBond(ctx, valAddr)
	if bond == 0 {
		return nil
	}

	loss := sdk.NewDecFromInt(math.NewIntFromUint64(bond)).Mul(fraction).Ceil().TruncateInt().Uint64()
	if loss > bond {
		loss = bond
	}
	if loss == 0 {
		return nil
	}

	k.setValidatorBond(ctx, valAddr, bond-loss)

	ps := k.GetPoolState(ctx)
	if ps.TotalBondedUmc > loss {
		ps.TotalBondedUmc -= loss
	} else {
		ps.TotalBondedUmc = 0
	}
	ps.CumulativeSlashedUmc += loss
	k.SetPoolState(ctx, ps)

	k.Logger(ctx).Info(
		"liquid staking pool slashed",
		"validator", valAddr,
		"fraction", fraction.String(),
		"loss_umc", loss,
		"exchange_rate", k.ExchangeRate(ctx).String(),
	)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeSlashApplied,
		sdk.NewAttribute(types.AttributeKeyValidator, valAddr),
		sdk.NewAttribute(types.AttributeKeySlashFraction, fraction.String()),
		sdk.NewAttribute(types.AttributeKeyAmountUmc, strconv.FormatUint(loss, 10)),
		sdk.NewAttribute(types.AttributeKeyExchangeRate, k.ExchangeRate(ctx).String()),
	))

	return nil
}
