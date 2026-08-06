package keeper

import (
	"math/big"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"mcchain/x/dex/types"
)

// ============================================================================
// AMM 原语边界回归
//
// 这些用例逐条对应本轮修复的缺陷：任何一条回归都会让链在 DeliverTx 里 panic
// （= 全网停机）或让池子被单笔交易抽干，因此必须长期钉死。
// ============================================================================

// TestCalcSwapOutput_DegenerateInputs 退化输入必须返回 0 而不是除零 panic。
func TestCalcSwapOutput_DegenerateInputs(t *testing.T) {
	one := sdk.NewInt(1)
	cases := []struct {
		name                          string
		reserveIn, reserveOut, amount sdk.Int
	}{
		{"zero reserveIn", sdk.ZeroInt(), one, one},
		{"zero reserveOut", one, sdk.ZeroInt(), one},
		{"zero amountIn", one, one, sdk.ZeroInt()},
		{"negative reserveIn", sdk.NewInt(-5), one, one},
		{"negative amountIn", one, one, sdk.NewInt(-5)},
		{"nil reserveIn", sdk.Int{}, one, one},
		{"nil reserveOut", one, sdk.Int{}, one},
		{"nil amountIn", one, one, sdk.Int{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.NotPanics(t, func() {
				out := CalcSwapOutput(tc.reserveIn, tc.reserveOut, tc.amount, 30)
				require.True(t, out.IsZero(), "degenerate input must yield zero output")
			})
		})
	}
}

// TestCalcSwapOutput_FeeRateOverflowClamped 费率 > 100% 时，`10000 - feeRateBps`
// 在 uint32 上会回绕成约 4.29e9 的费率因子，单笔交易即可抽干池子。必须被钳制。
func TestCalcSwapOutput_FeeRateOverflowClamped(t *testing.T) {
	reserveIn := sdk.NewInt(1_000_000)
	reserveOut := sdk.NewInt(1_000_000)
	amountIn := sdk.NewInt(1_000)

	// 100% 费率 → 有效输入为 0 → 输出为 0
	require.True(t, CalcSwapOutput(reserveIn, reserveOut, amountIn, types.MaxFeeRateBps).IsZero())

	// 超过 100% 的费率一律钳制到 100%，绝不允许回绕
	for _, bad := range []uint32{10001, 20000, 4_294_967_295} {
		out := CalcSwapOutput(reserveIn, reserveOut, amountIn, bad)
		require.True(t, out.IsZero(), "feeRateBps=%d must clamp to 100%%, got out=%s", bad, out)
		require.Equal(t, types.MaxFeeRateBps, int(ClampFeeRateBps(bad)))
	}
}

// TestCalcSwapOutput_NeverDrainsReserve 输出永远严格小于输出侧储备。
func TestCalcSwapOutput_NeverDrainsReserve(t *testing.T) {
	reserveIn := sdk.NewInt(1_000)
	reserveOut := sdk.NewInt(1_000)
	// 用一个天量输入尝试抽干
	huge, _ := sdk.NewIntFromString("1000000000000000000000000000000")
	out := CalcSwapOutput(reserveIn, reserveOut, huge, 0)
	require.True(t, out.LT(reserveOut), "output must stay strictly below reserveOut")
	require.True(t, out.IsPositive())
}

// TestCalcSwapOutput_BigIntNoPanic 超出 int64 的储备/输入不得 panic。
func TestCalcSwapOutput_BigIntNoPanic(t *testing.T) {
	big1, _ := sdk.NewIntFromString("100000000000000000000000000") // 1e26 > int64
	big2, _ := sdk.NewIntFromString("200000000000000000000000000")
	require.NotPanics(t, func() {
		out := CalcSwapOutput(big1, big2, big1, 30)
		require.True(t, out.IsPositive())
	})
}

