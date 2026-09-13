package keeper

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"mcchain/x/dex/types"
	tokenomicstypes "mcchain/x/tokenomics/types"
)

// 首存必须永久锁定 MinimumLiquidity 份 LP。
//
// 修复前：lpMinted = sqrt(addedA*addedB) 全额发给首存者，TotalLp 可被其一人
// 全额赎回归零。攻击路径为——用极小额建池拿走 100% LP，再单边打入大额资产把
// 「每份 LP 的含金量」抬到远高于 1 单位，随后任何正常存款按比例折算都不足 1 份、
// 被整数除法舍入为 0 份，而两条腿的资产照收（首存者/份额膨胀攻击）；赎回清零后
// 还能对同一个池子按操纵价反复重新播种。
//
// 修复后：首存额外铸出 MinimumLiquidity 份直接打入黑洞（无私钥、不可支出），
// 该份额计入 TotalLp 但永不可赎回，池子的 LP 供应与储备都不再可能归零。

func TestCreatePool_LocksMinimumLiquidity(t *testing.T) {
	k, ctx, bk := setupDex(t)
	lp := addrOfDex(t)
	bk.setBalance(lp, "umc", 10_000_000_000)
	bk.setBalance(lp, "uusdc", 10_000_000_000)

	pool, err := k.CreatePool(ctx, "umc", "uusdc",
		sdk.NewInt(1_000_000), sdk.NewInt(1_000_000), 30, lp, 0)
	require.NoError(t, err)

	// 记账口径不变：TotalLp 仍是几何平均 sqrt(1e6 × 1e6) = 1e6。
	require.Equal(t, "1000000", pool.TotalLp)

	lpDenom := types.PoolDenom(pool.Id)
	userLP := bk.GetBalance(ctx, sdk.MustAccAddressFromBech32(lp), lpDenom).Amount
	lockedLP := bk.GetBalance(ctx, tokenomicstypes.BlackHoleAddress(), lpDenom).Amount

	require.Equal(t, sdk.NewInt(types.MinimumLiquidity), lockedLP,
		"MinimumLiquidity 份 LP 必须被打入黑洞地址")
	require.Equal(t, sdk.NewInt(1_000_000-types.MinimumLiquidity), userLP,
		"建池者只应拿到扣除锁定份额后的余量")
	require.Equal(t, pool.TotalLp, userLP.Add(lockedLP).String(),
		"bank 侧 LP 总供应必须与池记账 TotalLp 完全一致，不得出现悬空份额")
}

func TestCreatePool_RejectsDustInitialDeposit(t *testing.T) {
	k, ctx, bk := setupDex(t)
	lp := addrOfDex(t)
	bk.setBalance(lp, "umc", 10_000_000_000)
	bk.setBalance(lp, "uusdc", 10_000_000_000)

	// sqrt(1×1) = 1，远小于锁定份额 → 拒绝，而不是给建池者铸 0 份。
	_, err := k.CreatePool(ctx, "umc", "uusdc",
		sdk.NewInt(1), sdk.NewInt(1), 30, lp, 0)
	require.ErrorIs(t, err, types.ErrInsufficientLiquidity)

	// 边界：sqrt 恰好等于 MinimumLiquidity → 全部会被锁死、用户拿 0 份 → 仍须拒绝。
	_, err = k.CreatePool(ctx, "umc", "uusdc",
		sdk.NewInt(types.MinimumLiquidity), sdk.NewInt(types.MinimumLiquidity), 30, lp, 0)
	require.ErrorIs(t, err, types.ErrInsufficientLiquidity)

	// 边界 +1：sqrt = 1001 → 放行，用户拿到 1 份。
	pool, err := k.CreatePool(ctx, "umc", "uusdc",
		sdk.NewInt(types.MinimumLiquidity+1), sdk.NewInt(types.MinimumLiquidity+1), 30, lp, 0)
	require.NoError(t, err)
	require.Equal(t, "1001", pool.TotalLp)
	require.Equal(t, sdk.OneInt(),
		bk.GetBalance(ctx, sdk.MustAccAddressFromBech32(lp), types.PoolDenom(pool.Id)).Amount)
}

