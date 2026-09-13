package keeper_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	"mcchain/x/referral/types"
)

// build10LevelChain 构造 10 层推荐链：invitee(0) ← a1 ← a2 ← ... ← a10
// （a1 直接推荐 invitee，a10 是第 10 代祖先）。
func build10LevelChain(t *testing.T, k interface {
	CreateReferral(ctx sdk.Context, inviter, invitee, inviteCode string) (uint64, error)
}, ctx sdk.Context) []string {
	t.Helper()
	addrs := make([]string, 0, 11)
	for i := 0; i <= 10; i++ {
		addrs = append(addrs, sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String())
	}
	// addrs[0] = 被推荐人；addrs[1..10] = 第 1..10 代祖先
	for i := 1; i <= 10; i++ {
		_, err := k.CreateReferral(ctx, addrs[i], addrs[i-1], "T10")
		require.NoError(t, err)
	}
	return addrs
}

func fundEcosystem(t *testing.T, bank *mockRefBank) {
	t.Helper()
	bank.mint(authtypes.NewModuleAddress(types.EcosystemModuleAccount).String(),
		sdk.NewCoins(sdk.NewCoin("umc", sdkmath.NewInt(10_000_000_000))))
}

// TestTrackReward_TenTier_AllNodes 验证 10 级权重表逐层到账：
// 1000/500/300/200/100/50/50/50/50/50 bps，合计 23.5%。
func TestTrackReward_TenTier_AllNodes(t *testing.T) {
	bank := newMockRefBank()
	fundEcosystem(t, bank)
	k, ctx := newReferralKeeper(t, bank)

	addrs := build10LevelChain(t, k, ctx)
	invitee, a1 := addrs[0], addrs[1]

	require.NoError(t, k.TrackReward(ctx, invitee, sdkmath.NewInt(10_000_000)))

	// 期望各代到账：10% / 5% / 3% / 2% / 1% / 0.5%×5
	want := []int64{1_000_000, 500_000, 300_000, 200_000, 100_000, 50_000, 50_000, 50_000, 50_000, 50_000}
	for i := 0; i < 10; i++ {
		require.Equal(t, want[i], k.GetPendingRewards(ctx, addrs[i+1]).Amount.Int64(),
			"第 %d 代权重错误", i+1)
	}
	_ = a1
}

// TestTrackReward_NonNodeOnlyFiveTiers 验证 10 代分级：
// 非挖矿节点祖先最多享受前 5 代；第 6 代起只有挖矿节点才能获得。
func TestTrackReward_NonNodeOnlyFiveTiers(t *testing.T) {
	bank := newMockRefBank()
	fundEcosystem(t, bank)

	// 仅 a6..a10（第 6-10 代祖先）是挖矿节点；a1..a5 与 invitee 非节点。
	active := map[string]bool{}
	k, ctx := newReferralKeeperWithNodes(t, bank, active)

	addrs := build10LevelChain(t, k, ctx)
	// 手工指定节点集合：第 6-10 代祖先为节点
	active[addrs[6]] = true
	active[addrs[7]] = true
	active[addrs[8]] = true
	active[addrs[9]] = true
	active[addrs[10]] = true

	invitee := addrs[0]
	require.NoError(t, k.TrackReward(ctx, invitee, sdkmath.NewInt(10_000_000)))

	// a1..a5 非节点：享受前 5 代（10/5/3/2/1%）
	wantFront := []int64{1_000_000, 500_000, 300_000, 200_000, 100_000}
	for i := 0; i < 5; i++ {
		require.Equal(t, wantFront[i], k.GetPendingRewards(ctx, addrs[i+1]).Amount.Int64(),
			"非节点第 %d 代应得", i+1)
	}
	// a6..a10 节点：享受第 6-10 代（各 0.5%）
	for i := 5; i < 10; i++ {
		require.Equal(t, int64(50_000), k.GetPendingRewards(ctx, addrs[i+1]).Amount.Int64(),
			"节点第 %d 代应得 0.5%", i+1)
	}
}

// TestTrackReward_NonNodeAncestorBlocksDeepTiers 验证：当链条深处出现非节点祖先时，
// 该祖先不得分，但链条继续向上（节点上级仍可拿到更深层收益）。
func TestTrackReward_NonNodeAncestorBlocksDeepTiers(t *testing.T) {
	bank := newMockRefBank()
	fundEcosystem(t, bank)

	active := map[string]bool{}
	k, ctx := newReferralKeeperWithNodes(t, bank, active)

	addrs := build10LevelChain(t, k, ctx)
	// 只有 a10（最深的第 10 代）是节点，其余全部非节点
	active[addrs[10]] = true

	invitee := addrs[0]
	require.NoError(t, k.TrackReward(ctx, invitee, sdkmath.NewInt(10_000_000)))

	// a1..a5 非节点：前 5 代正常到账
	for i := 0; i < 5; i++ {
		want := []int64{1_000_000, 500_000, 300_000, 200_000, 100_000}[i]
		require.Equal(t, want, k.GetPendingRewards(ctx, addrs[i+1]).Amount.Int64(), "前段第 %d 代", i+1)
	}
	// a6..a9 非节点：第 6-9 代不得分
	for i := 5; i < 9; i++ {
		require.Equal(t, int64(0), k.GetPendingRewards(ctx, addrs[i+1]).Amount.Int64(),
			"非节点第 %d 代不得分", i+1)
	}
	// a10 节点：第 10 代 0.5% 正常到账
	require.Equal(t, int64(50_000), k.GetPendingRewards(ctx, addrs[10]).Amount.Int64(), "节点第 10 代")
}

// TestTrackReward_Depth11Terminates 验证超过 10 代的链条被截断（第 11 代祖先无收益）。
func TestTrackReward_Depth11Terminates(t *testing.T) {
	bank := newMockRefBank()
	fundEcosystem(t, bank)
	k, ctx := newReferralKeeper(t, bank)

	addrs := build10LevelChain(t, k, ctx)
	// 追加第 11 代祖先
	a11 := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
	_, err := k.CreateReferral(ctx, a11, addrs[10], "T11")
	require.NoError(t, err)

	require.NoError(t, k.TrackReward(ctx, addrs[0], sdkmath.NewInt(10_000_000)))

	require.Equal(t, int64(0), k.GetPendingRewards(ctx, a11).Amount.Int64(), "第 11 代不得分")
	require.Equal(t, int64(50_000), k.GetPendingRewards(ctx, addrs[10]).Amount.Int64(), "第 10 代 0.5%")
}
