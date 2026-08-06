package keeper_test

import (
	"math"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"mcchain/x/referral/types"
)

// setCaps 覆盖测试 keeper 的日上限参数，其余字段沿用脚手架默认值。
func setCaps(t *testing.T, k interface {
	SetParams(sdk.Context, types.Params)
	GetParams(sdk.Context) types.Params
}, ctx sdk.Context, perUser, network uint64) {
	t.Helper()
	p := k.GetParams(ctx)
	p.DailyPerUserCap = perUser
	p.DailyNetworkCap = network
	k.SetParams(ctx, p)
}

// TestCheckDailyCaps_RejectsOverCap 验证正常量级下的上限拦截。
func TestCheckDailyCaps_RejectsOverCap(t *testing.T) {
	bank := newMockRefBank()
	k, ctx := newReferralKeeper(t, bank)
	inviter := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()

	setCaps(t, k, ctx, 1_000, 10_000)

	// 恰好等于上限：允许。
	require.NoError(t, k.CheckDailyCaps(ctx, inviter, sdkmath.NewInt(1_000)))
	// 超过 1 个单位：拒绝。
	require.Error(t, k.CheckDailyCaps(ctx, inviter, sdkmath.NewInt(1_001)))

	// 累计到上限后，再来 1 个单位也必须拒绝。
	k.RecordDailyCapUsage(ctx, inviter, sdkmath.NewInt(1_000))
	require.Error(t, k.CheckDailyCaps(ctx, inviter, sdkmath.NewInt(1)))
}

// TestCheckDailyCaps_NoOverflowBypass 是本轮修复的核心回归：
//
// 旧实现用 `used + bonus.Uint64()` 与上限比较。攻击者只要让
// used + bonus 在 uint64 上回绕，比较结果就会变成一个极小的数，
// 日上限被完全绕过。改用 sdkmath.Int 任意精度比较后，回绕面消失。
func TestCheckDailyCaps_NoOverflowBypass(t *testing.T) {
	bank := newMockRefBank()
	k, ctx := newReferralKeeper(t, bank)
	inviter := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()

	setCaps(t, k, ctx, 1_000, 1_000)

	// 先把当日已用额度推到接近 uint64 上限（模拟长期饱和累加后的极端状态）。
	k.RecordDailyCapUsage(ctx, inviter, sdkmath.NewIntFromUint64(math.MaxUint64-10))

	// bonus = 100 时，旧实现 used+bonus 会回绕为 89（远小于 cap=1000）→ 放行；
	// 新实现在任意精度上比较 → 必须拒绝。
	require.Error(t, k.CheckDailyCaps(ctx, inviter, sdkmath.NewInt(100)),
		"uint64 回绕不得绕过日上限")
}

// TestCheckDailyCaps_HugeBonusDoesNotPanic 验证超出 uint64 的 bonus
// 不再触发 Int.Uint64() 的 "integer out of range" panic。
// DeliverTx 内 panic 会中止整个区块，属于停机级缺陷。
func TestCheckDailyCaps_HugeBonusDoesNotPanic(t *testing.T) {
	bank := newMockRefBank()
	k, ctx := newReferralKeeper(t, bank)
	inviter := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()

	setCaps(t, k, ctx, 1_000, 1_000)

	// 2^128：远超 uint64 值域。
	huge := sdkmath.NewIntFromBigInt(sdkmath.NewInt(1).BigInt().Lsh(sdkmath.NewInt(1).BigInt(), 128))

	require.NotPanics(t, func() {
		err := k.CheckDailyCaps(ctx, inviter, huge)
		require.Error(t, err, "超大 bonus 必须被上限拒绝")
	})

	require.NotPanics(t, func() {
		k.RecordDailyCapUsage(ctx, inviter, huge)
	})
}

// TestRecordDailyCapUsage_Saturates 验证计数器溢出时饱和到 MaxUint64，
// 而不是回绕清零。回绕清零会让攻击者在同一天内无限次领取奖励。
func TestRecordDailyCapUsage_Saturates(t *testing.T) {
	bank := newMockRefBank()
	k, ctx := newReferralKeeper(t, bank)
	inviter := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()

	setCaps(t, k, ctx, 1_000, 1_000)

	k.RecordDailyCapUsage(ctx, inviter, sdkmath.NewIntFromUint64(math.MaxUint64))
	k.RecordDailyCapUsage(ctx, inviter, sdkmath.NewIntFromUint64(math.MaxUint64))

	// 饱和后依旧撞上限（若回绕清零，这里会变成 NoError）。
	require.Error(t, k.CheckDailyCaps(ctx, inviter, sdkmath.NewInt(1)))
}

// TestCheckDailyCaps_IgnoresNonPositive 验证 nil / 零 / 负 bonus 被安全忽略。
func TestCheckDailyCaps_IgnoresNonPositive(t *testing.T) {
	bank := newMockRefBank()
	k, ctx := newReferralKeeper(t, bank)
	inviter := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()

	setCaps(t, k, ctx, 1_000, 1_000)

	require.NotPanics(t, func() {
		require.NoError(t, k.CheckDailyCaps(ctx, inviter, sdkmath.Int{}))
		require.NoError(t, k.CheckDailyCaps(ctx, inviter, sdkmath.ZeroInt()))
		require.NoError(t, k.CheckDailyCaps(ctx, inviter, sdkmath.NewInt(-5)))
		k.RecordDailyCapUsage(ctx, inviter, sdkmath.Int{})
		k.RecordDailyCapUsage(ctx, inviter, sdkmath.NewInt(-5))
	})

	// 负数不得被记成用量。
	require.NoError(t, k.CheckDailyCaps(ctx, inviter, sdkmath.NewInt(1_000)))
}

// TestParamsRejectZeroCap 验证 cap=0 无法通过 params 校验器。
// 这意味着链上「日上限」永远是 > 0 的硬约束，不存在"0 = 不限额"
// 的配置面；代码里 `if params.DailyPerUserCap > 0` 仅是纵深防御。
func TestParamsRejectZeroCap(t *testing.T) {
	p := types.DefaultParams()
	p.DailyPerUserCap = 0
	require.Error(t, p.Validate())
	p = types.DefaultParams()
	p.DailyNetworkCap = 0
	require.Error(t, p.Validate())
	require.NoError(t, types.DefaultParams().Validate())
}
