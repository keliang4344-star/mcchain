package keeper_test

import (
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	"mcchain/x/referral/keeper"
	"mcchain/x/referral/types"
)

// TestEffectiveNetworkCap_PhaseMultiplierIsPercent 锁定阶段倍数的单位口径。
//
// 回归背景：Phase1MultiplierBps 的语义是百分数（100 = 1 倍、200 = 2 倍），
// 但早期实现按万分比除以 10000 换算，导致「不加速」的 100 被解释成 0.01 倍，
// 治理参数日上限被静默削掉 100 倍（param_cap=10000 → effective_cap=100）。
// 该缺陷会让预算看着还有、实际几乎发不出去，且不产生任何报错。
func TestEffectiveNetworkCap_PhaseMultiplierIsPercent(t *testing.T) {
	bank := newMockRefBank()
	k, ctx := newReferralKeeper(t, bank)
	setCaps(t, k, ctx, 1_000_000, 10_000)

	// 冷启动期：倍数 200 = 2 倍。
	k.SetReleaseSchedule(ctx, keeper.ReleaseSchedule{
		StartTimeUnix:       1, // 早于测试 ctx 的区块时间（零值 header），必落在 Phase1 内
		TotalDays:           4015,
		Phase1Days:          365,
		Phase1MultiplierBps: 200,
	})

	eff, mult := k.EffectiveNetworkCap(ctx)
	require.EqualValues(t, 200, mult)
	require.Equal(t, sdkmath.NewInt(20_000), eff,
		"倍数 200 必须是 2 倍（20000），而不是 200 或 2")

	// 倍数 100 = 不加速，等于治理参数本身。
	k.SetReleaseSchedule(ctx, keeper.ReleaseSchedule{
		StartTimeUnix:       1,
		TotalDays:           4015,
		Phase1Days:          365,
		Phase1MultiplierBps: 100,
	})
	eff, _ = k.EffectiveNetworkCap(ctx)
	require.Equal(t, sdkmath.NewInt(10_000), eff, "100 必须等于治理参数上限（1 倍）")
}

// TestEffectiveNetworkCap_PacedByRemainingBudget 验证配速项在预算变小后自动收紧上限，
// 保证预算必然撑满整个释放窗口（这是消除「提前 25% 停摆」的关键性质）。
func TestEffectiveNetworkCap_PacedByRemainingBudget(t *testing.T) {
	bank := newMockRefBank()
	k, ctx := newReferralKeeper(t, bank)
	setCaps(t, k, ctx, 1_000_000, 1_000_000_000)

	// 把生态账户余额换成「刚好只够按配速再发几天」的小额。
	eco := authtypes.NewModuleAddress(types.EcosystemModuleAccount)
	require.True(t, bank.GetBalance(ctx, eco, types.BaseDenom).Amount.IsPositive(),
		"测试前置：生态账户应已由 keeper 脚手架注资")
	// 余额 4015 × 1000 = 4,015,000 umc，窗口 4015 天 → 配速上限恰为 1000/天。
	bank.balances[eco.String()] = sdk.NewCoins(sdk.NewCoin(types.BaseDenom, sdkmath.NewInt(4_015_000)))

	k.SetReleaseSchedule(ctx, keeper.ReleaseSchedule{
		StartTimeUnix:       1,
		TotalDays:           4015,
		Phase1Days:          0, // 不做阶段加速，隔离出纯配速
		Phase1MultiplierBps: 100,
	})

	eff, _ := k.EffectiveNetworkCap(ctx)
	require.Equal(t, sdkmath.NewInt(1000), eff, "配速项应把上限收紧到 剩余预算/剩余天数")
}