// TestCalcRemoveLiquidity_Bounds 越界赎回必须被钳制，绝不允许算出超过储备的payout。
func TestCalcRemoveLiquidity_Bounds(t *testing.T) {
	reserveA := sdk.NewInt(1_000_000)
	reserveB := sdk.NewInt(2_000_000)
	totalLP := sdk.NewInt(1_000_000)

	// totalLP = 0 → 返回 0 而不是除零
	a, b := CalcRemoveLiquidity(reserveA, reserveB, sdk.NewInt(100), sdk.ZeroInt())
	require.True(t, a.IsZero() && b.IsZero())

	// lpAmount > totalLP → 钳制为 totalLP，payout 恰为全部储备（不会超出）
	a, b = CalcRemoveLiquidity(reserveA, reserveB, totalLP.MulRaw(5), totalLP)
	require.Equal(t, reserveA, a)
	require.Equal(t, reserveB, b)

	// 正常按比例
	a, b = CalcRemoveLiquidity(reserveA, reserveB, totalLP.QuoRaw(2), totalLP)
	require.Equal(t, int64(500_000), a.Int64())
	require.Equal(t, int64(1_000_000), b.Int64())

	// nil / 负数不 panic
	require.NotPanics(t, func() {
		CalcRemoveLiquidity(sdk.Int{}, reserveB, totalLP, totalLP)
		CalcRemoveLiquidity(reserveA, reserveB, sdk.NewInt(-1), totalLP)
	})
}

// TestCalcAddLiquidity_Degenerate 空储备对活跃 LP 供应、零腿存款等退化场景。
func TestCalcAddLiquidity_Degenerate(t *testing.T) {
	// 首次注入：lp = sqrt(a*b)
	lp, a, b := CalcAddLiquidity(sdk.ZeroInt(), sdk.ZeroInt(),
		sdk.NewInt(100), sdk.NewInt(400), sdk.ZeroInt())
	require.Equal(t, int64(200), lp.Int64()) // sqrt(40000)
	require.Equal(t, int64(100), a.Int64())
	require.Equal(t, int64(400), b.Int64())

	// 活跃 LP 供应 + 空储备（状态损坏）→ 拒绝定价而不是除零
	lp, a, b = CalcAddLiquidity(sdk.ZeroInt(), sdk.NewInt(100),
		sdk.NewInt(10), sdk.NewInt(10), sdk.NewInt(1_000))
	require.True(t, lp.IsZero() && a.IsZero() && b.IsZero())

	// 存款过小导致份额取整为 0 → 不得铸出 0 LP 的「白送」存款
	lp, _, _ = CalcAddLiquidity(sdk.NewInt(1_000_000_000), sdk.NewInt(1_000_000_000),
		sdk.NewInt(1), sdk.NewInt(1), sdk.NewInt(1))
	require.True(t, lp.IsZero())

	// 非正存款
	lp, _, _ = CalcAddLiquidity(sdk.NewInt(100), sdk.NewInt(100),
		sdk.ZeroInt(), sdk.NewInt(100), sdk.NewInt(100))
	require.True(t, lp.IsZero())

	// nil 输入不 panic
	require.NotPanics(t, func() {
		CalcAddLiquidity(sdk.Int{}, sdk.NewInt(1), sdk.NewInt(1), sdk.NewInt(1), sdk.NewInt(1))
	})
}

// TestIntegerSqrt_Exact 平方根必须是精确 floor，且在 uint64 范围之外仍然正确。
//
// 创世池 5e12 umc × 1e11 uusdt = 5e23，远超 uint64（1.8e19）。旧实现先
// .Uint64() 截断再开方，算出 1,000,940,608 而非正确的 707,106,781,186。
func TestIntegerSqrt_Exact(t *testing.T) {
	require.Equal(t, int64(0), integerSqrt(big.NewInt(0)).Int64())
	require.Equal(t, int64(0), integerSqrt(big.NewInt(-9)).Int64())
	require.Equal(t, int64(0), integerSqrt(nil).Int64())
	require.Equal(t, int64(1), integerSqrt(big.NewInt(1)).Int64())
	require.Equal(t, int64(2), integerSqrt(big.NewInt(4)).Int64())
	require.Equal(t, int64(2), integerSqrt(big.NewInt(8)).Int64()) // floor
	require.Equal(t, int64(3), integerSqrt(big.NewInt(9)).Int64())

	// floor 性质：sqrt(n)^2 <= n < (sqrt(n)+1)^2
	for _, v := range []int64{5, 15, 99, 100, 101, 1 << 40, (1 << 40) + 1} {
		n := big.NewInt(v)
		s := integerSqrt(n)
		sq := new(big.Int).Mul(s, s)
		next := new(big.Int).Add(s, big.NewInt(1))
		nextSq := new(big.Int).Mul(next, next)
		require.True(t, sq.Cmp(n) <= 0, "sqrt(%d)^2 must be <= n", v)
		require.True(t, nextSq.Cmp(n) > 0, "(sqrt(%d)+1)^2 must be > n", v)
	}

	// 创世池实际数值
	mc, _ := new(big.Int).SetString(InitialPoolMC, 10)
	usdt, _ := new(big.Int).SetString(InitialPoolUSDT, 10)
	product := new(big.Int).Mul(mc, usdt)
	require.Equal(t, "707106781186", integerSqrt(product).String())
}

