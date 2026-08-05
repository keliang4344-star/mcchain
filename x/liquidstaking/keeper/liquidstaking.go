package keeper

import (
	"fmt"
	"strconv"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"mcchain/x/liquidstaking/types"
)

// maxDelegationsScanned bounds the reward-compounding sweep so BeginBlock work
// stays predictable.
const maxDelegationsScanned uint16 = 200

// ---------------------------------------------------------------------------
// Exchange rate
// ---------------------------------------------------------------------------

// ExchangeRate returns how much umc one ulmc share is worth, as a decimal.
// An empty pool is defined as 1.0 so the first depositor mints 1:1.
func (k Keeper) ExchangeRate(ctx sdk.Context) sdk.Dec {
	ps := k.GetPoolState(ctx)
	if ps.TotalSharesUlmc == 0 || ps.TotalBondedUmc == 0 {
		return sdk.OneDec()
	}
	return sdk.NewDecFromInt(math.NewIntFromUint64(ps.TotalBondedUmc)).
		Quo(sdk.NewDecFromInt(math.NewIntFromUint64(ps.TotalSharesUlmc)))
}

// sharesForStake converts an incoming umc deposit into ulmc shares using the
// pre-deposit pool state (standard share-pool accounting).
func sharesForStake(ps types.PoolState, amountUmc uint64) uint64 {
	if ps.TotalSharesUlmc == 0 || ps.TotalBondedUmc == 0 {
		return amountUmc
	}
	shares := math.NewIntFromUint64(amountUmc).
		Mul(math.NewIntFromUint64(ps.TotalSharesUlmc)).
		Quo(math.NewIntFromUint64(ps.TotalBondedUmc))
	return shares.Uint64()
}

// umcForShares converts ulmc shares back into umc at the current rate.
func umcForShares(ps types.PoolState, shares uint64) uint64 {
	if ps.TotalSharesUlmc == 0 {
		return 0
	}
	amt := math.NewIntFromUint64(shares).
		Mul(math.NewIntFromUint64(ps.TotalBondedUmc)).
		Quo(math.NewIntFromUint64(ps.TotalSharesUlmc))
	return amt.Uint64()
}

// ---------------------------------------------------------------------------
// Stake
// ---------------------------------------------------------------------------

// LiquidStake bonds `amountUmc` on behalf of `delegator` through the module
// account and mints the corresponding ulmc receipt to the delegator.
//
// The MC never leaves the protocol: it is transferred to the module account and
// immediately delegated to the chosen validator, so the stake keeps securing
// consensus while the delegator holds a transferable claim on it.
func (k Keeper) LiquidStake(ctx sdk.Context, delegator sdk.AccAddress, valAddr sdk.ValAddress, amountUmc uint64) (uint64, error) {
	params := k.GetParams(ctx)
	if !params.Enabled {
		return 0, types.ErrModuleDisabled
	}
	if delegator.Empty() {
		return 0, types.ErrInvalidAddress
	}
	if amountUmc < params.MinStakeUmc {
		return 0, types.ErrBelowMinimumStake.Wrapf("got %d umc, minimum %d umc", amountUmc, params.MinStakeUmc)
	}

	validator, found := k.stakingKpr.GetValidator(ctx, valAddr)
	if !found {
		return 0, types.ErrValidatorNotFound.Wrap(valAddr.String())
	}
	if validator.IsJailed() {
		return 0, types.ErrValidatorJailed.Wrap(valAddr.String())
	}

	ps := k.GetPoolState(ctx)
	// A fully slashed pool still carries share supply with zero backing. Letting
	// a new depositor in at that point would mint them shares alongside worthless
	// ones and hand part of their principal to the wiped-out holders. Deposits
	// stay closed until governance resolves the pool.
	if ps.TotalSharesUlmc > 0 && ps.TotalBondedUmc == 0 {
		return 0, types.ErrPoolWipedOut
	}
	if err := k.checkValidatorCap(ctx, params, ps, valAddr.String(), amountUmc); err != nil {
		return 0, err
	}

	shares := sharesForStake(ps, amountUmc)
	if shares == 0 {
		return 0, types.ErrDustRedemption.Wrap("stake rounds down to zero shares")
	}

	stakeCoins := sdk.NewCoins(sdk.NewCoin(types.BondDenom, math.NewIntFromUint64(amountUmc)))
	if err := k.bankKpr.SendCoinsFromAccountToModule(ctx, delegator, types.ModuleName, stakeCoins); err != nil {
		return 0, fmt.Errorf("liquidstaking: escrow stake: %w", err)
	}

	moduleAddr := k.ModuleAddress()
	if _, err := k.stakingKpr.Delegate(
		ctx, moduleAddr, math.NewIntFromUint64(amountUmc), stakingtypes.Unbonded, validator, true,
	); err != nil {
		return 0, fmt.Errorf("liquidstaking: delegate: %w", err)
	}

	receipt := sdk.NewCoins(sdk.NewCoin(types.LiquidBondDenom, math.NewIntFromUint64(shares)))
	if err := k.bankKpr.MintCoins(ctx, types.ModuleName, receipt); err != nil {
		return 0, fmt.Errorf("liquidstaking: mint receipt: %w", err)
	}
	if err := k.bankKpr.SendCoinsFromModuleToAccount(ctx, types.ModuleName, delegator, receipt); err != nil {
		return 0, fmt.Errorf("liquidstaking: deliver receipt: %w", err)
	}

	ps.TotalBondedUmc += amountUmc
	ps.TotalSharesUlmc += shares
	k.SetPoolState(ctx, ps)
	telemetry.IncrCounter(float32(amountUmc), "liquidstaking", "staked_umc")
	k.setValidatorBond(ctx, valAddr.String(), k.GetValidatorBond(ctx, valAddr.String())+amountUmc)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeLiquidStake,
		sdk.NewAttribute(types.AttributeKeyDelegator, delegator.String()),
		sdk.NewAttribute(types.AttributeKeyValidator, valAddr.String()),
		sdk.NewAttribute(types.AttributeKeyAmountUmc, strconv.FormatUint(amountUmc, 10)),
		sdk.NewAttribute(types.AttributeKeySharesUlmc, strconv.FormatUint(shares, 10)),
		sdk.NewAttribute(types.AttributeKeyExchangeRate, k.ExchangeRate(ctx).String()),
	))

	return shares, nil
}

