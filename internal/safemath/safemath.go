// Package safemath provides overflow-safe conversions from arbitrary-precision
// Cosmos integers to fixed-width Go numeric types, plus checked fixed-width
// arithmetic helpers.
//
// Background
//
// sdk.Int (cosmossdk.io/math.Int) wraps big.Int and legitimately holds values
// far beyond the int64/uint64 range: token amounts, LP shares and every
// user-supplied amount are 256-bit. Calling Int.Int64() / Int.Uint64() on such
// a value PANICS with "integer out of range". Because most of those values are
// attacker-controlled, a naive conversion inside a message handler is a
// chain-halt vector — a panic during DeliverTx aborts the whole block.
//
// Use these helpers for any conversion whose result is only informational
// (telemetry counters, log fields, event metrics). Never use them where the
// exact value is consensus-critical: there, keep working in sdk.Int.
package safemath

import (
	"math"
	"math/big"

	sdkmath "cosmossdk.io/math"
)

// ClampInt64 converts i to int64, saturating at math.MinInt64 / math.MaxInt64
// instead of panicking. A nil Int yields 0.
func ClampInt64(i sdkmath.Int) int64 {
	if i.IsNil() {
		return 0
	}
	b := i.BigInt()
	if b.IsInt64() {
		return b.Int64()
	}
	if b.Sign() > 0 {
		return math.MaxInt64
	}
	return math.MinInt64
}

// ClampUint64 converts i to uint64, saturating at math.MaxUint64 and mapping
// any negative (or nil) value to 0 instead of panicking.
func ClampUint64(i sdkmath.Int) uint64 {
	if i.IsNil() || i.Sign() <= 0 {
		return 0
	}
	b := i.BigInt()
	if b.IsUint64() {
		return b.Uint64()
	}
	return math.MaxUint64
}

// Float32 converts i to float32 for telemetry purposes, saturating at
// ±math.MaxFloat32 instead of producing ±Inf. Precision loss is expected and
// acceptable: the result is only ever used for metrics, never for consensus.
func Float32(i sdkmath.Int) float32 {
	if i.IsNil() {
		return 0
	}
	f, _ := new(big.Float).SetInt(i.BigInt()).Float32()
	if math.IsInf(float64(f), 1) {
		return math.MaxFloat32
	}
	if math.IsInf(float64(f), -1) {
		return -math.MaxFloat32
	}
	return f
}

// AddUint64 returns a+b and reports whether the addition overflowed.
// On overflow the returned sum is meaningless and must not be used.
func AddUint64(a, b uint64) (uint64, bool) {
	sum := a + b
	if sum < a {
		return 0, false
	}
	return sum, true
}

// MulUint64 returns a*b and reports whether the multiplication overflowed.
func MulUint64(a, b uint64) (uint64, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	product := a * b
	if product/a != b {
		return 0, false
	}
	return product, true
}

// ClampToInt converts i to the platform int, saturating at the platform int
// bounds (MaxInt64 on 64-bit, MaxInt32 on 32-bit) instead of panicking or
// truncating. A nil Int yields 0. Negative values saturate at MinInt.
func ClampToInt(i sdkmath.Int) int {
	v := ClampInt64(i)
	bits := 32 << (^uint(0) >> 63) // 32 on 32-bit, 64 on 64-bit platforms
	if bits == 32 {
		if v > math.MaxInt32 {
			return math.MaxInt32
		}
		if v < math.MinInt32 {
			return math.MinInt32
		}
	}
	return int(v)
}
