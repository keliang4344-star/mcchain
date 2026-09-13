package keeper_test

import (
	"strings"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	keepertest "mcchain/testutil/keeper"
	"mcchain/x/tokenomics/keeper"
	"mcchain/x/tokenomics/types"
)

const auditPoolName = "device_incentive"

// setupAuditKeeper 造一个最小可控的经济状态：单池记账 1000 umc、累计发行 1000 umc，
// 池地址取该池模块账户地址（与实际部署一致）。
func setupAuditKeeper(t *testing.T) (*keeper.Keeper, sdk.Context, *keepertest.MockBankKeeper) {
	t.Helper()
	k, ctx, bk, _ := keepertest.TokenomicsKeeper(t)
	k.SetAllocations(ctx, []types.PoolAllocation{{
		Name:            auditPoolName,
		PercentBps:      10000,
		AllocatedAmount: 1000,
		Address:         authtypes.NewModuleAddress(auditPoolName).String(),
	}})
	k.SetMintedSupply(ctx, sdk.NewInt(1000))
	return k, ctx, bk
}

// TestAuditEconomyConsistentBaseline 基线：记账自洽且余额不超记账额时，
// 巡检应报告一致，并把各池的已支出量算对。
func TestAuditEconomyConsistentBaseline(t *testing.T) {
	k, ctx, _ := setupAuditKeeper(t)

	rep := k.AuditEconomy(ctx)
	require.True(t, rep.Consistent(), "基线状态应一致: %+v", rep.Issues)
	require.Equal(t, "1000", rep.MintedSupply)
	require.Equal(t, "1000", rep.AllocatedSum)
	require.Len(t, rep.Pools, 1)
	require.False(t, rep.Pools[0].OverAllocated)
	require.Equal(t, "0", rep.Pools[0].Balance, "池未被拨付时余额为 0")
	require.Equal(t, "1000", rep.Pools[0].Spent, "未拨付时已支出应为全额待发")
}

// TestAuditEconomyDetectsOverAllocation 池余额高于记账额时必须被标记为待核查。
//
// 注意这里刻意**不**把它判成硬违规：外部向池地址直接转账也会造成同样结果，
// 而这些检查一旦做成 crisis 不变量就会因无关行为停链。
func TestAuditEconomyDetectsOverAllocation(t *testing.T) {
	k, ctx, bk := setupAuditKeeper(t)

	require.NoError(t, bk.MintCoins(ctx, auditPoolName,
		sdk.NewCoins(sdk.NewInt64Coin(types.DefaultDenom, 1001))))

	rep := k.AuditEconomy(ctx)
	require.False(t, rep.Consistent(), "余额超记账额应进入待核查项")
	require.True(t, rep.Pools[0].OverAllocated)
	require.Equal(t, "1001", rep.Pools[0].Balance)
	require.Contains(t, rep.Issues[0], "高于记账分配额")
}

// TestAuditEconomyDetectsAllocationMismatch 五池记账合计与累计发行量不等时，
// 必须报告会计口径恒等式被破坏。
func TestAuditEconomyDetectsAllocationMismatch(t *testing.T) {
	k, ctx, _ := setupAuditKeeper(t)

	// 篡改分配记账，使五池合计 != minted。
	k.SetAllocations(ctx, []types.PoolAllocation{{
		Name:            auditPoolName,
		PercentBps:      10000,
		AllocatedAmount: 999,
		Address:         authtypes.NewModuleAddress(auditPoolName).String(),
	}})

	rep := k.AuditEconomy(ctx)
	require.False(t, rep.Consistent())
	require.Equal(t, "999", rep.AllocatedSum)
	require.True(t, hasIssue(rep, "五池记账合计"), "应报告记账合计与发行量不符: %+v", rep.Issues)
}

// TestAuditEconomyDetectsOverMint 累计发行量突破总量上限时必须报告。
func TestAuditEconomyDetectsOverMint(t *testing.T) {
	k, ctx, _ := setupAuditKeeper(t)

	k.SetMintedSupply(ctx, sdk.NewIntFromUint64(types.TotalSupplyCap+1))

	rep := k.AuditEconomy(ctx)
	require.False(t, rep.Consistent())
	require.True(t, hasIssue(rep, "超过总量上限"), "应报告发行量突破上限: %+v", rep.Issues)
}

// TestAuditEconomyReportsBlackHole 黑洞余额必须出现在报告中——它是累计销毁量的
// 权威口径，任何人可用标准 bank 查询独立核对。
func TestAuditEconomyReportsBlackHole(t *testing.T) {
	k, ctx, _ := setupAuditKeeper(t)

	rep := k.AuditEconomy(ctx)
	require.Equal(t, "0", rep.BlackHoleBalance, "未发生销毁时黑洞余额为 0")
	require.NotEmpty(t, rep.FeeCollectorBalance, "手续费归集账户余额应被报告（可为 0）")
}

// hasIssue 判断报告中的待核查项是否包含指定子串。
func hasIssue(rep keeper.EconomyAuditReport, sub string) bool {
	for _, issue := range rep.Issues {
		if strings.Contains(issue, sub) {
			return true
		}
	}
	return false
}
