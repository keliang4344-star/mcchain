package safemath

import (
	"math"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/stretchr/testify/require"
)

func big2pow(n uint) sdkmath.Int {
	one := sdkmath.NewInt(1)
	return sdkmath.NewIntFromBigInt(one.BigInt().Lsh(one.BigInt(), n))
}

// TestClampInt64 覆盖正/负/零/超界/nil 五种情形。
func TestClampInt64(t *testing.T) {
	require.Equal(t, int64(0), ClampInt64(sdkmath.Int{}))            // nil
	require.Equal(t, int64(0), ClampInt64(sdkmath.ZeroInt()))        // 0
	require.Equal(t, int64(42), ClampInt64(sdkmath.NewInt(42)))      // 正常
	require.Equal(t, int64(-7), ClampInt64(sdkmath.NewInt(-7)))      // 负数
	require.Equal(t, int64(math.MaxInt64), ClampInt64(big2pow(127))) // 正向超界 → 饱和
	require.Equal(t, int64(math.MinInt64), ClampInt64(big2pow(127).Neg())) // 负向超界 → 饱和
	require.Equal(t, int64(math.MaxInt64), ClampInt64(big2pow(63)))  // 恰在界外 1 → 饱和
}

// TestClampUint64 覆盖负值映射为 0、超界饱和、nil 为 0。
func TestClampUint64(t *testing.T) {
	require.Equal(t, uint64(0), ClampUint64(sdkmath.Int{}))
	require.Equal(t, uint64(0), ClampUint64(sdkmath.NewInt(-1)))
	require.Equal(t, uint64(0), ClampUint64(sdkmath.ZeroInt()))
	require.Equal(t, uint64(7), ClampUint64(sdkmath.NewInt(7)))
	require.Equal(t, uint64(math.MaxUint64), ClampUint64(big2pow(64)))
	require.Equal(t, uint64(math.MaxUint64), ClampUint64(big2pow(255)))
}

// TestFloat32 验证超出 float32 值域时饱和到 MaxFloat32 而非 Inf。
func TestFloat32(t *testing.T) {
	require.Equal(t, float32(0), Float32(sdkmath.Int{}))
	require.Equal(t, float32(1.5), Float32(sdkmath.NewIntWithDecimal(15, 1)))
	// 2^200 远超 float32 最大值（~3.4e38）→ 必须饱和，不得为 Inf。
	f := Float32(big2pow(200))
	require.False(t, math.IsInf(float64(f), 0), "Float32 不得产生 Inf")
	require.Equal(t, float32(math.MaxFloat32), f)
	// 负向同样饱和。
	nf := Float32(big2pow(200).Neg())
	require.False(t, math.IsInf(float64(nf), 0))
	require.Equal(t, -float32(math.MaxFloat32), nf)
}

// TestAddUint64 验证加法溢出检测。
func TestAddUint64(t *testing.T) {
	sum, ok := AddUint64(1, 2)
	require.True(t, ok)
	require.Equal(t, uint64(3), sum)

	_, ok = AddUint64(math.MaxUint64, 1)
	require.False(t, ok, "MaxUint64+1 必须报告溢出")

	sum, ok = AddUint64(math.MaxUint64-5, 5)
	require.True(t, ok)
	require.Equal(t, uint64(math.MaxUint64), sum)
}

// TestMulUint64 验证乘法溢出检测与零值行为。
func TestMulUint64(t *testing.T) {
	prod, ok := MulUint64(6, 7)
	require.True(t, ok)
	require.Equal(t, uint64(42), prod)

	_, ok = MulUint64(math.MaxUint64, 2)
	require.False(t, ok)

	// 零乘任何数不溢出。
	_, ok = MulUint64(0, math.MaxUint64)
	require.True(t, ok)
}