// checkValidatorCap enforces the per-validator concentration limit.
//
// The cap is only meaningful once the pool is spread across enough validators
// to satisfy it: with a 20% cap at least five validators are required. Below
// that threshold the check is skipped, otherwise the very first delegation
// could never be made.
func (k Keeper) checkValidatorCap(ctx sdk.Context, params types.Params, ps types.PoolState, valAddr string, amountUmc uint64) error {
	bonds := k.AllValidatorBonds(ctx)
	distinct := len(bonds)
	if k.GetValidatorBond(ctx, valAddr) == 0 {
		distinct++
	}

	required := int(10_000 / params.MaxValidatorShareBps)
	if 10_000%params.MaxValidatorShareBps != 0 {
		required++
	}
	if distinct < required {
		return nil
	}

	newValidatorBond := math.NewIntFromUint64(k.GetValidatorBond(ctx, valAddr) + amountUmc)
	newTotal := math.NewIntFromUint64(ps.TotalBondedUmc + amountUmc)
	limit := newTotal.MulRaw(int64(params.MaxValidatorShareBps)).QuoRaw(10_000)
	if newValidatorBond.GT(limit) {
		return types.ErrValidatorCapExceed.Wrapf(
			"validator %s would hold %s umc, cap is %s umc (%d bps)",
			valAddr, newValidatorBond, limit, params.MaxValidatorShareBps,
		)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Unstake
// ---------------------------------------------------------------------------

// LiquidUnstake burns `sharesUlmc` from the delegator, undelegates the matching
// umc from `valAddr` and writes a redemption receipt that matures when the
// staking unbonding period completes.
//
// Passing an empty validator selects the validator this module has the largest
// bond with, which keeps the pool naturally rebalanced on the way out.
func (k Keeper) LiquidUnstake(ctx sdk.Context, delegator sdk.AccAddress, valAddr string, sharesUlmc uint64) (types.UnbondingEntry, error) {
	params := k.GetParams(ctx)
	if delegator.Empty() {
		return types.UnbondingEntry{}, types.ErrInvalidAddress
	}
	if sharesUlmc == 0 {
		return types.UnbondingEntry{}, types.ErrInsufficientShares.Wrap("zero shares")
	}

	ps := k.GetPoolState(ctx)
	if ps.TotalSharesUlmc == 0 || ps.TotalBondedUmc == 0 {
		return types.UnbondingEntry{}, types.ErrEmptyPool
	}

	receipt := sdk.NewCoin(types.LiquidBondDenom, math.NewIntFromUint64(sharesUlmc))
	if !k.bankKpr.HasBalance(ctx, delegator, receipt) {
		return types.UnbondingEntry{}, types.ErrInsufficientShares.Wrapf(
			"%s holds less than %s", delegator.String(), receipt.String())
	}

	amountUmc := umcForShares(ps, sharesUlmc)
	if amountUmc == 0 {
		return types.UnbondingEntry{}, types.ErrDustRedemption
	}

	if valAddr == "" {
		selected, ok := k.largestBondValidator(ctx)
		if !ok {
			return types.UnbondingEntry{}, types.ErrEmptyPool.Wrap("no tracked validator bond")
		}
		valAddr = selected
	}
	bonded := k.GetValidatorBond(ctx, valAddr)
	if bonded < amountUmc {
		return types.UnbondingEntry{}, types.ErrInsufficientShares.Wrapf(
			"validator %s holds %d umc for this module, need %d umc", valAddr, bonded, amountUmc)
	}

	valAcc, err := sdk.ValAddressFromBech32(valAddr)
	if err != nil {
		return types.UnbondingEntry{}, types.ErrInvalidAddress.Wrap(valAddr)
	}

	// Burn the receipt first so the share supply can never exceed the claim.
	receiptCoins := sdk.NewCoins(receipt)
	if err := k.bankKpr.SendCoinsFromAccountToModule(ctx, delegator, types.ModuleName, receiptCoins); err != nil {
		return types.UnbondingEntry{}, fmt.Errorf("liquidstaking: collect receipt: %w", err)
	}
	if err := k.bankKpr.BurnCoins(ctx, types.ModuleName, receiptCoins); err != nil {
		return types.UnbondingEntry{}, fmt.Errorf("liquidstaking: burn receipt: %w", err)
	}

	moduleAddr := k.ModuleAddress()
	delShares, err := k.stakingKpr.ValidateUnbondAmount(ctx, moduleAddr, valAcc, math.NewIntFromUint64(amountUmc))
	if err != nil {
		return types.UnbondingEntry{}, fmt.Errorf("liquidstaking: resolve unbond shares: %w", err)
	}
	completion, err := k.stakingKpr.Undelegate(ctx, moduleAddr, valAcc, delShares)
	if err != nil {
		return types.UnbondingEntry{}, fmt.Errorf("liquidstaking: undelegate: %w", err)
	}

	id := k.NextUnbondingID(ctx)
	entry := types.UnbondingEntry{
		ID:                 id,
		Delegator:          delegator.String(),
		Validator:          valAddr,
		AmountUmc:          amountUmc,
		SharesBurnedUlmc:   sharesUlmc,
		CompletionUnixTime: completion.Unix() + params.UnbondingClaimGraceSeconds,
	}
	k.SetUnbondingEntry(ctx, entry)
	k.setNextUnbondingID(ctx, id+1)

	ps.TotalBondedUmc -= amountUmc
	ps.TotalSharesUlmc -= sharesUlmc
	ps.TotalUnbondingUmc += amountUmc
	k.SetPoolState(ctx, ps)
	k.setValidatorBond(ctx, valAddr, bonded-amountUmc)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeLiquidUnstake,
		sdk.NewAttribute(types.AttributeKeyDelegator, delegator.String()),
		sdk.NewAttribute(types.AttributeKeyValidator, valAddr),
		sdk.NewAttribute(types.AttributeKeyAmountUmc, strconv.FormatUint(amountUmc, 10)),
		sdk.NewAttribute(types.AttributeKeySharesUlmc, strconv.FormatUint(sharesUlmc, 10)),
		sdk.NewAttribute(types.AttributeKeyUnbondingID, strconv.FormatUint(id, 10)),
		sdk.NewAttribute(types.AttributeKeyCompletionAt, strconv.FormatInt(entry.CompletionUnixTime, 10)),
	))

	return entry, nil
}