// TestEffectiveNetworkCap_ZeroWhenBudgetExhausted 预算耗尽时必须返回 0，
// 让调用方走「预算耗尽」分支（发事件 + 顺延），而不是继续按参数上限发放。
func TestEffectiveNetworkCap_ZeroWhenBudgetExhausted(t *testing.T) {
	bank := newMockRefBank()
	k, ctx := newReferralKeeper(t, bank)

	eco := authtypes.NewModuleAddress(types.EcosystemModuleAccount)
	bank.balances[eco.String()] = sdk.NewCoins()

	eff, _ := k.EffectiveNetworkCap(ctx)
	require.True(t, eff.IsZero(), "无余额时有效上限必须为 0")

	status := k.GetBudgetStatus(ctx)
	require.True(t, status.BudgetExhausted)
	require.True(t, status.Remaining.IsZero())
}

// TestEnsureReleaseScheduleStartIsIdempotent 释放窗口起点只能钉一次：
// 重复调用（每个区块都会调用）不得把窗口重置，否则配速永远停在第一天。
func TestEnsureReleaseScheduleStartIsIdempotent(t *testing.T) {
	bank := newMockRefBank()
	k, ctx := newReferralKeeper(t, bank)

	// 测试 ctx 的区块时间为零值（Unix 为负），此处显式给一个真实时间。
	t1 := time.Unix(1_700_000_000, 0).UTC()
	c1 := ctx.WithBlockTime(t1)
	k.EnsureReleaseScheduleStart(c1)
	first := k.GetReleaseSchedule(c1)
	require.Equal(t, t1.Unix(), first.StartTimeUnix)

	c2 := ctx.WithBlockTime(t1.Add(48 * time.Hour))
	k.EnsureReleaseScheduleStart(c2)
	second := k.GetReleaseSchedule(c2)
	require.Equal(t, first.StartTimeUnix, second.StartTimeUnix, "已初始化的窗口不得被重置")

	require.Equal(t, uint64(4015-2), k.DaysRemaining(c2), "48 小时后剩余天数应减 2")
}

// TestDeferRewardLedgerAccumulates 延后账本必须累加而不是覆盖：
// 同一天内多笔触顶返佣都记在同一个桶里，否则用户权益会被静默丢弃。
func TestDeferRewardLedgerAccumulates(t *testing.T) {
	bank := newMockRefBank()
	k, ctx := newReferralKeeper(t, bank)
	inviter := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()

	k.DeferReward(ctx, inviter, sdkmath.NewInt(30))
	k.DeferReward(ctx, inviter, sdkmath.NewInt(12))
	require.Equal(t, sdkmath.NewInt(42), k.DeferredTotal(ctx))

	n, truncated := k.DeferredEntryCount(ctx)
	require.Equal(t, 1, n, "同一 inviter 同一天只应有一个桶")
	require.False(t, truncated)
}

// 回归测试：阶段倍数必须作用在「基准额度」上。
//
// 注意：阶段倍数只乘在「治理参数上限」上是不够的，而 min(参数上限, 配速项)
// 在真实参数下总是取配速项（参数 20,600 MC/日 vs 配速 ≈ 20,548 MC/日），
// 于是倍数对结果没有任何影响，文档所述「首年日上限约 41,096 MC、前置投放约
// 15M」也就无从生效。
//
// 因此倍数必须作用在「基准额度 = min(参数上限, 配速项)」上。
func TestEffectiveNetworkCap_PhaseMultiplierAppliesToPacing(t *testing.T) {
	bank := newMockRefBank()
	k, ctx := newReferralKeeper(t, bank)

	// 生产口径：治理参数 20,600 MC/日，略高于配速项 → 配速项是真正的约束。
	const networkCap = uint64(20_600_000_000)
	setCaps(t, k, ctx, 1_000_000_000_000, networkCap)

	eco := authtypes.NewModuleAddress(types.EcosystemModuleAccount)
	// 预算 82.5M MC = 8.25e13 umc，窗口 4015 天 → 配速 ≈ 20,548 MC/日。
	const budget = int64(82_500_000_000_000)
	bank.balances[eco.String()] = sdk.NewCoins(sdk.NewCoin(types.BaseDenom, sdkmath.NewInt(budget)))

	k.SetReleaseSchedule(ctx, keeper.ReleaseSchedule{
		StartTimeUnix:       1, // 早于零值 header 的区块时间 → 必落在 Phase1 内
		TotalDays:           4015,
		Phase1Days:          365,
		Phase1MultiplierBps: 200,
	})

	eff, mult := k.EffectiveNetworkCap(ctx)
	require.EqualValues(t, 200, mult)

	paced := sdkmath.NewInt(budget).Quo(sdkmath.NewInt(4015))
	require.Equal(t, paced.MulRaw(2), eff,
		"阶段倍数必须放大配速额度（paced×2），否则冷启动前置投放形同虚设")
	require.True(t, eff.GT(sdkmath.NewInt(int64(networkCap))),
		"前置投放期日上限应高于治理参数上限（这是「前置投放」的含义）")

	// 倍数回到 100（不加速）→ 应等于纯配速额度
	k.SetReleaseSchedule(ctx, keeper.ReleaseSchedule{
		StartTimeUnix: 1, TotalDays: 4015, Phase1Days: 365, Phase1MultiplierBps: 100,
	})
	eff, _ = k.EffectiveNetworkCap(ctx)
	require.Equal(t, paced, eff, "100 = 不加速，应严格等于配速额度")
}

