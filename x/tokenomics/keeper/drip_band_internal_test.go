package keeper

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"mcchain/x/tokenomics/types"
)

// TestRenewalTarget_ClampedIntoBand —— 国库续期 APR 必须落在白皮书
// 定义的 1.00%–2.00% 带内：名义滴灌率（DripRatioBps，默认 5%）被 clamp，
// 而不是原样用于国库放水。
func TestRenewalTarget_ClampedIntoBand(t *testing.T) {
	staked := sdk.NewInt(1_000_000_000_000) // 1e12 umc

	perInterval := func(aprBps uint32) sdk.Int {
		return staked.MulRaw(int64(aprBps)).QuoRaw(10000).QuoRaw(IntervalsPerYear)
	}

	cases := []struct {
		name       string
		drip       uint32
		floor      uint32
		ceil       uint32
		wantAPRBps uint32
	}{
		// 默认参数：5% 名义率被 2% 上限截断（修复前这里会返回 5%，国库按 A 池速度放水）。
		{"default 5pct clamped to 2pct ceil", 500, 100, 200, 200},
		// 治理把名义率调到带内：原样使用。
		{"nominal inside band kept", 150, 100, 200, 150},
		// 治理把名义率压到下限以下：由下限托底，防止质押安全被饿死。
		{"nominal below floor lifted", 20, 100, 200, 100},
		// 边界：恰好等于上限/下限。
		{"nominal equals ceil", 200, 100, 200, 200},
		{"nominal equals floor", 100, 100, 200, 100},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := types.DefaultParams()
			p.DripRatioBps = tc.drip
			p.RenewalFloorAPRBps = tc.floor
			p.RenewalFloorAPRCeilBps = tc.ceil
			require.NoError(t, p.Validate())

			got := renewalTarget(staked, p)
			require.Equal(t, perInterval(tc.wantAPRBps), got,
				"renewal APR must be clamp(drip, floor, ceil)")
		})
	}

	// 回归护栏：默认参数下续期额必须严格小于 A 池的 5% 目标额。
	p := types.DefaultParams()
	require.True(t, renewalTarget(staked, p).LT(perInterval(p.DripRatioBps)),
		"treasury renewal must be slower than the staking-security pool release rate")
}
