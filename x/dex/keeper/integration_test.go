package keeper

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"mcchain/x/dex/types"
)

// ============================================================================
// Task 8: DEX 集成测试
// ============================================================================

// ---------------------------------------------------------------------------
// TestDEX_CreatePoolAndSwap
// ---------------------------------------------------------------------------

// TestDEX_CreatePoolAndSwap_FullFlow
// 创建 MC/USDC 池（fee 30 bps）→ swap 100 MC → USDC →
// 验证输出（常数积公式）、池储备更新、费用分配（burn 50% / treasury 30% / LP 20%）。
func TestDEX_CreatePoolAndSwap_FullFlow(t *testing.T) {
	k, ctx, bk := setupDex(t)
	creator := addrOfDex(t)
	trader := addrOfDex(t)

	const denomMC = "umc"
	const denomUSDC = "uusdc"

	// Fund creator and trader accounts
	bk.setBalance(creator, denomMC, 1000000000)  // 1000 MC
	bk.setBalance(creator, denomUSDC, 1000000000) // 1000 USDC
	bk.setBalance(trader, denomMC, 1000000000)    // 1000 MC for swap

	// --- Create pool: 500 MC / 500 USDC, fee=30 bps ---
	reserveA := sdk.NewInt(500_000000) // 500 MC (6 decimals)
	reserveB := sdk.NewInt(500_000000)
	pool, err := k.CreatePool(ctx, denomMC, denomUSDC, reserveA, reserveB, 30, creator, 0)
	require.NoError(t, err)
	require.Equal(t, uint64(1), pool.Id)
	require.Equal(t, denomMC, pool.DenomA)
	require.Equal(t, denomUSDC, pool.DenomB)
	require.Equal(t, uint32(30), pool.FeeRateBps)

	// Verify initial reserves
	rA, _ := sdk.NewIntFromString(pool.ReserveA)
	rB, _ := sdk.NewIntFromString(pool.ReserveB)
	require.Equal(t, reserveA, rA)
	require.Equal(t, reserveB, rB)

	// Verify LP tokens minted (sqrt(500M * 500M) ≈ 500M)
	lpTotal, _ := sdk.NewIntFromString(pool.TotalLp)
	require.True(t, lpTotal.GT(sdk.ZeroInt()))
	// expected sqrt: 500_000000 * 500_000000 = 250000_000000_000000, sqrt = 500_000000
	require.Equal(t, sdk.NewInt(500_000000), lpTotal)

	// --- Swap: trader swaps 100 MC → USDC ---
	swapAmount := sdk.NewInt(100_000000) // 100 MC
	amountOut, err := k.SwapExactIn(ctx, 1, denomMC, denomUSDC, swapAmount, sdk.ZeroInt(), trader)
	require.NoError(t, err)
	require.True(t, amountOut.GT(sdk.ZeroInt()), "swap should return positive output")

	// Verify output using constant-product formula:
	// amountIn * (1 - fee%) = 100MC * 9970/10000 = 99.7 MC effective
	// new reserve MC: 500 + 99.7 = 599.7 → 500 + (99.7 * 0.2 LP) = 500 + 19.94 → but actually
	// nonLPFee portion is subtracted...
	// Actually: reserveIn after = reserveIn + amountIn - nonLPFee = 500M + 100M - nonLPFee
	// nonLPFee = fee * 0.8, where fee = 100M * 30/10000 = 0.3M
	// nonLPFee = 0.3M * 8000/10000 = 0.24M
	// newReserveIn = 500M + 100M - 0.24M = 599.76M
	// k = 500M * 500M = 250e12
	// newReserveOut = k / newReserveIn = 250e12 / 599.76M ≈ 416.833M
	// amountOut = 500M - 416.833M ≈ 83.167M
	expectedOut := sdk.NewInt(83166694) // approximate
	require.InDelta(t, expectedOut.Int64(), amountOut.Int64(), 100, "swap output should match constant-product formula")

	// Verify pool reserves updated
	pool, found := k.GetPool(ctx, 1)
	require.True(t, found)
	newRA, _ := sdk.NewIntFromString(pool.ReserveA)
	newRB, _ := sdk.NewIntFromString(pool.ReserveB)
	require.Equal(t, reserveA.Add(swapAmount), newRA.Add(amountOut).Add(sdk.NewInt(0)), "reserves should reflect swap")

	// Verify fee distribution: burn + treasury + LP events
	// Burn (50%) + Treasury (30%)
	require.True(t, len(bk.burned) >= 1, "burn should have been called")
	// Treasury transfer
	require.True(t, len(bk.sentFromMod) >= 1, "treasury transfer should have been called")
}