// TestInitGenesisPool_LPAmountIsCorrect 创世池 LP 铸造量必须是精确几何平均，
// 且必须在储备已预存的前提下才建池（零通胀守卫）。
func TestInitGenesisPool_LPAmountIsCorrect(t *testing.T) {
	k, ctx, bk := setupDex(t)

	// 未预存储备 → 跳过建池（不得凭空铸造储备）
	k.InitGenesisPool(ctx)
	_, found := k.GetPool(ctx, 1)
	require.False(t, found, "储备未预存时不得建池")

	// 预存储备后建池
	bk.setModuleBalance(types.ModuleName, InitialPoolDenomMC, 5_000_000_000_000)
	bk.setModuleBalance(types.ModuleName, InitialPoolDenomUSDT, 100_000_000_000)
	k.InitGenesisPool(ctx)

	pool, found := k.GetPool(ctx, 1)
	require.True(t, found)
	require.Equal(t, InitialPoolDenomMC, pool.DenomA)
	require.Equal(t, InitialPoolDenomUSDT, pool.DenomB)
	require.Equal(t, InitialPoolMC, pool.ReserveA)
	require.Equal(t, InitialPoolUSDT, pool.ReserveB)
	// sqrt(5e12 × 1e11) = 707,106,781,186（旧的 uint64 截断实现会得到 1,000,940,608）
	require.Equal(t, "707106781186", pool.TotalLp)
	require.NotEqual(t, "1000940608", pool.TotalLp)

	// 幂等
	k.InitGenesisPool(ctx)
	pool2, _ := k.GetPool(ctx, 1)
	require.Equal(t, pool.TotalLp, pool2.TotalLp)
}

// TestQuoteMatchesExecution 报价必须与成交完全一致（同一定价函数）。
//
// 修复前 EstimateSwap 用全额费率定价、SwapExactIn 只扣非 LP 份额，
// 报价系统性低于实际成交，前端滑点保护会围绕错误的中值设置。
func TestQuoteMatchesExecution(t *testing.T) {
	k, ctx, bk := setupDex(t)
	lp := addrOfDex(t)
	trader := addrOfDex(t)

	bk.setBalance(lp, "umc", 10_000_000_000)
	bk.setBalance(lp, "uusdc", 10_000_000_000)
	bk.setBalance(trader, "umc", 10_000_000_000)

	_, err := k.CreatePool(ctx, "umc", "uusdc",
		sdk.NewInt(1_000_000_000), sdk.NewInt(1_000_000_000), 30, lp, 0)
	require.NoError(t, err)

	amountIn := sdk.NewInt(100_000_000)
	res, err := k.EstimateSwap(sdk.WrapSDKContext(ctx), &types.QueryEstimateSwapRequest{
		PoolId:   1,
		DenomIn:  "umc",
		DenomOut: "uusdc",
		AmountIn: amountIn.String(),
	})
	require.NoError(t, err)

	actual, err := k.SwapExactIn(ctx, 1, "umc", "uusdc", amountIn, sdk.ZeroInt(), trader)
	require.NoError(t, err)
	require.Equal(t, res.AmountOut, actual.String(), "quote must equal execution exactly")
}