func (k Keeper) largestBondValidator(ctx sdk.Context) (string, bool) {
	var best string
	var bestAmt uint64
	for _, vb := range k.AllValidatorBonds(ctx) {
		if vb.AmountUmc > bestAmt {
			best, bestAmt = vb.Validator, vb.AmountUmc
		}
	}
	return best, bestAmt > 0
}

// ---------------------------------------------------------------------------
// Claim
// ---------------------------------------------------------------------------

// ClaimMatured releases every matured redemption receipt owned by `delegator`
// and returns the total umc paid out.
func (k Keeper) ClaimMatured(ctx sdk.Context, delegator sdk.AccAddress) (uint64, error) {
	if delegator.Empty() {
		return 0, types.ErrInvalidAddress
	}
	now := ctx.BlockTime().Unix()
	addr := delegator.String()

	var total uint64
	matured := make([]types.UnbondingEntry, 0, 4)
	for _, e := range k.GetDelegatorUnbondings(ctx, addr) {
		if e.Claimed || e.CompletionUnixTime > now {
			continue
		}
		matured = append(matured, e)
		total += e.AmountUmc
	}
	if total == 0 {
		return 0, types.ErrNothingToClaim
	}

	payout := sdk.NewCoins(sdk.NewCoin(types.BondDenom, math.NewIntFromUint64(total)))
	if err := k.bankKpr.SendCoinsFromModuleToAccount(ctx, types.ModuleName, delegator, payout); err != nil {
		return 0, fmt.Errorf("liquidstaking: pay redemption: %w", err)
	}

	store := ctx.KVStore(k.storeKey)
	for _, e := range matured {
		store.Delete(types.UnbondingEntryKey(e.Delegator, e.ID))
	}

	ps := k.GetPoolState(ctx)
	if ps.TotalUnbondingUmc >= total {
		ps.TotalUnbondingUmc -= total
	} else {
		ps.TotalUnbondingUmc = 0
	}
	k.SetPoolState(ctx, ps)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeLiquidClaim,
		sdk.NewAttribute(types.AttributeKeyDelegator, addr),
		sdk.NewAttribute(types.AttributeKeyAmountUmc, strconv.FormatUint(total, 10)),
	))

	return total, nil
}

