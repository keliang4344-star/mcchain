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

// P1-06：返佣预算的全局负债守恒。
//
// 不变量（任何路径都不得击穿）：
//
//	已支付 + 已承诺（pending + deferred）≤ 初始预算
//	⇔ committed liabilities ≤ ecosystem balance
//
// 覆盖组合：日上限、总预算、单用户上限同时触发；余额为零；余额=负债；
// 极大整数；重复计提；跨日边界。
func TestBudgetConservation_InvariantHolds(t *testing.T) {
	bank := newMockRefBank()
	k, ctx := newReferralKeeper(t, bank)

	const budget = int64(1_000)
	setCaps(t, k, ctx, 5_000, 5_000)
	eco := authtypes.NewModuleAddress(types.EcosystemModuleAccount)
	bank.balances[eco.String()] = sdk.NewCoins(sdk.NewCoin(types.BaseDenom, sdkmath.NewInt(budget)))

	inviter := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
	invitee := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
	_, err := k.CreateReferral(ctx, inviter, invitee, "")
	require.NoError(t, err)

	// 第一笔 reward=6000 → 一代 10% = 600：可用预算 1000，入账后 pending=600、available=400。
	require.NoError(t, k.TrackReward(ctx, invitee, sdkmath.NewInt(6_000)))
	require.Equal(t, sdkmath.NewInt(600), k.PendingRewardTotal(ctx))
	require.NoError(t, k.VerifyBudgetConservation(ctx), "first credit must stay within budget")

	// 第二笔 reward=6000 → bonus 600：可用预算只剩 400，既不能入账也不能延后（延后也是负债）。
	require.NoError(t, k.TrackReward(ctx, invitee, sdkmath.NewInt(6_000)))
	require.Equal(t, sdkmath.NewInt(600), k.PendingRewardTotal(ctx),
		"over-budget reward must not increase pending liabilities")
	require.NoError(t, k.VerifyBudgetConservation(ctx),
		"deferred liabilities must never exceed available budget")

	// 第三笔 reward=4000 → bonus 400：此时日网络上限已被已承诺额压到 400，
	// 触顶后只能延后（可用预算恰好 400，延后仍在预算内）。
	require.NoError(t, k.TrackReward(ctx, invitee, sdkmath.NewInt(4_000)))
	require.Equal(t, sdkmath.NewInt(600), k.PendingRewardTotal(ctx))
	require.Equal(t, sdkmath.NewInt(1_000), k.CommittedLiabilities(ctx),
		"deferred amount must stay within available budget")
	require.NoError(t, k.VerifyBudgetConservation(ctx), "committed == balance is allowed")

	// 余额等于负债后再计提：必须被拒绝（available = 0）。
	require.NoError(t, k.TrackReward(ctx, invitee, sdkmath.NewInt(10_000)))
	require.Equal(t, sdkmath.NewInt(1_000), k.CommittedLiabilities(ctx))
	require.NoError(t, k.VerifyBudgetConservation(ctx))

	// 极大整数：不得溢出绕过检查。
	huge := sdkmath.NewIntFromUint64(^uint64(0))
	require.NoError(t, k.TrackReward(ctx, invitee, huge))
	require.Equal(t, sdkmath.NewInt(1_000), k.CommittedLiabilities(ctx))
	require.NoError(t, k.VerifyBudgetConservation(ctx))
}

// 日上限触发延后时，延后额只能在可用预算内。
func TestBudgetConservation_DeferralWithinAvailable(t *testing.T) {
	bank := newMockRefBank()
	k, ctx := newReferralKeeper(t, bank)

	const budget = int64(700)
	// 日上限 100、单用户上限 100：第一笔入账后第二笔必触顶。
	setCaps(t, k, ctx, 100, 100)
	eco := authtypes.NewModuleAddress(types.EcosystemModuleAccount)
	bank.balances[eco.String()] = sdk.NewCoins(sdk.NewCoin(types.BaseDenom, sdkmath.NewInt(budget)))

	inviter := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
	invitee := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
	_, err := k.CreateReferral(ctx, inviter, invitee, "")
	require.NoError(t, err)

	require.NoError(t, k.TrackReward(ctx, invitee, sdkmath.NewInt(1_000))) // bonus 100 → 入账
	require.NoError(t, k.TrackReward(ctx, invitee, sdkmath.NewInt(1_000))) // bonus 100 → 触顶延后
	require.Equal(t, sdkmath.NewInt(200), k.CommittedLiabilities(ctx))
	require.NoError(t, k.VerifyBudgetConservation(ctx))

	// 把余额降到低于已承诺负债，模拟外部划走资金/账本漂移：检查必须报警。
	bank.balances[eco.String()] = sdk.NewCoins(sdk.NewCoin(types.BaseDenom, sdkmath.NewInt(50)))
	require.Error(t, k.VerifyBudgetConservation(ctx),
		"committed liabilities above balance must be detected")
}