// 回归测试：配速上限必须扣除已承诺的负债。
//
// 注意：返佣在计提时只登记负债（pending rewards），真正扣减生态账户余额
// 发生在 ClaimRewards。配速上限若只看账户余额，就会把「已承诺的钱」当成
// 「还能再花的钱」：用户长期不领取时，每天都能按同一速率继续承诺，
// 累计承诺额可以超过预算总额，最后一批用户领取时账户已空。
func TestEffectiveNetworkCap_ExcludesCommittedLiabilities(t *testing.T) {
	bank := newMockRefBank()
	k, ctx := newReferralKeeper(t, bank)

	const budget = int64(4_015_000)
	setCaps(t, k, ctx, 1_000_000_000_000, 1_000_000_000_000)
	eco := authtypes.NewModuleAddress(types.EcosystemModuleAccount)
	bank.balances[eco.String()] = sdk.NewCoins(sdk.NewCoin(types.BaseDenom, sdkmath.NewInt(budget)))

	k.SetReleaseSchedule(ctx, keeper.ReleaseSchedule{
		StartTimeUnix:       1,
		TotalDays:           4015,
		Phase1Days:          0, // 隔离出纯配速
		Phase1MultiplierBps: 100,
	})

	// 基线：4,015,000 / 4015 = 1000
	before, _ := k.EffectiveNetworkCap(ctx)
	require.Equal(t, sdkmath.NewInt(1000), before)
	require.True(t, k.PendingRewardTotal(ctx).IsZero(), "前置条件：尚无已承诺负债")

	// 建立推荐关系并计提一笔返佣（形成 pending 负债，但用户不领取）
	inviter := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
	invitee := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
	_, err := k.CreateReferral(ctx, inviter, invitee, "")
	require.NoError(t, err)
	require.NoError(t, k.TrackReward(ctx, invitee, sdkmath.NewInt(2_000)))

	committed := k.PendingRewardTotal(ctx)
	require.True(t, committed.IsPositive(), "计提后必须形成已承诺负债")

	// 账户余额没变，但可用预算必须相应下降
	require.Equal(t, sdkmath.NewInt(budget), k.EcosystemBalance(ctx),
		"计提不移动币，余额不变（这正是缺陷的成因）")
	require.Equal(t, committed, k.CommittedLiabilities(ctx))
	require.Equal(t, sdkmath.NewInt(budget).Sub(committed), k.AvailableBudget(ctx))

	after, _ := k.EffectiveNetworkCap(ctx)
	expected := sdkmath.NewInt(budget).Sub(committed).Quo(sdkmath.NewInt(4015))
	require.Equal(t, expected, after,
		"已承诺的 pending 必须从配速基数里扣除，否则累计承诺额可以超过预算")

	status := k.GetBudgetStatus(ctx)
	require.Equal(t, committed, status.PendingTotal)
	require.Equal(t, committed, status.Committed)
	require.Equal(t, sdkmath.NewInt(budget).Sub(committed), status.Available)
}
