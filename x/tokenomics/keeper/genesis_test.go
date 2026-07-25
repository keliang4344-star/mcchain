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

// B1 各池分配额（umc，五池模型）：
// 设备激励 55% / 质押安全 15% / 团队 12% / 基金会 13% / 早期开发 5%。
const (
	capAmt        = uint64(1e15)                // 总量上限
	deviceAmt     = uint64(550_000_000_000_000) // 55%（设备激励，全额注入 depin）
	stakingAmt    = uint64(150_000_000_000_000) // 15%（质押安全）
	teamAmt       = uint64(120_000_000_000_000) // 12%（团队 vesting）
	foundationAmt = uint64(130_000_000_000_000) // 13%（基金会，拆分运营+vesting）
	earlyDevAmt   = uint64(50_000_000_000_000)  // 5%（早期开发，T0 全额拨付）
	foundOpsAmt   = uint64(50_000_000_000_000)  // 基金会 T0 即时解锁（运营流动，5000 万）
	foundVestAmt  = uint64(80_000_000_000_000)  // 基金会 2 年期线性释放（8000 万）
)

// TestInitGenesis 验证 tokenomics InitGenesis（B1 核心编排，R1/R2，五池模型）：
//   - 一次性铸造 cap 并记账 minted_supply = 1e15；
//   - 团队 vesting 账户余额 = 1.2e14；
//   - 设备激励池全额（5.5e14）注入 depin 模块账户；
//   - 质押安全模块账户 == 1.5e14；
//   - 团队 vesting 账户 == 1.2e14；早期开发地址 == 0.5e14（T0 全额）；
//   - 基金会拆分：运营流动地址 == 0.5e14（T0 即时）+ 2 年期线性 vesting 地址 == 0.8e14；
//   - 团队 vesting 账户已写入状态；ReleaseSchedule 已写入且 start==cliff。
func TestInitGenesis(t *testing.T) {
	k, ctx, bk, ak := keepertest.TokenomicsKeeper(t)

	gs := types.DefaultGenesis()
	require.NoError(t, k.InitGenesis(ctx, *gs))
	genesisTime := ctx.BlockTime()

	// ① 累计已发行 == 总量上限（单一发行入口，R1）。
	require.Equal(t, sdk.NewInt(int64(capAmt)), k.GetMintedSupply(ctx))

	// ② 团队 vesting 账户余额 == 1.2e14。
	teamBal := bk.GetBalance(ctx, types.TeamAddress, types.DefaultDenom)
	require.Equal(t, teamAmt, uint64(teamBal.Amount.Int64()))

	// ③ 设备激励池全额注入 depin 模块账户 == 5.5e14。
	depinAddr := authtypes.NewModuleAddress(types.DepinModuleName)
	depinBal := bk.GetBalance(ctx, depinAddr, types.DefaultDenom)
	require.Equal(t, deviceAmt, uint64(depinBal.Amount.Int64()))

	// ④ 质押安全模块账户余额 == 1.5e14。
	stakingAddr := authtypes.NewModuleAddress(types.StakingSecurityPoolName)
	stakingBal := bk.GetBalance(ctx, stakingAddr, types.DefaultDenom)
	require.Equal(t, stakingAmt, uint64(stakingBal.Amount.Int64()))

	// ⑤ 基金会拆分：运营流动地址 == 0.5e14（T0 即时解锁）。
	foundOpsBal := bk.GetBalance(ctx, types.FoundationOpsAddress, types.DefaultDenom)
	require.Equal(t, foundOpsAmt, uint64(foundOpsBal.Amount.Int64()))

	// ⑥ 早期开发地址 == 0.5e14（T0 全额拨付，无锁仓）。
	earlyDevBal := bk.GetBalance(ctx, types.EarlyDevAddress, types.DefaultDenom)
	require.Equal(t, earlyDevAmt, uint64(earlyDevBal.Amount.Int64()))

	// ⑦ 基金会 2 年期线性释放 vesting 账户 == 0.8e14，且为连续锁仓账户、endTime = genesis + 2yr。
	foundVestBal := bk.GetBalance(ctx, types.FoundationVestingAddress, types.DefaultDenom)
	require.Equal(t, foundVestAmt, uint64(foundVestBal.Amount.Int64()))
	vestAcc := ak.GetAccount(ctx, types.FoundationVestingAddress)
	require.NotNil(t, vestAcc, "foundation vesting account must be set")
	cva, isVesting := vestAcc.(*vestingtypes.ContinuousVestingAccount)
	require.True(t, isVesting, "foundation vesting account must be a ContinuousVestingAccount")
	require.Equal(t, genesisTime.AddDate(2, 0, 0).Unix(), cva.EndTime, "foundation vesting end must be genesis + 2yr")

	// ⑧ 团队多签 vesting 账户已写入状态，且为连续锁仓账户。
	acc := ak.GetAccount(ctx, types.TeamAddress)
	require.NotNil(t, acc, "team vesting account must be set")
	_, isTeamVesting := acc.(*vestingtypes.ContinuousVestingAccount)
	require.True(t, isTeamVesting, "team account must be a ContinuousVestingAccount")

	// ⑨ ReleaseSchedule 已写入：地址=团队、锁仓额=1.2e14、start==cliff（1 年 cliff）。
	rs := k.GetReleaseSchedule(ctx)
	require.Equal(t, types.TeamAddress.String(), rs.TeamAddress)
	require.Equal(t, teamAmt, rs.TotalLocked)
	require.Equal(t, rs.StartTime, rs.CliffTime, "cliff_time must equal start_time (1yr cliff, 0 release)")
	require.Greater(t, rs.EndTime, rs.StartTime, "end_time must be after start_time")
}
