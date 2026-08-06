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

// setLevelRates 覆盖三级费率参数。
func setLevelRates(t *testing.T, k interface {
	SetParams(sdk.Context, types.Params)
	GetParams(sdk.Context) types.Params
}, ctx sdk.Context, l1, l2, l3 uint32) {
	t.Helper()
	p := k.GetParams(ctx)
	p.Level1RewardRateBps = l1
	p.Level2RewardRateBps = l2
	p.Level3RewardRateBps = l3
	k.SetParams(ctx, p)
}

func newAddr() string {
	return sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
}

// TestTrackReward_M4_ZeroMiddleRateDoesNotMisroute 是 M-4 修复的回归：
//
// 旧实现 `if rate == 0 { continue }` 不推进 currentInvitee，当 Level2 被治理
// 调为 0 时，Level3 的 2% 会错误发放给二代祖先（而非三代祖先）。
// 修复后：rate=0 的层级同样沿链上移一级，各祖先拿到的是自己的费率。
func TestTrackReward_M4_ZeroMiddleRateDoesNotMisroute(t *testing.T) {
	bank := newMockRefBank()
	k, ctx := newReferralKeeper(t, bank)

	// 三级费率：10% / 0%（模拟治理关闭二代） / 2%
	setLevelRates(t, k, ctx, 1000, 0, 200)

	// 生态账户预拨资金（推荐奖励领取来源）
	bank.mint(authtypes.NewModuleAddress(types.EcosystemModuleAccount).String(),
		sdk.NewCoins(sdk.NewCoin("umc", sdkmath.NewInt(1_000_000_000))))

	a := newAddr() // 一代祖先
	b := newAddr() // 二代祖先
	c := newAddr() // 三代祖先
	d := newAddr() // 被推荐人

	// 建链：c → b → a → d（d 由 a 推荐，a 由 b 推荐，b 由 c 推荐）
	_, err := k.CreateReferral(ctx, c, b, "CCODE")
	require.NoError(t, err)
	_, err = k.CreateReferral(ctx, b, a, "BCODE")
	require.NoError(t, err)
	_, err = k.CreateReferral(ctx, a, d, "ACODE")
	require.NoError(t, err)

	// 被推荐人 d 赚 1,000,000 umc
	err = k.TrackReward(ctx, d, sdkmath.NewInt(1_000_000))
	require.NoError(t, err)

	// 期望：a 得一代 10% = 100,000；c 得三代 2% = 20,000（跳过 rate=0 的二代）；
	// b 分文不得（二代费率被治理关闭，不得错配成三代费率发给 b）。
	require.Equal(t, int64(100_000), k.GetPendingRewards(ctx, a).Amount.Int64(), "a 应得一代 10%")
	require.Equal(t, int64(0), k.GetPendingRewards(ctx, b).Amount.Int64(), "b 二代费率=0 不得得钱")
	require.Equal(t, int64(20_000), k.GetPendingRewards(ctx, c).Amount.Int64(), "c 应得三代 2%（不得被 b 截胡）")
}

// TestTrackReward_ChainTerminatesAtMaxDepth 验证超出三级时链正常截断，
// 不向第四代祖先发放任何奖励。
func TestTrackReward_ChainTerminatesAtMaxDepth(t *testing.T) {
	bank := newMockRefBank()
	k, ctx := newReferralKeeper(t, bank)

	bank.mint(authtypes.NewModuleAddress(types.EcosystemModuleAccount).String(),
		sdk.NewCoins(sdk.NewCoin("umc", sdkmath.NewInt(1_000_000_000))))

	// 四级链：e → d → c → b → a(invitee 的推荐人)
	// 注意 CreateReferral(inviter, invitee) 语义：inviter 是推荐人（上级）。
	a, b, c, d, e := newAddr(), newAddr(), newAddr(), newAddr(), newAddr()
	_, err := k.CreateReferral(ctx, e, d, "E")
	require.NoError(t, err)
	_, err = k.CreateReferral(ctx, d, c, "D")
	require.NoError(t, err)
	_, err = k.CreateReferral(ctx, c, b, "C")
	require.NoError(t, err)
	_, err = k.CreateReferral(ctx, b, a, "B")
	require.NoError(t, err)
	invitee := newAddr()
	_, err = k.CreateReferral(ctx, a, invitee, "A")
	require.NoError(t, err)

	require.NoError(t, k.TrackReward(ctx, invitee, sdkmath.NewInt(1_000_000)))

	// 三级内：a=10% 100k、b=5% 50k、c=2% 20k
	require.Equal(t, int64(100_000), k.GetPendingRewards(ctx, a).Amount.Int64())
	require.Equal(t, int64(50_000), k.GetPendingRewards(ctx, b).Amount.Int64())
	require.Equal(t, int64(20_000), k.GetPendingRewards(ctx, c).Amount.Int64())
	// 第四代 e 与中间节点 d 分文不得（超出 MaxReferralDepth=3）
	require.Equal(t, int64(0), k.GetPendingRewards(ctx, d).Amount.Int64())
	require.Equal(t, int64(0), k.GetPendingRewards(ctx, e).Amount.Int64())
}