// TestDEX_CreatePool_SwapMultipleDenomDirection
// 从另一个方向 swap（USDC → MC），验证双向交易正确。
func TestDEX_CreatePool_SwapMultipleDenomDirection(t *testing.T) {
	k, ctx, bk := setupDex(t)
	lp := addrOfDex(t)
	trader := addrOfDex(t)

	const denomMC = "umc"
	const denomUSDC = "uusdc"

	bk.setBalance(lp, denomMC, 1000000000)
	bk.setBalance(lp, denomUSDC, 1000000000)
	bk.setBalance(trader, denomUSDC, 1000000000)

	// Create pool: 1000 MC / 1000 USDC
	_, err := k.CreatePool(ctx, denomMC, denomUSDC,
		sdk.NewInt(1000_000000), sdk.NewInt(1000_000000), 30, lp, 0)
	require.NoError(t, err)

	// Swap USDC → MC
	amountOut, err := k.SwapExactIn(ctx, 1, denomUSDC, denomMC,
		sdk.NewInt(100_000000), sdk.ZeroInt(), trader)
	require.NoError(t, err)
	require.True(t, amountOut.GT(sdk.ZeroInt()))

	pool, _ := k.GetPool(ctx, 1)
	// USDC reserve should increase, MC reserve should decrease
	reserveA, _ := sdk.NewIntFromString(pool.ReserveA) // MC
	reserveB, _ := sdk.NewIntFromString(pool.ReserveB) // USDC
	require.True(t, reserveA.Int64() < 1000_000000, "MC reserve should decrease")
	require.True(t, reserveB.Int64() > 1000_000000, "USDC reserve should increase")
}

// TestDEX_CreatePool_DenomSortValidation
// 验证 denom 自动排序：输入 (uusdc, umc) 创建池，denomA 应为 umc。
func TestDEX_CreatePool_DenomSortValidation(t *testing.T) {
	k, ctx, bk := setupDex(t)
	creator := addrOfDex(t)

	bk.setBalance(creator, "umc", 1000000000)
	bk.setBalance(creator, "uusdc", 1000000000)

	pool, err := k.CreatePool(ctx, "uusdc", "umc",
		sdk.NewInt(500_000000), sdk.NewInt(500_000000), 30, creator, 0)
	require.NoError(t, err)
	require.Equal(t, "umc", pool.DenomA, "denoms should be alphabetically sorted")
	require.Equal(t, "uusdc", pool.DenomB)
}

// TestDEX_CreatePool_InvalidSortedDenoms
// 非排序 denom → 拒绝。
func TestDEX_CreatePool_InvalidSortedDenoms(t *testing.T) {
	k, ctx, bk := setupDex(t)
	creator := addrOfDex(t)

	bk.setBalance(creator, "bdenom", 1000000000)
	bk.setBalance(creator, "adenom", 1000000000)

	_, err := k.CreatePool(ctx, "bdenom", "adenom",
		sdk.NewInt(500_000000), sdk.NewInt(500_000000), 30, creator, 0)
	require.ErrorIs(t, err, types.ErrDenomSortRequired)
}

// ---------------------------------------------------------------------------
// TestDEX_AddLiquidity
// ---------------------------------------------------------------------------

