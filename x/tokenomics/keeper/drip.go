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
	params := k.GetParams(ctx)
	staked := k.totalStaked(ctx)
	// Target per-interval drip = DripRatioBps of staked, amortized over intervals/year.
	target := staked.MulRaw(int64(params.DripRatioBps)).QuoRaw(10000).QuoRaw(IntervalsPerYear)

	// Pool A (staking_security) first.
	aAddr := types.StakingSecurityPoolAddress()
	aBal := k.bankKeeper.GetBalance(ctx, aAddr, types.DefaultDenom).Amount
	if aBal.IsPositive() {
		return k.dripFrom(ctx, aBal, target, uint64(params.DripFloorYears), types.StakingSecurityPoolName)
	}

	// Pool A exhausted → renew from protocol treasury (B) inside the renewal APR band.
	bAddr := types.ProtocolTreasuryAddress()
	bBal := k.bankKeeper.GetBalance(ctx, bAddr, types.DefaultDenom).Amount
	if bBal.IsPositive() {
		target = renewalTarget(staked, params)
		return k.dripFrom(ctx, bBal, target, uint64(params.DripFloorYears), types.ProtocolTreasuryPoolName)
	}
	// A、B 双池均耗尽：滴灌静默归零（无活水可放）。记录告警事件，便于运维感知
	// 安全池/国库流动性枯竭（通常意味着企业结算费尚未起量或 staking 规模异常）。
	ctx.EventManager().EmitEvent(
		sdk.NewEvent("tokenomics.DripExhausted",
			sdk.NewAttribute("staked", staked.String()),
			sdk.NewAttribute("target", target.String()),
		),
	)
	k.Logger(ctx).Info("tokenomics: drip source exhausted (staking_security + protocol_treasury both empty); staking security drip paused")
	return nil
}

// renewalTarget computes the per-interval drip target used when Pool A
// (staking_security) is exhausted and the protocol treasury (B) has to carry the
// drip. The whitepaper pins the treasury renewal APR to a 1.00%–2.00% band
// (`RenewalFloorAPRBps` / `RenewalFloorAPRCeilBps`), so the nominal drip rate is
// clamped into that band rather than reused verbatim:
//
//	renewalAPR = clamp(DripRatioBps, RenewalFloorAPRBps, RenewalFloorAPRCeilBps)
//
// Why the clamp: Pool A is a purpose-built allocation whose 5%/yr release is
// intended; the treasury is community/foundation money and must not be bled at
// the same rate once A runs dry. The ceil caps the treasury outlay, while the
// floor keeps a governance-tunable minimum so staking security cannot be
// starved by setting DripRatioBps absurdly low.
//
// Before this fix the renewal branch only *raised* the 5% Pool-A target to the
// 1% floor — a no-op — so `RenewalFloorAPRCeilBps` was dead configuration and
// the treasury silently drained at 5%/yr (DripFloorYears would be breached).
func renewalTarget(staked sdk.Int, p types.Params) sdk.Int {
	apr := int64(p.DripRatioBps)
	if ceil := int64(p.RenewalFloorAPRCeilBps); apr > ceil {
		apr = ceil
	}
	if floor := int64(p.RenewalFloorAPRBps); apr < floor {
		apr = floor
	}
	return staked.MulRaw(apr).QuoRaw(10000).QuoRaw(IntervalsPerYear)
}

// totalStaked returns the total bonded (staked) MC via the bonded pool module
// account balance — no staking keeper dependency required.
func (k Keeper) totalStaked(ctx sdk.Context) sdk.Int {
	bonded := authtypes.NewModuleAddress(stakingtypes.BondedPoolName)
	return k.bankKeeper.GetBalance(ctx, bonded, types.DefaultDenom).Amount
}

// dripFrom releases `drip` from source pool `srcName` to the fee_collector,
// where the distribution module allocates it to validators/delegators by stake.
// The drip is capped so the source cannot be exhausted before the `floorYears` floor.
func (k Keeper) dripFrom(ctx sdk.Context, bal, target sdk.Int, floorYears uint64, srcName string) error {
	// Floor cap: at most balance / (intervals-per-year × floorYears), guaranteeing
	// the drip floor regardless of staked amount.
	floorCap := bal.QuoRaw(IntervalsPerYear * int64(floorYears))
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

	params := k.GetParams(ctx)
	ctx.EventManager().EmitEvent(
		sdk.NewEvent("tokenomics.SecurityDripped",
			sdk.NewAttribute("amount", drip.String()),
			sdk.NewAttribute("ratio_bps", fmt.Sprintf("%d", params.DripRatioBps)),
			sdk.NewAttribute("source", srcName),
			sdk.NewAttribute("destination", "fee_collector"),
		),
	)
	k.Logger(ctx).Info("tokenomics: security pool dripped to fee_collector",
		"amount_umc", drip.String(), "source", srcName)
	return nil
}
