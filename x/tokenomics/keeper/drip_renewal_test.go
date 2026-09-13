package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	keepertest "mcchain/testutil/keeper"
	"mcchain/x/tokenomics/types"
)

// TestDripWithRenewal_TreasuryBand —— 池 A（staking_security）耗尽后，
// 国库 B 的续期滴灌必须按白皮书 1–2% 带内的续期 APR 放水，而不是继承 A 池的
// 5% 目标额（后者会让国库以 12 年保卫线被击穿的速度流失）。
func TestDripWithRenewal_TreasuryBand(t *testing.T) {
	k, ctx, bk, _ := keepertest.TokenomicsKeeper(t)
	params := types.DefaultParams()
	require.NoError(t, k.SetParams(ctx, params))

	staked := sdk.NewInt(1_000_000_000_000) // 已质押 1e12 umc
	require.NoError(t, bk.MintCoins(ctx, stakingtypes.BondedPoolName,
		sdk.NewCoins(sdk.NewCoin(types.DefaultDenom, staked))))

	// 池 A 按创世额度注资（15% = 1.5e14），再全额转入国库 B：A 归零（耗尽）、B = 1.5e14。
	require.NoError(t, bk.MintCoins(ctx, types.StakingSecurityPoolName,
		sdk.NewCoins(sdk.NewCoin(types.DefaultDenom, sdk.NewInt(150_000_000_000_000)))))
	aBal := bk.GetBalance(ctx, types.StakingSecurityPoolAddress(), types.DefaultDenom).Amount
	require.True(t, aBal.IsPositive())
	require.NoError(t, bk.SendCoinsFromModuleToModule(ctx,
		types.StakingSecurityPoolName, types.ProtocolTreasuryPoolName,
		sdk.NewCoins(sdk.NewCoin(types.DefaultDenom, aBal))))
	require.True(t, bk.GetBalance(ctx, types.StakingSecurityPoolAddress(), types.DefaultDenom).Amount.IsZero())
	bBefore := bk.GetBalance(ctx, types.ProtocolTreasuryAddress(), types.DefaultDenom).Amount
	require.Equal(t, aBal, bBefore)

	require.NoError(t, k.DripWithRenewal(ctx))

	feeCollector := bk.GetBalance(ctx,
		authtypes.NewModuleAddress(authtypes.FeeCollectorName), types.DefaultDenom).Amount

	// 期望：staked × ceil(2%) / 年区间数 —— 与 drip.go 的区间口径保持一致，
	// 但公式在测试里独立重写，避免与被测实现共享代码而失去甄别力。
	want := staked.MulRaw(int64(params.RenewalFloorAPRCeilBps)).
		QuoRaw(10000).QuoRaw(78840)
	require.Equal(t, want, feeCollector, "treasury renewal must use the 2% ceil, not the 5% pool-A target")

	// 反例护栏：若沿用 A 池 5% 目标，金额会是 5% 口径 —— 必须大于当前实发额。
	nominal5 := staked.MulRaw(int64(params.DripRatioBps)).QuoRaw(10000).QuoRaw(78840)
	require.True(t, feeCollector.LT(nominal5),
		"treasury must not drain at the staking-security release rate")

	// 国库余额按实发额扣减。
	require.Equal(t, bBefore.Sub(feeCollector),
		bk.GetBalance(ctx, types.ProtocolTreasuryAddress(), types.DefaultDenom).Amount)
}

// TestDripWithRenewal_PoolAStillUsesDripRatio —— 池 A 未耗尽时仍按 5% 名义率
// 放水（续期逻辑不得改变 A 池行为）。
func TestDripWithRenewal_PoolAStillUsesDripRatio(t *testing.T) {
	k, ctx, bk, _ := keepertest.TokenomicsKeeper(t)
	params := types.DefaultParams()
	require.NoError(t, k.SetParams(ctx, params))

	staked := sdk.NewInt(1_000_000_000_000)
	require.NoError(t, bk.MintCoins(ctx, stakingtypes.BondedPoolName,
		sdk.NewCoins(sdk.NewCoin(types.DefaultDenom, staked))))

	// 给池 A 注资（不设国库），使 A 有余额可放。
	fund := sdk.NewInt(1_000_000_000_000_000)
	require.NoError(t, bk.MintCoins(ctx, types.StakingSecurityPoolName,
		sdk.NewCoins(sdk.NewCoin(types.DefaultDenom, fund))))

	require.NoError(t, k.DripWithRenewal(ctx))

	got := bk.GetBalance(ctx, authtypes.NewModuleAddress(authtypes.FeeCollectorName), types.DefaultDenom).Amount
	want := staked.MulRaw(int64(params.DripRatioBps)).QuoRaw(10000).QuoRaw(78840)
	require.Equal(t, want, got, "pool A release rate must remain DripRatioBps (5%)")
}
