package keeper

import (
	"testing"

	tmdb "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/libs/log"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/store"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	typesparams "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/stretchr/testify/require"

	"mcchain/x/dex/types"
)

// ---- mock：LP 激励路径只用到 SendCoinsFromModuleToModule 与模块地址查询 ----

type lpBankMock struct{ sent []sdk.Coins }

func (m *lpBankMock) SpendableCoins(_ sdk.Context, _ sdk.AccAddress) sdk.Coins { return nil }
func (m *lpBankMock) SendCoinsFromModuleToAccount(_ sdk.Context, _ string, _ sdk.AccAddress, _ sdk.Coins) error {
	return nil
}
func (m *lpBankMock) SendCoinsFromAccountToModule(_ sdk.Context, _ sdk.AccAddress, _ string, _ sdk.Coins) error {
	return nil
}
func (m *lpBankMock) SendCoinsFromModuleToModule(_ sdk.Context, _, _ string, amt sdk.Coins) error {
	m.sent = append(m.sent, amt)
	return nil
}
func (m *lpBankMock) MintCoins(_ sdk.Context, _ string, _ sdk.Coins) error { return nil }
func (m *lpBankMock) BurnCoins(_ sdk.Context, _ string, _ sdk.Coins) error { return nil }
func (m *lpBankMock) GetBalance(_ sdk.Context, _ sdk.AccAddress, denom string) sdk.Coin {
	return sdk.NewCoin(denom, sdk.ZeroInt())
}
func (m *lpBankMock) HasBalance(_ sdk.Context, _ sdk.AccAddress, _ sdk.Coin) bool { return true }

type lpAccountMock struct{}

func (lpAccountMock) GetModuleAddress(name string) sdk.AccAddress {
	return authtypes.NewModuleAddress(name)
}
func (lpAccountMock) GetModuleAccount(_ sdk.Context, name string) authtypes.ModuleAccountI {
	return authtypes.NewEmptyModuleAccount(name)
}
func (lpAccountMock) GetAccount(_ sdk.Context, _ sdk.AccAddress) authtypes.AccountI { return nil }

// mockReleaseGate 设备池每日释放闸门。
type mockReleaseGate struct {
	remaining uint64
	recorded  uint64
}

func (m *mockReleaseGate) CheckDailyReleaseCap(_ sdk.Context, amount uint64) (bool, uint64, uint64, error) {
	return amount <= m.remaining, m.remaining, m.remaining, nil
}
func (m *mockReleaseGate) RecordDailyRelease(_ sdk.Context, amount uint64) {
	m.recorded += amount
}

func setupLPKeeper(t *testing.T) (Keeper, sdk.Context, *lpBankMock, *mockReleaseGate) {
	t.Helper()
	storeKey := sdk.NewKVStoreKey(types.StoreKey)
	memKey := storetypes.NewMemoryStoreKey("dex_mem")
	db := tmdb.NewMemDB()
	cs := store.NewCommitMultiStore(db)
	cs.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	cs.MountStoreWithDB(memKey, storetypes.StoreTypeMemory, nil)
	require.NoError(t, cs.LoadLatestVersion())

	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	ps := typesparams.NewSubspace(cdc, types.Amino, storeKey, memKey, "DexParams").WithKeyTable(types.ParamKeyTable())

	bank := &lpBankMock{}
	gate := &mockReleaseGate{remaining: 5_000_000_000} // 5000 MC
	k := NewKeeper(cdc, storeKey, ps, bank, lpAccountMock{})
	k.SetDepinKeeper(gate)

	ctx := sdk.NewContext(cs, tmproto.Header{Height: 10}, false, log.NewNopLogger())
	k.SetParams(ctx, types.DefaultParams())
	// 一个含 umc 的池子
	k.SetPool(ctx, types.Pool{
		Id: 1, DenomA: InitialPoolDenomMC, DenomB: "uatom",
		ReserveA: "1000000", ReserveB: "1000000", TotalLp: "1000000", FeeRateBps: 30,
	})
	return *k, ctx, bank, gate
}

// TestLPIncentiveGoesThroughDepinReleaseGate 是设备池日释放闸门的回归闸门。
//
// 回归守则：LP 激励（5000 MC/日）从设备池出账时，必须同时
// CheckDailyReleaseCap 与 RecordDailyRelease。否则「4015 天线性释放」的可对账性
// 被破坏 —— 账本记「已释放 X」，实际设备池少了 X + 5000 MC/日，长此以往释放曲线
// 与金库余额对不上且无法归因。
func TestLPIncentiveGoesThroughDepinReleaseGate(t *testing.T) {
	k, ctx, bank, gate := setupLPKeeper(t)

	k.DistributeLPIncentive(ctx)

	require.Len(t, bank.sent, 1, "激励应正常发放一次")
	require.Positive(t, gate.recorded, "实际出账必须计入设备池当日释放账本")
	require.Equal(t, bank.sent[0].AmountOf(InitialPoolDenomMC).Uint64(), gate.recorded,
		"记账金额必须等于实际出账金额，不得按预算金额虚增")
}

// TestLPIncentiveWithheldWhenDailyCapExhausted 额度不足时不得出账。
func TestLPIncentiveWithheldWhenDailyCapExhausted(t *testing.T) {
	k, ctx, bank, gate := setupLPKeeper(t)
	gate.remaining = 1 // 当日只剩 1 umc

	k.DistributeLPIncentive(ctx)

	require.Empty(t, bank.sent, "当日释放额度不足时不得发激励")
	require.Zero(t, gate.recorded)
}

// TestLPIncentiveWithheldWhenGateUnwired 闸门未接线时 fail-closed。
func TestLPIncentiveWithheldWhenGateUnwired(t *testing.T) {
	k, ctx, bank, _ := setupLPKeeper(t)
	k.SetDepinKeeper(nil) // 模拟 app.go 接线遗漏

	k.DistributeLPIncentive(ctx)

	require.Empty(t, bank.sent, "释放闸门未接线时必须 fail-closed，不得直抽设备池")
}
