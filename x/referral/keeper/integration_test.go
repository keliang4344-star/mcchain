package keeper_test

import (
	"fmt"
	"testing"

	tmdb "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/libs/log"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/store"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	sdkmath "cosmossdk.io/math"
	typesparams "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/stretchr/testify/require"
	"mcchain/x/referral/keeper"
	"mcchain/x/referral/types"
)

// ---- mock bank keeper ----

type mockRefBank struct {
	balances map[string]sdk.Coins
}

func newMockRefBank() *mockRefBank {
	return &mockRefBank{balances: map[string]sdk.Coins{}}
}
func (m *mockRefBank) mint(addr string, amt sdk.Coins) {
	m.balances[addr] = m.balances[addr].Add(amt...)
}
func (m *mockRefBank) GetBalance(_ sdk.Context, addr sdk.AccAddress, denom string) sdk.Coin {
	return sdk.NewCoin(denom, m.balances[addr.String()].AmountOf(denom))
}
func (m *mockRefBank) SpendableCoins(_ sdk.Context, addr sdk.AccAddress) sdk.Coins {
	return m.balances[addr.String()]
}
func (m *mockRefBank) SendCoinsFromModuleToAccount(_ sdk.Context, senderModule string, to sdk.AccAddress, amt sdk.Coins) error {
	from := authtypes.NewModuleAddress(senderModule).String()
	have := m.balances[from]
	if !have.IsAllGTE(amt) {
		return fmt.Errorf("insufficient in %s", senderModule)
	}
	m.balances[from] = have.Sub(amt...)
	m.balances[to.String()] = m.balances[to.String()].Add(amt...)
	return nil
}
func (m *mockRefBank) BurnCoins(_ sdk.Context, moduleName string, amt sdk.Coins) error {
	from := authtypes.NewModuleAddress(moduleName).String()
	have := m.balances[from]
	if !have.IsAllGTE(amt) {
		return fmt.Errorf("insufficient to burn in %s", moduleName)
	}
	m.balances[from] = have.Sub(amt...)
	return nil
}

// mockPhonenode 让任何地址都视为已注册节点（通过反女巫校验）。
type mockPhonenode struct{}

func (mockPhonenode) HasNode(_ sdk.Context, _ string) bool { return true }

func newReferralKeeper(t *testing.T, bank types.BankKeeper) (*keeper.Keeper, sdk.Context) {
	t.Helper()
	storeKey := sdk.NewKVStoreKey(types.StoreKey)
	memStoreKey := storetypes.NewMemoryStoreKey(types.MemStoreKey)
	db := tmdb.NewMemDB()
	stateStore := store.NewCommitMultiStore(db)
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	stateStore.MountStoreWithDB(memStoreKey, storetypes.StoreTypeMemory, nil)
	require.NoError(t, stateStore.LoadLatestVersion())
	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	ps := typesparams.NewSubspace(cdc, types.Amino, storeKey, memStoreKey, "ReferralParams")
	k := keeper.NewKeeper(cdc, storeKey, ps, bank, mockPhonenode{})
	ctx := sdk.NewContext(stateStore, tmproto.Header{}, false, log.NewNopLogger())
	k.SetParams(ctx, types.Params{
		Level1RewardRateBps: 1000, // 10%
		Level2RewardRateBps: 500,  // 5%
		Level3RewardRateBps: 200,  // 2%
		MinPayout:          "0",
		MaxReferralsPerUser: 100,
		CooldownBlocks:      0,
		DailyPerUserCap:     1_000_000_000_000,
		DailyNetworkCap:     1_000_000_000_000,
	})
	return k, ctx
}

// TestReferralEndToEnd 端到端验证推荐模块真实可用：
// 创建推荐 → 追踪被推荐人奖励（三级分成）→ 领取（含 1% 销毁）→ 查询。
func TestReferralEndToEnd(t *testing.T) {
	bank := newMockRefBank()
	k, ctx := newReferralKeeper(t, bank)
	// 生态账户预拨资金（领取/销毁来源）
	bank.mint(authtypes.NewModuleAddress(types.EcosystemModuleAccount).String(),
		sdk.NewCoins(sdk.NewCoin("umc", sdkmath.NewInt(1_000_000_000))))

	inviter := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
	invitee := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()

	// 1) 创建推荐关系（A → B）
	id, err := k.CreateReferral(ctx, inviter, invitee, "CODE123")
	require.NoError(t, err)
	require.Equal(t, uint64(1), id)

	// 2) 追踪被推荐人 B 的 DePIN 奖励 1,000,000 umc → A 得一代 10% = 100,000
	err = k.TrackReward(ctx, invitee, sdkmath.NewInt(1_000_000))
	require.NoError(t, err)
	require.Equal(t, int64(100_000), k.GetPendingRewards(ctx, inviter).Amount.Int64())

	// 3) 查询服务端：PendingRewards / Referral
	q, err := k.PendingRewards(ctx, &types.QueryPendingRewardsRequest{Claimer: inviter})
	require.NoError(t, err)
	require.Equal(t, "100000", q.Amount)

	rq, err := k.Referral(ctx, &types.QueryReferralRequest{ReferralId: id})
	require.NoError(t, err)
	require.Equal(t, inviter, rq.Referral.Inviter)

	// 4) 领取：销毁 1% (1,000)，实付 99,000
	claimed, err := k.ClaimRewards(ctx, inviter)
	require.NoError(t, err)
	require.Equal(t, int64(99_000), claimed.Amount.Int64())
	require.Equal(t, int64(99_000), bank.balances[inviter].AmountOf("umc").Int64())
	require.Equal(t, int64(0), k.GetPendingRewards(ctx, inviter).Amount.Int64())

	// 5) 反女巫：被推荐人不可再被推荐
	other := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
	_, err = k.CreateReferral(ctx, other, invitee, "X")
	require.ErrorIs(t, err, types.ErrInviteeAlreadyReferred)
}