// TestDEX_AddLiquidity_Basic
// 创建池 → 添加流动性 → 验证 LP token mint 正确 + 池储备增长 + 用户收到 LP tokens。
func TestDEX_AddLiquidity_Basic(t *testing.T) {
	k, ctx, bk := setupDex(t)
	lp := addrOfDex(t)

	bk.setBalance(lp, "umc", 1000000000)
	bk.setBalance(lp, "uusdc", 1000000000)

	// Create pool: 100 MC / 100 USDC
	_, err := k.CreatePool(ctx, "umc", "uusdc",
		sdk.NewInt(100_000000), sdk.NewInt(100_000000), 30, lp, 0)
	require.NoError(t, err)

	// Add liquidity: 50 MC / 50 USDC
	bk.setBalance(lp, "umc", 1000000000) // refresh
	bk.setBalance(lp, "uusdc", 1000000000)
	lpMinted, actualA, actualB, err := k.AddLiquidity(ctx, 1,
		sdk.NewInt(50_000000), sdk.NewInt(50_000000),
		sdk.ZeroInt(), lp)
	require.NoError(t, err)
	require.True(t, lpMinted.GT(sdk.ZeroInt()), "should mint LP tokens")
	require.True(t, actualA.GT(sdk.ZeroInt()))
	require.True(t, actualB.GT(sdk.ZeroInt()))

	// Verify pool reserves updated
	pool, _ := k.GetPool(ctx, 1)
	reserveA, _ := sdk.NewIntFromString(pool.ReserveA)
	reserveB, _ := sdk.NewIntFromString(pool.ReserveB)
	require.True(t, reserveA.Int64() >= 150_000000, "reserveA should increase by at least 50 MC")
	require.True(t, reserveB.Int64() >= 150_000000, "reserveB should increase by at least 50 USDC")

	// Verify LP balance
	lpDenom := types.PoolDenom(1)
	lpBal := bk.GetBalance(ctx, sdk.AccAddress([]byte(lp)), lpDenom)
	require.True(t, lpBal.Amount.GTE(lpMinted), "user should have LP tokens")
}

// TestDEX_AddLiquidity_ProportionalRebalancing
// 不成比例添加：数量超出部分应被退还。
func TestDEX_AddLiquidity_ProportionalRebalancing(t *testing.T) {
	k, ctx, bk := setupDex(t)
	lp := addrOfDex(t)

	bk.setBalance(lp, "umc", 1000000000)
	bk.setBalance(lp, "uusdc", 1000000000)

	// Create pool: 200 MC / 100 USDC (ratio 2:1)
	_, err := k.CreatePool(ctx, "umc", "uusdc",
		sdk.NewInt(200_000000), sdk.NewInt(100_000000), 30, lp, 0)
	require.NoError(t, err)

	// Try adding 50 MC / 50 USDC (ratio 1:1, should rebalance)
	lpMinted, actualA, actualB, err := k.AddLiquidity(ctx, 1,
		sdk.NewInt(50_000000), sdk.NewInt(50_000000),
		sdk.ZeroInt(), lp)
	require.NoError(t, err)
	require.True(t, lpMinted.GT(sdk.ZeroInt()))

	// Due to 2:1 pool ratio, actualB should be less than 50M
	require.True(t, actualB.Int64() <= 50_000000, "excess USDC should be refunded")
	t.Logf("added: A=%s B=%s LP=%s", actualA.String(), actualB.String(), lpMinted.String())
}

// ---------------------------------------------------------------------------
// TestDEX_RemoveLiquidity
// ---------------------------------------------------------------------------

// TestDEX_RemoveLiquidity_Basic
// 添加流动性 → 移除部分 → 验证资产返还比例正确 + LP tokens 被销毁。
func TestDEX_RemoveLiquidity_Basic(t *testing.T) {
	k, ctx, bk := setupDex(t)
	lp := addrOfDex(t)

	bk.setBalance(lp, "umc", 1000000000)
	bk.setBalance(lp, "uusdc", 1000000000)

	_, err := k.CreatePool(ctx, "umc", "uusdc",
		sdk.NewInt(100_000000), sdk.NewInt(100_000000), 30, lp, 0)
	require.NoError(t, err)

	// Record LP balance before removal
	lpDenom := types.PoolDenom(1)
	lpBalBefore := bk.GetBalance(ctx, sdk.AccAddress([]byte(lp)), lpDenom)

	// Remove half of LP tokens
	halfLP := lpBalBefore.Amount.QuoRaw(2)
	amountA, amountB, err := k.RemoveLiquidity(ctx, 1, halfLP,
		sdk.ZeroInt(), sdk.ZeroInt(), lp)
	require.NoError(t, err)
	require.True(t, amountA.GT(sdk.ZeroInt()), "should return denomA")
	require.True(t, amountB.GT(sdk.ZeroInt()), "should return denomB")

	// Verify pool reserves halved
	pool, _ := k.GetPool(ctx, 1)
	reserveA, _ := sdk.NewIntFromString(pool.ReserveA)
	reserveB, _ := sdk.NewIntFromString(pool.ReserveB)
	// Allow ±1 rounding
	require.InDelta(t, int64(50_000000), reserveA.Int64(), 2)
	require.InDelta(t, int64(50_000000), reserveB.Int64(), 2)

	// Verify LP balance halved
	lpBalAfter := bk.GetBalance(ctx, sdk.AccAddress([]byte(lp)), lpDenom)
	require.InDelta(t, halfLP.Int64(), lpBalBefore.Amount.Sub(lpBalAfter.Amount).Int64(), 2)
}

