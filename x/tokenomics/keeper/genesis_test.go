package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	"github.com/stretchr/testify/require"

	keepertest "mcchain/testutil/keeper"
	"mcchain/x/tokenomics/types"
)

// B1 各池分配额（umc）：团队 15% / 社区 35% / 生态 50%（Q2）。
const (
	capAmt       = uint64(1e15)   // 总量上限
	teamAmt      = uint64(1.5e14) // 15% of 1e15
	communityAmt = uint64(3.5e14) // 35%
	ecosystemAmt = uint64(5e14)   // 50%（分配额）
	depinSlice   = uint64(1e14)   // 生态→depin 的 InitialPool 切片（Q4/Q7）
)

// TestInitGenesis 验证 tokenomics InitGenesis（B1 核心编排，R1/R2）：
//   - 一次性铸造 cap 并记账 minted_supply = 1e15；
//   - 团队 vesting 账户余额 = 1.5e14，社区 = 3.5e14，生态 = 5e14 - 1e14 = 4e14；
//   - depin 模块账户收到 InitialPool 切片 = 1e14；
//   - 团队 vesting 账户已写入状态；ReleaseSchedule 已写入且 start==cliff。
func TestInitGenesis(t *testing.T) {
	k, ctx, bk, ak := keepertest.TokenomicsKeeper(t)

	gs := types.DefaultGenesis()
	require.NoError(t, k.InitGenesis(ctx, *gs))

	// ① 累计已发行 == 总量上限（单一发行入口，R1）。
	require.Equal(t, sdk.NewInt(int64(capAmt)), k.GetMintedSupply(ctx))

	// ② 团队 vesting 账户余额 == 1.5e14。
	teamBal := bk.GetBalance(ctx, types.TeamAddress, types.DefaultDenom)
	require.Equal(t, teamAmt, uint64(teamBal.Amount.Int64()))

	// ③ 社区模块账户余额 == 3.5e14。
	commAddr := authtypes.NewModuleAddress(types.CommunityPoolName)
	commBal := bk.GetBalance(ctx, commAddr, types.DefaultDenom)
	require.Equal(t, communityAmt, uint64(commBal.Amount.Int64()))

	// ④ 生态模块账户余额 == 5e14 分配额 - 1e14(转 depin) == 4e14。
	ecoAddr := authtypes.NewModuleAddress(types.EcosystemPoolName)
	ecoBal := bk.GetBalance(ctx, ecoAddr, types.DefaultDenom)
	require.Equal(t, ecosystemAmt-depinSlice, uint64(ecoBal.Amount.Int64()))

	// ⑤ depin 模块账户收到 InitialPool 切片 == 1e14（Q4/Q7）。
	depinAddr := authtypes.NewModuleAddress("depin")
	depinBal := bk.GetBalance(ctx, depinAddr, types.DefaultDenom)
	require.Equal(t, depinSlice, uint64(depinBal.Amount.Int64()))

	// ⑥ 团队多签 vesting 账户已写入状态，且为连续锁仓账户。
	acc := ak.GetAccount(ctx, types.TeamAddress)
	require.NotNil(t, acc, "team vesting account must be set")
	_, isVesting := acc.(*vestingtypes.ContinuousVestingAccount)
	require.True(t, isVesting, "team account must be a ContinuousVestingAccount")

	// ⑦ ReleaseSchedule 已写入：地址=团队、锁仓额=1.5e14、start==cliff（1 年 cliff）。
	rs := k.GetReleaseSchedule(ctx)
	require.Equal(t, types.TeamAddress.String(), rs.TeamAddress)
	require.Equal(t, teamAmt, rs.TotalLocked)
	require.Equal(t, rs.StartTime, rs.CliffTime, "cliff_time must equal start_time (1yr cliff, 0 release)")
	require.Greater(t, rs.EndTime, rs.StartTime, "end_time must be after start_time")
}
