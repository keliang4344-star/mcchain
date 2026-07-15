package keeper_test

import (
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	keepertest "mcchain/testutil/keeper"
	"mcchain/x/tokenomics/types"
)

// TestQuerySupply 验证 Query/Supply：返回 cap、minted（=cap）、denom。
func TestQuerySupply(t *testing.T) {
	k, ctx, _, _ := keepertest.TokenomicsKeeper(t)

	gs := types.DefaultGenesis()
	require.NoError(t, k.InitGenesis(ctx, *gs))

	res, err := k.Supply(sdk.WrapSDKContext(ctx), &types.QuerySupplyRequest{})
	require.NoError(t, err)
	require.Equal(t, capAmt, res.TotalSupplyCap)
	require.Equal(t, capAmt, res.MintedSupply)
	require.Equal(t, types.DefaultDenom, res.Denom)
}

// TestQueryAllocations 验证 Query/Allocations：占比、拨付额、当前余额（实时）正确。
func TestQueryAllocations(t *testing.T) {
	k, ctx, _, _ := keepertest.TokenomicsKeeper(t)

	gs := types.DefaultGenesis()
	require.NoError(t, k.InitGenesis(ctx, *gs))

	res, err := k.Allocations(sdk.WrapSDKContext(ctx), &types.QueryAllocationsRequest{})
	require.NoError(t, err)
	require.Len(t, res.Allocations, 3)

	byName := make(map[string]types.PoolView)
	for _, v := range res.Allocations {
		byName[v.Name] = v
	}

	// 占比（基点）与拨付额（umc）符合 Q2 默认：15% / 35% / 50%。
	require.Equal(t, types.TeamPercentBps, byName[types.TeamPoolName].PercentBps)
	require.Equal(t, types.CommunityPercentBps, byName[types.CommunityPoolName].PercentBps)
	require.Equal(t, types.EcosystemPercentBps, byName[types.EcosystemPoolName].PercentBps)
	require.Equal(t, teamAmt, byName[types.TeamPoolName].AllocatedAmount)
	require.Equal(t, communityAmt, byName[types.CommunityPoolName].AllocatedAmount)
	require.Equal(t, ecosystemAmt, byName[types.EcosystemPoolName].AllocatedAmount)

	// 当前余额（实时 bank）与拨付一致（生态已转 depin 1e14）。
	require.Equal(t, teamAmt, byName[types.TeamPoolName].CurrentBalance)
	require.Equal(t, communityAmt, byName[types.CommunityPoolName].CurrentBalance)
	require.Equal(t, ecosystemAmt-depinSlice, byName[types.EcosystemPoolName].CurrentBalance)
}

// TestQueryRelease 验证 Query/Release：释放进度随区块时间推进（Q3/Q9）。
// 取 ReleaseSchedule 的 start/end 计算测试时间点，覆盖 cliff / 线性中点 / 结束。
func TestQueryRelease(t *testing.T) {
	k, ctx, _, _ := keepertest.TokenomicsKeeper(t)

	gs := types.DefaultGenesis()
	require.NoError(t, k.InitGenesis(ctx, *gs))

	rs := k.GetReleaseSchedule(ctx)
	start := rs.StartTime
	end := rs.EndTime
	span := end - start // 3 年线性窗口

	// cliff 边界（now == start）：vested = 0，remaining = 全额，progress = 0。
	ctxCliff := ctx.WithBlockTime(time.Unix(start, 0))
	res, err := k.Release(sdk.WrapSDKContext(ctxCliff), &types.QueryReleaseRequest{})
	require.NoError(t, err)
	require.Equal(t, uint64(0), res.Team.Vested)
	require.Equal(t, teamAmt, res.Team.Remaining)
	require.Equal(t, uint32(0), res.Team.ProgressBps)
	require.Equal(t, start, res.Team.StartTime)
	require.Equal(t, end, res.Team.EndTime)

	// 线性中点（now == start + span/2）：vested = 0.5 * teamAmt，progress = 5000。
	mid := start + span/2
	ctxMid := ctx.WithBlockTime(time.Unix(mid, 0))
	res, err = k.Release(sdk.WrapSDKContext(ctxMid), &types.QueryReleaseRequest{})
	require.NoError(t, err)
	require.Equal(t, teamAmt/2, res.Team.Vested)
	require.Equal(t, teamAmt-teamAmt/2, res.Team.Remaining)
	require.Equal(t, uint32(5000), res.Team.ProgressBps)

	// 结束后（now == end）：vested = 全额，remaining = 0，progress = 10000。
	ctxEnd := ctx.WithBlockTime(time.Unix(end, 0))
	res, err = k.Release(sdk.WrapSDKContext(ctxEnd), &types.QueryReleaseRequest{})
	require.NoError(t, err)
	require.Equal(t, teamAmt, res.Team.Vested)
	require.Equal(t, uint64(0), res.Team.Remaining)
	require.Equal(t, uint32(10000), res.Team.ProgressBps)
}