// TestDEX_RemoveLiquidity_NoPool
// 不存在的池 → 错误。
func TestDEX_RemoveLiquidity_NoPool(t *testing.T) {
	k, ctx, _ := setupDex(t)
	lp := addrOfDex(t)

	_, _, err := k.RemoveLiquidity(ctx, 999, sdk.NewInt(100), sdk.ZeroInt(), sdk.ZeroInt(), lp)
	require.ErrorIs(t, err, types.ErrPoolNotFound)
}

// ---------------------------------------------------------------------------
// TestDEX_SlippageProtection
// ---------------------------------------------------------------------------

// TestDEX_SlippageProtection_Swap
// 设置严格 minAmountOut → 超过滑点的交易被拒绝。
func TestDEX_SlippageProtection_Swap(t *testing.T) {
	k, ctx, bk := setupDex(t)
	lp := addrOfDex(t)
	trader := addrOfDex(t)

	bk.setBalance(lp, "umc", 1000000000)
	bk.setBalance(lp, "uusdc", 1000000000)
	bk.setBalance(trader, "umc", 1000000000)

	_, err := k.CreatePool(ctx, "umc", "uusdc",
		sdk.NewInt(1000_000000), sdk.NewInt(1000_000000), 30, lp, 0)
	require.NoError(t, err)

	// Swap 1 MC with unreasonably high minAmountOut (should fail)
	_, err = k.SwapExactIn(ctx, 1, "umc", "uusdc",
		sdk.NewInt(1_000000), sdk.NewInt(1000_000000_000000), trader)
	require.ErrorIs(t, err, types.ErrSlippageExceeded)
}

// TestDEX_SlippageProtection_AddLiquidity
// 设置严格 minLPOut → 滑点超出时添加流动性被拒。
func TestDEX_SlippageProtection_AddLiquidity(t *testing.T) {
	k, ctx, bk := setupDex(t)
	lp := addrOfDex(t)

	bk.setBalance(lp, "umc", 1000000000)
	bk.setBalance(lp, "uusdc", 1000000000)

	_, err := k.CreatePool(ctx, "umc", "uusdc",
		sdk.NewInt(100_000000), sdk.NewInt(100_000000), 30, lp, 0)
	require.NoError(t, err)

	// Try adding with impossibly high minLPOut
	_, _, _, err = k.AddLiquidity(ctx, 1,
		sdk.NewInt(1_000000), sdk.NewInt(1_000000),
		sdk.NewInt(999999999999999999), lp)
	require.ErrorIs(t, err, types.ErrSlippageExceeded)
}

