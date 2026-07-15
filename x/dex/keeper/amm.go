package keeper

import (
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// CalcSwapOutput computes the output amount for a constant-product AMM swap.
//
// Formula: amountOut = (amountIn * (10000 - feeRateBps) * reserveOut) /
//
//	(reserveIn * 10000 + amountIn * (10000 - feeRateBps))
//
// This implements x*y=k with fee deducted before output calculation.
// All operations use sdk.Int (big.Int) for exact integer arithmetic.
func CalcSwapOutput(reserveIn, reserveOut, amountIn sdk.Int, feeRateBps uint32) sdk.Int {
	feeFactor := sdk.NewInt(int64(10000 - feeRateBps))
	numerator := amountIn.Mul(feeFactor).Mul(reserveOut)
	denominator := reserveIn.MulRaw(10000).Add(amountIn.Mul(feeFactor))
	return numerator.Quo(denominator)
}

// CalcAddLiquidity computes LP tokens to mint when adding liquidity.
//
// If totalLP is zero (initial deposit): lpMinted = sqrt(addedA * addedB)
// Otherwise: lpMinted = min(addedA/reserveA, addedB/reserveB) * totalLP
// Excess assets are refunded.
func CalcAddLiquidity(reserveA, reserveB, addedA, addedB, totalLP sdk.Int) (lpMinted sdk.Int, actualA, actualB sdk.Int) {
	if totalLP.IsZero() {
		lpMinted = sdk.NewIntFromBigInt(approxSqrt(addedA.Mul(addedB).BigInt()))
		return lpMinted, addedA, addedB
	}

	shareA := addedA.Mul(totalLP).Quo(reserveA)
	shareB := addedB.Mul(totalLP).Quo(reserveB)

	if shareA.LT(shareB) {
		lpMinted = shareA
		actualB = addedB.Mul(shareA).Quo(shareB)
		actualA = addedA
	} else {
		lpMinted = shareB
		actualA = addedA.Mul(shareB).Quo(shareA)
		actualB = addedB
	}
	return
}

// CalcRemoveLiquidity computes asset amounts returned when removing liquidity.
//
// amountA = reserveA * lpAmount / totalLP
// amountB = reserveB * lpAmount / totalLP
func CalcRemoveLiquidity(reserveA, reserveB, lpAmount, totalLP sdk.Int) (amountA, amountB sdk.Int) {
	amountA = reserveA.Mul(lpAmount).Quo(totalLP)
	amountB = reserveB.Mul(lpAmount).Quo(totalLP)
	return
}

// approxSqrt computes the integer square root using Newton's method.
func approxSqrt(n *big.Int) *big.Int {
	if n.Sign() <= 0 {
		return big.NewInt(0)
	}

	x := new(big.Int).Set(n)
	// Initial guess: 2^((bitlen+1)/2)
	bitLen := n.BitLen()
	shift := uint(bitLen+1) / 2
	x.Lsh(big.NewInt(1), shift)

	// Newton's method: x = (x + n/x) / 2
	for {
		quotient := new(big.Int).Div(n, x)
		next := new(big.Int).Add(x, quotient)
		next.Rsh(next, 1)

		diff := new(big.Int).Sub(x, next)
		diff.Abs(diff)
		if diff.Cmp(big.NewInt(1)) <= 0 {
			break
		}
		x = next
	}

	// Ensure x*x <= n (we may overshoot by 1)
	sq := new(big.Int).Mul(x, x)
	if sq.Cmp(n) > 0 {
		x.Sub(x, big.NewInt(1))
	}

	return x
}
