package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"mcchain/x/tokenomics/types"
)

// DripIntervalBlocks defines the staking-security drip cadence (blocks).
// A drip is executed every 100 blocks (~6.7 min @ 4s block time).
const DripIntervalBlocks int64 = 100

// DripRatioBps is the annualized drip rate as a fraction of total staked MC (5%).
const DripRatioBps uint32 = 500

// BlocksPerYear assumes a 4s block time (see phonenode params: ~12h @ 4s).
const BlocksPerYear int64 = 7_884_000

// IntervalsPerYear = BlocksPerYear / DripIntervalBlocks.
const IntervalsPerYear int64 = BlocksPerYear / DripIntervalBlocks

// DripWithRenewal drips the staking-security incentive to validators/delegators.
//
// Mechanism (whitepaper §staking-security):
//
//	D_t = min( 5% × Staked ,  balance / (IntervalsPerYear × 12yr) )
//
// i.e. target 5% of total staked per year, but never faster than the rate that
// would exhaust the source within the 12-year floor.
//
// Source selection — two-address physical separation (finalized 2026-08):
//   - Pool A = staking_security (1.5e8 MC, code-unspendable) is used first.
//   - If A is exhausted before the 12-year floor, the protocol treasury
//     (B, the 6th address) continues the drip at the renewal floor APR (1–2%).
func (k Keeper) DripWithRenewal(ctx sdk.Context) error {
	staked := k.totalStaked(ctx)
	// Target per-interval drip = 5% of staked, amortized over intervals/year.
	target := staked.MulRaw(int64(DripRatioBps)).QuoRaw(10000).QuoRaw(IntervalsPerYear)

	// Pool A (staking_security) first.
	aAddr := types.StakingSecurityPoolAddress()
	aBal := k.bankKeeper.GetBalance(ctx, aAddr, types.DefaultDenom).Amount
	if aBal.IsPositive() {
		return k.dripFrom(ctx, aBal, target, types.StakingSecurityPoolName)
	}

	// Pool A exhausted → renew from protocol treasury (B) at renewal floor APR.
	bAddr := types.ProtocolTreasuryAddress()
	bBal := k.bankKeeper.GetBalance(ctx, bAddr, types.DefaultDenom).Amount
	if bBal.IsPositive() {
		renewal := staked.MulRaw(int64(types.RenewalFloorAPRBps)).
			QuoRaw(10000).QuoRaw(IntervalsPerYear)
		if renewal.GT(target) {
			target = renewal
		}
		return k.dripFrom(ctx, bBal, target, types.ProtocolTreasuryPoolName)
	}
	return nil
}

// totalStaked returns the total bonded (staked) MC via the bonded pool module
// account balance — no staking keeper dependency required.
func (k Keeper) totalStaked(ctx sdk.Context) sdk.Int {
	bonded := authtypes.NewModuleAddress(stakingtypes.BondedPoolName)
	return k.bankKeeper.GetBalance(ctx, bonded, types.DefaultDenom).Amount
}

// dripFrom releases `drip` from source pool `srcName` to the fee_collector,
// where the distribution module allocates it to validators/delegators by stake.
// The drip is capped so the source cannot be exhausted before the 12-year floor.
func (k Keeper) dripFrom(ctx sdk.Context, bal, target sdk.Int, srcName string) error {
	// Floor cap: at most balance / (intervals-per-year × 12 years), guaranteeing
	// the 12-year drip floor regardless of staked amount.
	floorCap := bal.QuoRaw(IntervalsPerYear * int64(types.DripFloorYears))
	drip := target
	if drip.GT(floorCap) {
		drip = floorCap
	}
	if drip.IsZero() {
		return nil
	}

	coins := sdk.NewCoins(sdk.NewCoin(types.DefaultDenom, drip))
	if err := k.bankKeeper.SendCoinsFromModuleToModule(
		ctx, srcName, authtypes.FeeCollectorName, coins,
	); err != nil {
		k.Logger(ctx).Error("tokenomics: drip failed",
			"source", srcName, "amount", drip.String(), "err", err.Error())
		return fmt.Errorf("tokenomics drip from %s: %w", srcName, err)
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("tokenomics.SecurityDripped",
			sdk.NewAttribute("amount", drip.String()),
			sdk.NewAttribute("ratio_bps", fmt.Sprintf("%d", DripRatioBps)),
			sdk.NewAttribute("source", srcName),
			sdk.NewAttribute("destination", "fee_collector"),
		),
	)
	k.Logger(ctx).Info("tokenomics: security pool dripped to fee_collector",
		"amount_umc", drip.String(), "source", srcName)
	return nil
}