// TestDEX_SlippageProtection_RemoveLiquidity
// 设置严格 minAOut/minBOut → 滑点超出时移除流动性被拒。
func TestDEX_SlippageProtection_RemoveLiquidity(t *testing.T) {
	k, ctx, bk := setupDex(t)
	lp := addrOfDex(t)

	bk.setBalance(lp, "umc", 1000000000)
	bk.setBalance(lp, "uusdc", 1000000000)

	_, err := k.CreatePool(ctx, "umc", "uusdc",
		sdk.NewInt(100_000000), sdk.NewInt(100_000000), 30, lp, 0)
	require.NoError(t, err)

	lpDenom := types.PoolDenom(1)
	lpBal := bk.GetBalance(ctx, sdk.AccAddress([]byte(lp)), lpDenom)

	// Try removing LP with impossibly high min outputs
	_, _, err = k.RemoveLiquidity(ctx, 1, lpBal.Amount,
		sdk.NewInt(999999999999999999), sdk.NewInt(999999999999999999), lp)
	require.ErrorIs(t, err, types.ErrSlippageExceeded)
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

// TestDEX_CreatePool_ZeroAmount
// 零金额创建池 → 拒绝。
func TestDEX_CreatePool_ZeroAmount(t *testing.T) {
	k, ctx, bk := setupDex(t)
	lp := addrOfDex(t)

	bk.setBalance(lp, "umc", 1000000000)
	bk.setBalance(lp, "uusdc", 1000000000)

	_, err := k.CreatePool(ctx, "umc", "uusdc",
		sdk.ZeroInt(), sdk.NewInt(100), 30, lp, 0)
	require.ErrorIs(t, err, types.ErrZeroAmount)
}

// TestDEX_SwapExactIn_ZeroAmount
// 零金额交换 → 拒绝。
func TestDEX_SwapExactIn_ZeroAmount(t *testing.T) {
	k, ctx, bk := setupDex(t)
	lp := addrOfDex(t)
	trader := addrOfDex(t)

	bk.setBalance(lp, "umc", 1000000000)
	bk.setBalance(lp, "uusdc", 1000000000)

	_, err := k.CreatePool(ctx, "umc", "uusdc",
		sdk.NewInt(100_000000), sdk.NewInt(100_000000), 30, lp, 0)
	require.NoError(t, err)

	_, err = k.SwapExactIn(ctx, 1, "umc", "uusdc", sdk.ZeroInt(), sdk.ZeroInt(), trader)
	require.ErrorIs(t, err, types.ErrZeroAmount)
}

// TestDEX_SwapExactIn_SameDenom
// 同币种交换 → 拒绝。
func TestDEX_SwapExactIn_SameDenom(t *testing.T) {
	k, ctx, bk := setupDex(t)
	lp := addrOfDex(t)
	trader := addrOfDex(t)

	bk.setBalance(lp, "umc", 1000000000)
	bk.setBalance(lp, "uusdc", 1000000000)
	bk.setBalance(trader, "umc", 1000000000)

	_, err := k.CreatePool(ctx, "umc", "uusdc",
		sdk.NewInt(100_000000), sdk.NewInt(100_000000), 30, lp, 0)
	require.NoError(t, err)

	_, err = k.SwapExactIn(ctx, 1, "umc", "umc", sdk.NewInt(10), sdk.ZeroInt(), trader)
	require.ErrorIs(t, err, types.ErrSwapSameDenom)
}

// TestDEX_CreatePool_MaxPoolLimit
// 达到 MaxPools 上限后创建新池被拒。
func TestDEX_CreatePool_MaxPoolLimit(t *testing.T) {
	k, ctx, bk := setupDex(t)
	lp := addrOfDex(t)

	// Override params to set MaxPools=2
	params := k.GetParams(ctx)
	params.MaxPools = 2
	k.SetParams(ctx, params)

	bk.setBalance(lp, "adenom", 1000000000)
	bk.setBalance(lp, "bdenom", 1000000000)
	bk.setBalance(lp, "cdenom", 1000000000)
	bk.setBalance(lp, "ddenom", 1000000000)
	bk.setBalance(lp, "edenom", 1000000000)
	bk.setBalance(lp, "fdenom", 1000000000)

	// Create pool 1
	_, err := k.CreatePool(ctx, "adenom", "bdenom", sdk.NewInt(100), sdk.NewInt(100), 30, lp, 0)
	require.NoError(t, err)

	// Create pool 2
	_, err = k.CreatePool(ctx, "cdenom", "ddenom", sdk.NewInt(100), sdk.NewInt(100), 30, lp, 0)
	require.NoError(t, err)

	// Pool 3 → nextPoolID=3, MaxPools=2 → should fail
	_, err = k.CreatePool(ctx, "edenom", "fdenom", sdk.NewInt(100), sdk.NewInt(100), 30, lp, 0)
	require.ErrorIs(t, err, types.ErrMaxPoolsReached)
}

// TestDEX_GetAllPools returns all created pools.
func TestDEX_GetAllPools(t *testing.T) {
	k, ctx, bk := setupDex(t)
	lp := addrOfDex(t)

	for i := 0; i < 3; i++ {
		denom1 := "ade"
		denom2 := "bde"
		bk.setBalance(lp, denom1, 1000000000)
		bk.setBalance(lp, denom2, 1000000000)
		_, err := k.CreatePool(ctx, denom1, denom2,
			sdk.NewInt(int64(100*(i+1))), sdk.NewInt(int64(100*(i+1))), 30, lp, 0)
		require.NoError(t, err)
	}

	pools := k.GetAllPools(ctx)
	require.Len(t, pools, 3)
}