// 池子被清空后重新播种，同样走首存路径，必须再次锁定 MinimumLiquidity。
// 否则「全额赎回 → 按操纵价重播」这条路可以绕开建池时的那道锁。
func TestAddLiquidity_ReseedAfterDrain_LocksMinimumLiquidity(t *testing.T) {
	k, ctx, bk := setupDex(t)
	lp := addrOfDex(t)
	bk.setBalance(lp, "umc", 10_000_000_000)
	bk.setBalance(lp, "uusdc", 10_000_000_000)

	k.SetPool(ctx, types.Pool{
		Id:         7,
		DenomA:     "umc",
		DenomB:     "uusdc",
		ReserveA:   "0",
		ReserveB:   "0",
		TotalLp:    "0",
		FeeRateBps: 30,
		Owner:      lp,
	})

	lpMinted, _, _, err := k.AddLiquidity(ctx, 7,
		sdk.NewInt(4_000_000), sdk.NewInt(4_000_000), sdk.ZeroInt(), lp)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(4_000_000-types.MinimumLiquidity), lpMinted)

	pool, found := k.GetPool(ctx, 7)
	require.True(t, found)
	require.Equal(t, "4000000", pool.TotalLp)
	require.Equal(t, sdk.NewInt(types.MinimumLiquidity),
		bk.GetBalance(ctx, tokenomicstypes.BlackHoleAddress(), types.PoolDenom(7)).Amount)

	// 再次追加（此时 TotalLp > 0）走的是按比例路径，不得重复锁定。
	before := bk.GetBalance(ctx, tokenomicstypes.BlackHoleAddress(), types.PoolDenom(7)).Amount
	_, _, _, err = k.AddLiquidity(ctx, 7,
		sdk.NewInt(1_000_000), sdk.NewInt(1_000_000), sdk.ZeroInt(), lp)
	require.NoError(t, err)
	require.Equal(t, before,
		bk.GetBalance(ctx, tokenomicstypes.BlackHoleAddress(), types.PoolDenom(7)).Amount,
		"非首存不得再锁定额外份额")
}

// 锁定份额不可赎回：池子的 LP 供应与储备永远不会被清零。
func TestRemoveLiquidity_LockedMinimumIsUnredeemable(t *testing.T) {
	k, ctx, bk := setupDex(t)
	lp := addrOfDex(t)
	bk.setBalance(lp, "umc", 10_000_000_000)
	bk.setBalance(lp, "uusdc", 10_000_000_000)

	pool, err := k.CreatePool(ctx, "umc", "uusdc",
		sdk.NewInt(1_000_000), sdk.NewInt(1_000_000), 30, lp, 0)
	require.NoError(t, err)

	lpDenom := types.PoolDenom(pool.Id)
	totalLP, _ := sdk.NewIntFromString(pool.TotalLp)

	// 试图赎回包含锁定份额在内的全部记账份额：用户根本不持有那 1000 份，扣款失败。
	_, _, err = k.RemoveLiquidity(ctx, pool.Id, totalLP, sdk.ZeroInt(), sdk.ZeroInt(), lp)
	require.Error(t, err, "无人可以赎回黑洞里的锁定份额")

	// 赎回自己持有的全部份额：成功，但池子不会归零。
	userLP := bk.GetBalance(ctx, sdk.MustAccAddressFromBech32(lp), lpDenom).Amount
	_, _, err = k.RemoveLiquidity(ctx, pool.Id, userLP, sdk.ZeroInt(), sdk.ZeroInt(), lp)
	require.NoError(t, err)

	after, found := k.GetPool(ctx, pool.Id)
	require.True(t, found)
	require.Equal(t, sdk.NewInt(types.MinimumLiquidity).String(), after.TotalLp,
		"清仓后 TotalLp 必须恰好剩下永久锁定的那部分")

	reserveA, _ := sdk.NewIntFromString(after.ReserveA)
	reserveB, _ := sdk.NewIntFromString(after.ReserveB)
	require.True(t, reserveA.IsPositive(), "锁定份额对应的储备必须留在池中")
	require.True(t, reserveB.IsPositive(), "锁定份额对应的储备必须留在池中")
}

// splitInitialLP 的纯函数边界。
func TestSplitInitialLP_Bounds(t *testing.T) {
	minLP := sdk.NewInt(types.MinimumLiquidity)

	_, _, ok := splitInitialLP(sdk.ZeroInt())
	require.False(t, ok)

	_, _, ok = splitInitialLP(minLP)
	require.False(t, ok, "恰好等于锁定份额时用户份额为 0，必须拒绝")

	user, locked, ok := splitInitialLP(minLP.AddRaw(1))
	require.True(t, ok)
	require.Equal(t, sdk.OneInt(), user)
	require.Equal(t, minLP, locked)

	user, locked, ok = splitInitialLP(sdk.NewInt(1_000_000))
	require.True(t, ok)
	require.Equal(t, sdk.NewInt(1_000_000-types.MinimumLiquidity), user)
	require.Equal(t, minLP, locked)
	require.Equal(t, sdk.NewInt(1_000_000), user.Add(locked), "拆分不得凭空增减总量")

	_, _, ok = splitInitialLP(sdk.Int{})
	require.False(t, ok, "nil Int 不得 panic")
}
