package keeper

import (
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/dex/types"
)

// ClampFeeRateBps forces a fee rate into the valid [0, MaxFeeRateBps] range.
//
// `10000 - feeRateBps` is evaluated on a uint32: any rate above 100% wraps
// around and inflates the fee factor to ~4.29e9, which would let a single swap
// drain the pool. Params validation and message validation both reject such a
// rate, but the AMM primitives are also reachable with values already stored in
// pool state, so the clamp is enforced here as the last line of defence.
func ClampFeeRateBps(feeRateBps uint32) uint32 {
	if feeRateBps > types.MaxFeeRateBps {
		return types.MaxFeeRateBps
	}
	return feeRateBps
}

// CalcSwapOutput computes the output amount for a constant-product AMM swap.
//
// Formula: amountOut = (amountIn * (10000 - feeRateBps) * reserveOut) /
//
//	(reserveIn * 10000 + amountIn * (10000 - feeRateBps))
//
// This implements x*y=k with fee deducted before output calculation.
// All operations use sdk.Int (big.Int) for exact integer arithmetic.
//
// Degenerate inputs (nil, non-positive reserves or input) return zero instead
// of dividing by zero: a panic inside DeliverTx would abort the whole block, so
// every division here is guarded and the caller is left on its error path.
func CalcSwapOutput(reserveIn, reserveOut, amountIn sdk.Int, feeRateBps uint32) sdk.Int {
	if reserveIn.IsNil() || reserveOut.IsNil() || amountIn.IsNil() {
		return sdk.ZeroInt()
	}
	if !reserveIn.IsPositive() || !reserveOut.IsPositive() || !amountIn.IsPositive() {
		return sdk.ZeroInt()
	}

	feeFactor := sdk.NewInt(int64(types.MaxFeeRateBps - ClampFeeRateBps(feeRateBps)))
	effective := amountIn.Mul(feeFactor)

	denominator := reserveIn.MulRaw(int64(types.MaxFeeRateBps)).Add(effective)
	if !denominator.IsPositive() {
		return sdk.ZeroInt()
	}

	out := effective.Mul(reserveOut).Quo(denominator)

	// Defence in depth: the constant-product formula already guarantees
	// out < reserveOut for any fee factor <= 10000, but never let the reserve
	// be emptied even if that invariant is ever violated upstream.
	if out.GTE(reserveOut) {
		return reserveOut.SubRaw(1)
	}
	return out
}

// CalcSwapFees splits a swap's gross fee into the LP share (which stays inside
// the pool reserve) and the non-LP share (burn + treasury, which is extracted
// from the reserve and routed by ProcessSwapFee).
func CalcSwapFees(amountIn sdk.Int, feeRateBps uint32) (feeTotal, nonLPFee sdk.Int) {
	if amountIn.IsNil() || !amountIn.IsPositive() {
		return sdk.ZeroInt(), sdk.ZeroInt()
	}
	rate := int64(ClampFeeRateBps(feeRateBps))
	feeTotal = amountIn.MulRaw(rate).QuoRaw(int64(types.MaxFeeRateBps))
	nonLPFee = feeTotal.MulRaw(nonLPFeeBps).QuoRaw(int64(types.MaxFeeRateBps))
	return feeTotal, nonLPFee
}

// CalcSwapOutputWithPoolFee is the single source of truth for pool pricing.
//
// The executed swap adds `amountIn - nonLPFee` to the reserve (the LP fee share
// stays in, the burn share is extracted), so the quote must be computed exactly
// the same way. Both the state-changing SwapExactIn handler and the read-only
// EstimateSwap query route through this helper, which guarantees a quote can
// never diverge from execution.
func CalcSwapOutputWithPoolFee(reserveIn, reserveOut, amountIn sdk.Int, feeRateBps uint32) (amountOut, nonLPFee sdk.Int) {
	_, nonLPFee = CalcSwapFees(amountIn, feeRateBps)
	effectiveIn := amountIn.Sub(nonLPFee)
	return CalcSwapOutput(reserveIn, reserveOut, effectiveIn, 0), nonLPFee
}

