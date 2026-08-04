package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	keepertest "mcchain/testutil/keeper"
	"mcchain/x/tokenomics/types"
)

// TestParamsDefaultMatchesConstants 验证默认参数与创世常量一致（保证行为不变）。
func TestParamsDefaultMatchesConstants(t *testing.T) {
	p := types.DefaultParams()
	require.NoError(t, p.Validate())
	require.Equal(t, types.DripRatioBps, p.DripRatioBps)
	require.Equal(t, types.RenewalFloorAPRBps, p.RenewalFloorAPRBps)
	require.Equal(t, types.RenewalFloorAPRCeilBps, p.RenewalFloorAPRCeilBps)
	require.Equal(t, uint32(types.DripFloorYears), p.DripFloorYears)
}

// TestParamsValidateRejectsBadValues 非法参数被拒（fail-closed）。
func TestParamsValidateRejectsBadValues(t *testing.T) {
	bad := types.DefaultParams()
	bad.DripFloorYears = 0
	require.Error(t, bad.Validate())

	bad2 := types.DefaultParams()
	bad2.RenewalFloorAPRBps = 500
	bad2.RenewalFloorAPRCeilBps = 200
	require.Error(t, bad2.Validate())

	bad3 := types.DefaultParams()
	bad3.DripRatioBps = 20_000
	require.Error(t, bad3.Validate())
}

// TestParamsGetSet 默认兜底 + 写入读回 + 非法值拒绝写入。
func TestParamsGetSet(t *testing.T) {
	k, ctx, _, _ := keepertest.TokenomicsKeeper(t)

	// 未初始化 → 默认值兜底。
	require.Equal(t, types.DefaultParams(), k.GetParams(ctx))

	// 写入自定义参数并读回。
	custom := types.DefaultParams()
	custom.DripRatioBps = 400 // 4%
	custom.RenewalFloorAPRBps = 150
	require.NoError(t, k.SetParams(ctx, custom))
	require.Equal(t, custom, k.GetParams(ctx))

	// 非法值拒绝写入，且不覆盖已存参数。
	require.Error(t, k.SetParams(ctx, types.Params{DripFloorYears: 0}))
	require.Equal(t, custom, k.GetParams(ctx))
}

// TestInitGenesisSetsDefaultParams 创世后参数为默认值。
func TestInitGenesisSetsDefaultParams(t *testing.T) {
	k, ctx, _, _ := keepertest.TokenomicsKeeper(t)
	gs := types.DefaultGenesis()
	require.NoError(t, k.InitGenesis(ctx, *gs))
	require.Equal(t, types.DefaultParams(), k.GetParams(ctx))
}
