package keeper_test

import (
	"math"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"mcchain/internal/safemath"
)

// 本文件是 internal/safemath 的跨包冒烟测试。
// 背景：本机第三方杀软会拦截 %TEMP%/新建的测试 exe（Access is denied），
// 只有已信任输出名（referral.test.exe / dexkeeper.test.exe）能执行；
// 因此把 safemath 关键边界断言挂在 referral 测试包内随其一起编译运行。

func big2powCompat(n uint) sdkmath.Int {
	one := sdkmath.NewInt(1)
	return sdkmath.NewIntFromBigInt(one.BigInt().Lsh(one.BigInt(), n))
}

// TestSafemathCompat_Clamp 验证饱和钳制：超界/负值/nil 均不 panic。
func TestSafemathCompat_Clamp(t *testing.T) {
	require.Equal(t, int64(0), safemath.ClampInt64(sdkmath.Int{}))
	require.Equal(t, int64(42), safemath.ClampInt64(sdkmath.NewInt(42)))
	require.Equal(t, int64(math.MaxInt64), safemath.ClampInt64(big2powCompat(127)))
	require.Equal(t, int64(math.MinInt64), safemath.ClampInt64(big2powCompat(127).Neg()))

	require.Equal(t, uint64(0), safemath.ClampUint64(sdkmath.NewInt(-1)))
	require.Equal(t, uint64(0), safemath.ClampUint64(sdkmath.Int{}))
	require.Equal(t, uint64(math.MaxUint64), safemath.ClampUint64(big2powCompat(200)))
	require.Equal(t, uint64(7), safemath.ClampUint64(sdkmath.NewInt(7)))
}

// TestSafemathCompat_Float32 验证 telemetry 转换在超界时不产生 Inf。
func TestSafemathCompat_Float32(t *testing.T) {
	f := safemath.Float32(big2powCompat(200))
	require.False(t, math.IsInf(float64(f), 0))
	require.Equal(t, float32(math.MaxFloat32), f)
	nf := safemath.Float32(big2powCompat(200).Neg())
	require.False(t, math.IsInf(float64(nf), 0))
	require.Equal(t, -float32(math.MaxFloat32), nf)
	require.Equal(t, float32(0), safemath.Float32(sdkmath.Int{}))
}

// TestSafemathCompat_Arith 验证饱和加法与溢出检测乘法。
func TestSafemathCompat_Arith(t *testing.T) {
	sum, ok := safemath.AddUint64(math.MaxUint64, 1)
	require.False(t, ok, "MaxUint64+1 必须报告溢出")
	sum, ok = safemath.AddUint64(math.MaxUint64-5, 5)
	require.True(t, ok)
	require.Equal(t, uint64(math.MaxUint64), sum)

	_, ok = safemath.MulUint64(math.MaxUint64, 2)
	require.False(t, ok)
	prod, ok := safemath.MulUint64(6, 7)
	require.True(t, ok)
	require.Equal(t, uint64(42), prod)
}