// CalcAddLiquidity computes LP tokens to mint when adding liquidity.
//
// If totalLP is zero (initial deposit): lpMinted = sqrt(addedA * addedB)
// Otherwise: lpMinted = min(addedA/reserveA, addedB/reserveB) * totalLP
// Excess assets are refunded.
//
// Returns (0, 0, 0) for any degenerate input — empty reserves against a live LP
// supply, non-positive deposits, or a deposit so small that either the minted
// share or one of the two legs rounds down to zero. The caller rejects a zero
// mint with ErrInsufficientLiquidity. Rejecting a zero leg matters: a coin of
// amount 0 is silently dropped by sdk.NewCoins, so allowing it would mint LP
// against a one-sided (partially free) deposit.
func CalcAddLiquidity(reserveA, reserveB, addedA, addedB, totalLP sdk.Int) (lpMinted sdk.Int, actualA, actualB sdk.Int) {
	zero := sdk.ZeroInt()
	if reserveA.IsNil() || reserveB.IsNil() || addedA.IsNil() || addedB.IsNil() || totalLP.IsNil() {
		return zero, zero, zero
	}
	if !addedA.IsPositive() || !addedB.IsPositive() {
		return zero, zero, zero
	}

	if !totalLP.IsPositive() {
		lpMinted = sdk.NewIntFromBigInt(integerSqrt(addedA.Mul(addedB).BigInt()))
		return lpMinted, addedA, addedB
	}

	// A live LP supply against an empty reserve can only come from corrupted
	// state; refuse to price against it rather than dividing by zero.
	if !reserveA.IsPositive() || !reserveB.IsPositive() {
		return zero, zero, zero
	}

	shareA := addedA.Mul(totalLP).Quo(reserveA)
	shareB := addedB.Mul(totalLP).Quo(reserveB)
	if !shareA.IsPositive() || !shareB.IsPositive() {
		return zero, zero, zero
	}

	if shareA.LT(shareB) {
		lpMinted = shareA
		actualA = addedA
		actualB = addedB.Mul(shareA).Quo(shareB)
	} else {
		lpMinted = shareB
		actualA = addedA.Mul(shareB).Quo(shareA)
		actualB = addedB
	}

	if !actualA.IsPositive() || !actualB.IsPositive() {
		return zero, zero, zero
	}
	return lpMinted, actualA, actualB
}

// CalcRemoveLiquidity computes asset amounts returned when removing liquidity.
//
// amountA = reserveA * lpAmount / totalLP
// amountB = reserveB * lpAmount / totalLP
//
// totalLP == 0 returns zero instead of dividing by zero, and lpAmount is capped
// at totalLP so the computed payout can never exceed the pool's reserves even
// if a caller forgets to bound-check the burn amount.
func CalcRemoveLiquidity(reserveA, reserveB, lpAmount, totalLP sdk.Int) (amountA, amountB sdk.Int) {
	zero := sdk.ZeroInt()
	if reserveA.IsNil() || reserveB.IsNil() || lpAmount.IsNil() || totalLP.IsNil() {
		return zero, zero
	}
	if !lpAmount.IsPositive() || !totalLP.IsPositive() {
		return zero, zero
	}
	if lpAmount.GT(totalLP) {
		lpAmount = totalLP
	}

	amountA, amountB = zero, zero
	if reserveA.IsPositive() {
		amountA = reserveA.Mul(lpAmount).Quo(totalLP)
	}
	if reserveB.IsPositive() {
		amountB = reserveB.Mul(lpAmount).Quo(totalLP)
	}
	return amountA, amountB
}

// integerSqrt returns floor(sqrt(n)) exactly.
//
// It delegates to big.Int.Sqrt, which is an exact floor square root, replacing
// a hand-rolled Newton iteration whose termination condition could settle one
// unit off for some inputs. Determinism across nodes is a consensus
// requirement, so the standard-library implementation is preferred.
func integerSqrt(n *big.Int) *big.Int {
	if n == nil || n.Sign() <= 0 {
		return big.NewInt(0)
	}
	return new(big.Int).Sqrt(n)
}
