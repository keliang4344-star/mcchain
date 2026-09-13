package keeper_test

import (
	"fmt"
	"testing"

	sdkmath "cosmossdk.io/math"
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
// activeNodes 为非空时，仅其中地址视为挖矿节点（用于 10 代分级测试）；
// 为空则全部视为挖矿节点（默认行为，向后兼容既有测试）。
type mockPhonenode struct{ activeNodes map[string]bool }

func (m mockPhonenode) HasNode(_ sdk.Context, _ string) bool { return true }

func (m mockPhonenode) IsActiveNode(_ sdk.Context, addr string) bool {
	if m.activeNodes == nil {
		return true
	}
	return m.activeNodes[addr]
}

func newReferralKeeper(t *testing.T, bank types.BankKeeper) (*keeper.Keeper, sdk.Context) {
	return newReferralKeeperWithNodes(t, bank, nil)
}

// newReferralKeeperWithNodes 允许指定"挖矿节点"集合（activeNodes）。
func newReferralKeeperWithNodes(t *testing.T, bank types.BankKeeper, activeNodes map[string]bool) (*keeper.Keeper, sdk.Context) {
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
	k := keeper.NewKeeper(cdc, storeKey, ps, bank, mockPhonenode{activeNodes: activeNodes})
	ctx := sdk.NewContext(stateStore, tmproto.Header{}, false, log.NewNopLogger())

	// 生态预算账户注资：链上由 x/tokenomics.InitGenesis 从设备激励池切出
	// types.InitialEcosystemBudget（82.5M MC）拨付到该模块账户，返佣由此支出。
	// 测试 keeper 必须复现这一步 —— 否则余额驱动的配速上限会把
	// 有效日上限钳到 0，所有「日上限」用例都会因为「预算耗尽」而失败，
	// 掩盖掉它本来要验证的参数上限语义。
	if mb, ok := bank.(*mockRefBank); ok {
		mb.mint(
			authtypes.NewModuleAddress(types.EcosystemModuleAccount).String(),
			sdk.NewCoins(sdk.NewCoin(types.BaseDenom, sdk.NewIntFromUint64(types.InitialEcosystemBudget))),
		)
	}
	k.SetParams(ctx, types.Params{
		Level1RewardRateBps: 1000, // 10%
		Level2RewardRateBps: 500,  // 5%
		Level3RewardRateBps: 300,  // 3%（10 代模型权重）
		MinPayout:           "0",
		MaxReferralsPerUser: 100,
		CooldownBlocks:      0,
		DailyPerUserCap:     1_000_000_000_000,
		DailyNetworkCap:     1_000_000_000_000,
	})
	return k, ctx
}

// TestReferralEndToEnd 端到端验证推荐模块真实可用：
// 创建推荐 → 追踪被推荐人奖励（按代际分成）→ 领取（100% 足额，无销毁）→ 查询。
func TestReferralEndToEnd(t *testing.T) {
	bank := newMockRefBank()
	k, ctx := newReferralKeeper(t, bank)
	// 生态账户预拨资金（推荐奖励领取来源）
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

	// 4) 领取：推荐奖励 100% 足额到手（1% 销毁已撤销），实付 100,000
	claimed, err := k.ClaimRewards(ctx, inviter)
	require.NoError(t, err)
	require.Equal(t, int64(100_000), claimed.Amount.Int64())
	require.Equal(t, int64(100_000), bank.balances[inviter].AmountOf("umc").Int64())
	require.Equal(t, int64(0), k.GetPendingRewards(ctx, inviter).Amount.Int64())

	// 5) 反女巫：被推荐人不可再被推荐
	other := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
	_, err = k.CreateReferral(ctx, other, invitee, "X")
	require.ErrorIs(t, err, types.ErrInviteeAlreadyReferred)
}
