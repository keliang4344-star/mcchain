package keeper

import (
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/dex/types"
)

// Fee distribution ratios (in bps of the total fee, 10000 = 100%).
//
// Whitepaper §24: total swap fee is 0.30% of the trade volume.
// 50% of the fee is burned (= 0.15% of trade volume, permanent deflation).
// 50% of the fee stays with LP providers (= 0.15% of trade volume, in pool reserve).
// No protocol treasury share (the treasury is funded separately).
const (
	FeeBurnBps     = 5000 // 50.00% of fee burned (= 0.15% of trade, whitepaper §24 通缩飞轮)
	FeeTreasuryBps = 0    //  0.00% — treasury funded separately (not from swap fees)
	FeeLPBps       = 5000 // 50.00% of fee to LP providers (= 0.15% of trade, stays in pool reserve)
)

// nonLPFeeBps is the portion of the fee NOT going to LP providers.
const nonLPFeeBps = FeeBurnBps + FeeTreasuryBps // 5000 = 50%

// ProcessSwapFee handles fee distribution after a swap has completed.
//
// At this point:
//   - amountIn has already been transferred from the user to the dex module
//   - amountOut has already been transferred from the dex module back to the user
//   - The pool reserves have been updated to: newReserveIn = reserveIn + amountIn - nonLPFee
//     (the LP portion stays in reserve; burn+treasury portions are deducted)
//
// This function performs the actual bank operations (burn + treasury transfer)
// and emits a FeeDistribution event.
//
// Parameters:
//   - denomIn: the input denom of the swap (fee is paid in input tokens)
//   - amountIn: the total input amount including fee
//   - feeRateBps: the pool's fee rate in basis points
func (k Keeper) ProcessSwapFee(
	ctx sdk.Context,
	poolID uint64,
	denomIn string,
	amountIn sdk.Int,
	feeRateBps uint32,
) error {
	// Calculate total fee collected
	feeTotal := amountIn.MulRaw(int64(feeRateBps)).QuoRaw(10000)
	if feeTotal.IsZero() {
		return nil
	}

	burnAmt := feeTotal.MulRaw(FeeBurnBps).QuoRaw(10000)
	treasuryAmt := feeTotal.MulRaw(FeeTreasuryBps).QuoRaw(10000)
	lpAmt := feeTotal.Sub(burnAmt).Sub(treasuryAmt) // remainder ≈ 50% (LP share)

	// Burn 16.67% of the fee (0.05% of trade) from the dex module account
	if burnAmt.IsPositive() {
		burnCoin := sdk.NewCoins(sdk.NewCoin(denomIn, burnAmt))
		if err := k.bankKeeper.BurnCoins(ctx, types.ModuleName, burnCoin); err != nil {
			return err
		}
	}

	// Send 30% to protocol treasury (community module)
	if treasuryAmt.IsPositive() {
		treasuryCoin := sdk.NewCoins(sdk.NewCoin(denomIn, treasuryAmt))
		if err := k.bankKeeper.SendCoinsFromModuleToModule(
			ctx, types.ModuleName, types.CommunityModuleName, treasuryCoin,
		); err != nil {
			return err
		}
	}

	// Emit fee distribution event
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeFeeDistribution,
		sdk.NewAttribute(types.AttrKeyPoolID, strconv.FormatUint(poolID, 10)),
		sdk.NewAttribute(types.AttrKeyFeeAmount, feeTotal.String()),
		sdk.NewAttribute(types.AttrKeyFeeBurned, burnAmt.String()),
		sdk.NewAttribute(types.AttrKeyFeeToLP, lpAmt.String()),
		sdk.NewAttribute(types.AttrKeyFeeToTreasury, treasuryAmt.String()),
		sdk.NewAttribute(types.AttrKeyFeeDenom, denomIn),
	))

	return nil
}
