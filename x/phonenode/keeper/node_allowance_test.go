package keeper_test

import (
	"fmt"
	"testing"
	"time"

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
	"mcchain/x/phonenode/keeper"
	"mcchain/x/phonenode/types"
	tokenomicstypes "mcchain/x/tokenomics/types"
)

// ---- mock bank keeper（仅实现节点津贴发放所需的接口子集）----

type mockBankAllowance struct {
	balances map[string]sdk.Coins
	sent     []struct {
		from, to string
		amt      sdk.Coins
	}
}

func newMockBankAllowance() *mockBankAllowance {
	return &mockBankAllowance{balances: map[string]sdk.Coins{}}
}

func (m *mockBankAllowance) mint(addr string, amt sdk.Coins) {
	m.balances[addr] = m.balances[addr].Add(amt...)
}

func (m *mockBankAllowance) SpendableCoins(_ sdk.Context, addr sdk.AccAddress) sdk.Coins {
	return m.balances[addr.String()]
}

func (m *mockBankAllowance) SendCoinsFromModuleToModule(_ sdk.Context, _, _ string, _ sdk.Coins) error {
	return nil
}

func (m *mockBankAllowance) BurnCoins(_ sdk.Context, _ string, _ sdk.Coins) error {
	return nil
}

// MintCoins credits a module account. The slashing path re-mints the treasury
// share of a native slash, so the keeper requires this capability.
func (m *mockBankAllowance) MintCoins(_ sdk.Context, moduleName string, amt sdk.Coins) error {
	addr := authtypes.NewModuleAddress(moduleName).String()
	m.balances[addr] = m.balances[addr].Add(amt...)
	return nil
}

func (m *mockBankAllowance) SendCoinsFromModuleToAccount(_ sdk.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	from := authtypes.NewModuleAddress(senderModule).String()
	have := m.balances[from]
	if !have.IsAllGTE(amt) {
		return fmt.Errorf("insufficient funds in module %s", senderModule)
	}
	m.balances[from] = have.Sub(amt...)
	m.balances[recipientAddr.String()] = m.balances[recipientAddr.String()].Add(amt...)
	m.sent = append(m.sent, struct {
		from, to string
		amt      sdk.Coins
	}{from, recipientAddr.String(), amt})
	return nil
}

// newKeeperWithBank 构造带 mock bank 的 phonenode keeper（镜像 testutil 脚手架）。
func newKeeperWithBank(t *testing.T, bank types.BankKeeper) (*keeper.Keeper, sdk.Context) {
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
	paramsSubspace := typesparams.NewSubspace(cdc, types.Amino, storeKey, memStoreKey, "PhonenodeParams")
	k := keeper.NewKeeper(cdc, storeKey, memStoreKey, paramsSubspace, bank, nil, nil)
	ctx := sdk.NewContext(stateStore, tmproto.Header{}, false, log.NewNopLogger())
	k.SetParams(ctx, types.DefaultParams())
	return k, ctx
}

func fundDepin(t *testing.T, bank *mockBankAllowance, mc uint64) {
	bank.mint(authtypes.NewModuleAddress(tokenomicstypes.DepinModuleName).String(),
		sdk.NewCoins(sdk.NewCoin(tokenomicstypes.DefaultDenom, sdk.NewIntFromUint64(mc*1_000_000))))
}

func TestNodeCapitalAllowanceDailyPayout(t *testing.T) {
	bank := newMockBankAllowance()
	k, ctx := newKeeperWithBank(t, bank)
	fundDepin(t, bank, 1000) // 1000 MC 进设备池

	priv := secp256k1.GenPrivKey()
	op := sdk.AccAddress(priv.PubKey().Address())
	_, err := k.RegisterNode(ctx, op.String(), "pixel8", "android", "validator")
	require.NoError(t, err)

	// 第 100 天
	ctx = ctx.WithBlockTime(time.Unix(86400*100, 0))
	require.NoError(t, k.DistributeNodeCapitalAllowances(ctx))
	require.Equal(t, int64(30_000_000), bank.balances[op.String()].AmountOf(tokenomicstypes.DefaultDenom).Int64())

	// 同日再分发 → 不应重复发放
	require.NoError(t, k.DistributeNodeCapitalAllowances(ctx))
	require.Equal(t, int64(30_000_000), bank.balances[op.String()].AmountOf(tokenomicstypes.DefaultDenom).Int64(),
		"同日重复分发不应叠加")

	// 第 101 天 → 再发一日
	ctx = ctx.WithBlockTime(time.Unix(86400*101, 0))
	require.NoError(t, k.DistributeNodeCapitalAllowances(ctx))
	require.Equal(t, int64(60_000_000), bank.balances[op.String()].AmountOf(tokenomicstypes.DefaultDenom).Int64())
}

func TestNodeCapitalAllowanceDisabled(t *testing.T) {
	bank := newMockBankAllowance()
	k, ctx := newKeeperWithBank(t, bank)
	fundDepin(t, bank, 1000)

	priv := secp256k1.GenPrivKey()
	op := sdk.AccAddress(priv.PubKey().Address())
	_, err := k.RegisterNode(ctx, op.String(), "pixel8", "android", "validator")
	require.NoError(t, err)

	k.SetNodeAllowanceConfig(ctx, keeper.NodeAllowanceConfig{Enabled: false, PerDay: 30_000_000})
	ctx = ctx.WithBlockTime(time.Unix(86400*100, 0))
	require.NoError(t, k.DistributeNodeCapitalAllowances(ctx))
	require.Empty(t, bank.balances[op.String()], "禁用后不应发放")
}

func TestNodeCapitalAllowanceSkipsJailed(t *testing.T) {
	bank := newMockBankAllowance()
	k, ctx := newKeeperWithBank(t, bank)
	fundDepin(t, bank, 1000)

	priv := secp256k1.GenPrivKey()
	op := sdk.AccAddress(priv.PubKey().Address())
	_, err := k.RegisterNode(ctx, op.String(), "pixel8", "android", "validator")
	require.NoError(t, err)
	// 标记 jail
	node, _ := k.GetNode(ctx, op.String())
	node.VerifierStatus = "jailed"
	require.NoError(t, k.SetNode(ctx, node))

	ctx = ctx.WithBlockTime(time.Unix(86400*100, 0))
	require.NoError(t, k.DistributeNodeCapitalAllowances(ctx))
	require.Empty(t, bank.balances[op.String()], "jail 节点不应领取津贴")
}
