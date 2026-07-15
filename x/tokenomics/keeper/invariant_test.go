package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	tokenomicskeeper "mcchain/x/tokenomics/keeper"
	keepertest "mcchain/testutil/keeper"
	"mcchain/x/tokenomics/types"
)

// TestMintedSupplyInvariant 验证 R1 不变量：累计已发行量不超过总量上限。
// 正常（minted == cap）不破坏；构造超 cap（cap+1）应触发不变量。
func TestMintedSupplyInvariant(t *testing.T) {
	k, ctx, _, _ := keepertest.TokenomicsKeeper(t)

	gs := types.DefaultGenesis()
	require.NoError(t, k.InitGenesis(ctx, *gs))

	inv := tokenomicskeeper.MintedSupplyInvariant(*k)
	desc, broken := inv(ctx)
	require.False(t, broken, desc)

	// 构造超 cap：直接篡改 minted_supply = cap + 1。
	k.SetMintedSupply(ctx, sdk.NewInt(int64(capAmt+1)))
	desc, broken = inv(ctx)
	require.True(t, broken, "minted > cap must break invariant")
	require.Contains(t, desc, "exceeds total supply cap")
}

// TestPoolSumInvariant 验证 R2 不变量（会计口径）：各池 allocated_amount 之和 == minted_supply。
// 正常（和为 cap）不破坏；构造池和 != minted 应触发不变量。
func TestPoolSumInvariant(t *testing.T) {
	k, ctx, _, _ := keepertest.TokenomicsKeeper(t)

	gs := types.DefaultGenesis()
	require.NoError(t, k.InitGenesis(ctx, *gs))

	inv := tokenomicskeeper.PoolSumInvariant(*k)
	desc, broken := inv(ctx)
	require.False(t, broken, desc)

	// 构造池和 != minted：篡改分配记账（仅保留 team，金额远小于 cap）。
	k.SetAllocations(ctx, []types.PoolAllocation{
		{Name: types.TeamPoolName, PercentBps: types.TeamPercentBps, AllocatedAmount: 100, Address: types.TeamAddress.String()},
	})
	desc, broken = inv(ctx)
	require.True(t, broken, "pool sum != minted must break invariant")
	require.Contains(t, desc, "sum of allocations")
}