// TestSwapPreservesModuleSolvency 交易后模块账户余额必须恰好等于池储备之和，
// 否则要么池子资不抵债、要么有资金滞留在模块账户里无人认领。
func TestSwapPreservesModuleSolvency(t *testing.T) {
	k, ctx, bk := setupDex(t)
	lp := addrOfDex(t)
	trader := addrOfDex(t)

	bk.setBalance(lp, "umc", 10_000_000_000)
	bk.setBalance(lp, "uusdc", 10_000_000_000)
	bk.setBalance(trader, "umc", 10_000_000_000)

	_, err := k.CreatePool(ctx, "umc", "uusdc",
		sdk.NewInt(1_000_000_000), sdk.NewInt(1_000_000_000), 30, lp, 0)
	require.NoError(t, err)

	_, err = k.SwapExactIn(ctx, 1, "umc", "uusdc", sdk.NewInt(123_456_789), sdk.ZeroInt(), trader)
	require.NoError(t, err)

	pool, _ := k.GetPool(ctx, 1)
	reserveA, _ := sdk.NewIntFromString(pool.ReserveA)
	reserveB, _ := sdk.NewIntFromString(pool.ReserveB)
	modAddr := sdk.MustAccAddressFromBech32(moduleAddrOf(types.ModuleName))

	require.Equal(t, reserveA.String(), bk.GetBalance(ctx, modAddr, "umc").Amount.String(),
		"模块 umc 余额必须恰等于储备 A")
	require.Equal(t, reserveB.String(), bk.GetBalance(ctx, modAddr, "uusdc").Amount.String(),
		"模块 uusdc 余额必须恰等于储备 B")
	require.True(t, k.UnencumberedBalance(ctx, "umc").IsZero(),
		"池储备之外不应有可动用余额")
}

// TestRemoveLiquidity_RejectsOverRedemption 赎回超过总供应量的 LP 必须被拒。
func TestRemoveLiquidity_RejectsOverRedemption(t *testing.T) {
	k, ctx, bk := setupDex(t)
	lp := addrOfDex(t)

	bk.setBalance(lp, "umc", 10_000_000_000)
	bk.setBalance(lp, "uusdc", 10_000_000_000)

	_, err := k.CreatePool(ctx, "umc", "uusdc",
		sdk.NewInt(100_000_000), sdk.NewInt(100_000_000), 30, lp, 0)
	require.NoError(t, err)

	pool, _ := k.GetPool(ctx, 1)
	totalLP, _ := sdk.NewIntFromString(pool.TotalLp)

	// 赎回量超过总供应 → 必须拒绝，不得按比例算出超额 payout
	_, _, err = k.RemoveLiquidity(ctx, 1, totalLP.AddRaw(1), sdk.ZeroInt(), sdk.ZeroInt(), lp)
	require.ErrorIs(t, err, types.ErrInsufficientLiquidity)

	// 池状态未被改动
	after, _ := k.GetPool(ctx, 1)
	require.Equal(t, pool.ReserveA, after.ReserveA)
	require.Equal(t, pool.ReserveB, after.ReserveB)
	require.Equal(t, pool.TotalLp, after.TotalLp)
}

// TestCreatePool_RejectsConfiscatoryFee 池主不得设置没收性费率。
func TestCreatePool_RejectsConfiscatoryFee(t *testing.T) {
	k, ctx, bk := setupDex(t)
	lp := addrOfDex(t)
	bk.setBalance(lp, "umc", 10_000_000_000)
	bk.setBalance(lp, "uusdc", 10_000_000_000)

	_, err := k.CreatePool(ctx, "umc", "uusdc",
		sdk.NewInt(100_000_000), sdk.NewInt(100_000_000),
		types.MaxPoolFeeRateBps+1, lp, 0)
	require.ErrorIs(t, err, types.ErrInvalidFeeRate)

	// 上限值本身允许
	_, err = k.CreatePool(ctx, "umc", "uusdc",
		sdk.NewInt(100_000_000), sdk.NewInt(100_000_000),
		types.MaxPoolFeeRateBps, lp, 0)
	require.NoError(t, err)
}