// ---------------------------------------------------------------------------
// Reward compounding
// ---------------------------------------------------------------------------

// AccrueRewards withdraws the module's staking rewards and re-delegates them to
// the validator that produced them. Share supply is unchanged, so the whole
// reward accrues to existing holders as a rising ulmc/umc exchange rate.
//
// Under MobileChain's zero-inflation design these rewards come from
// transaction fees and the staking-security pool drip, never from new issuance.
func (k Keeper) AccrueRewards(ctx sdk.Context) (uint64, error) {
	ps := k.GetPoolState(ctx)
	if ps.TotalBondedUmc == 0 {
		return 0, nil
	}

	moduleAddr := k.ModuleAddress()
	delegations := k.stakingKpr.GetDelegatorDelegations(ctx, moduleAddr, maxDelegationsScanned)

	var compounded uint64
	for _, del := range delegations {
		valAcc, err := sdk.ValAddressFromBech32(del.ValidatorAddress)
		if err != nil {
			continue
		}
		rewards, err := k.distrKpr.WithdrawDelegationRewards(ctx, moduleAddr, valAcc)
		if err != nil {
			k.Logger(ctx).Debug("withdraw delegation rewards failed", "validator", del.ValidatorAddress, "err", err)
			continue
		}
		amount := rewards.AmountOf(types.BondDenom)
		if !amount.IsPositive() {
			continue
		}
		validator, found := k.stakingKpr.GetValidator(ctx, valAcc)
		if !found || validator.IsJailed() {
			continue
		}
		if _, err := k.stakingKpr.Delegate(
			ctx, moduleAddr, amount, stakingtypes.Unbonded, validator, true,
		); err != nil {
			k.Logger(ctx).Debug("re-delegate rewards failed", "validator", del.ValidatorAddress, "err", err)
			continue
		}
		gained := amount.Uint64()
		compounded += gained
		k.setValidatorBond(ctx, del.ValidatorAddress, k.GetValidatorBond(ctx, del.ValidatorAddress)+gained)
	}

	if compounded == 0 {
		return 0, nil
	}

	ps = k.GetPoolState(ctx)
	ps.TotalBondedUmc += compounded
	ps.CumulativeRewardsUmc += compounded
	k.SetPoolState(ctx, ps)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeRewardsAccrue,
		sdk.NewAttribute(types.AttributeKeyAmountUmc, strconv.FormatUint(compounded, 10)),
		sdk.NewAttribute(types.AttributeKeyExchangeRate, k.ExchangeRate(ctx).String()),
	))

	return compounded, nil
}
