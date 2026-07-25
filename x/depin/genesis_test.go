package depin

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
	typesparams "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/stretchr/testify/require"
	"mcchain/x/depin/keeper"
	"mcchain/x/depin/types"
)

// mockBankKeeper records the coins minted into a module account (genesis pool
// funding) without touching real chain state.
type mockBankKeeper struct {
	mintedModule string
	mintedCoins  sdk.Coins
}

func (m *mockBankKeeper) SpendableCoins(ctx sdk.Context, addr sdk.AccAddress) sdk.Coins {
	return nil
}

func (m *mockBankKeeper) SendCoinsFromModuleToAccount(ctx sdk.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	return nil
}

func (m *mockBankKeeper) MintCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error {
	m.mintedModule = moduleName
	m.mintedCoins = amt
	return nil
}

// newDePinKeeperForGenesis builds a depin keeper backed by an in-memory store
// with the provided (mock) bank keeper; phonenode keeper is unused by genesis.
func newDePinKeeperForGenesis(t *testing.T, bank types.BankKeeper) (*keeper.Keeper, sdk.Context) {
	storeKey := sdk.NewKVStoreKey(types.StoreKey)
	memStoreKey := storetypes.NewMemoryStoreKey(types.MemStoreKey)

	db := tmdb.NewMemDB()
	stateStore := store.NewCommitMultiStore(db)
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	stateStore.MountStoreWithDB(memStoreKey, storetypes.StoreTypeMemory, nil)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	paramsSubspace := typesparams.NewSubspace(cdc, types.Amino, storeKey, memStoreKey, "DepinParams")
	k := keeper.NewKeeper(cdc, storeKey, memStoreKey, paramsSubspace, bank, nil)
	ctx := sdk.NewContext(stateStore, tmproto.Header{}, false, log.NewNopLogger())
	return k, ctx
}

// TestGenesis restores the ignite-scaffold genesis sanity check: the default
// genesis state (with the P1 InitialPool / RewardDenom params) must validate.
func TestGenesis(t *testing.T) {
	genesisState := types.DefaultGenesis()
	require.NoError(t, genesisState.Validate())
}

// P1-1 / D2（Q7 变更 + 五池模型）：depin 不再自铸。InitGenesis 仅 SetParams；
// InitialPool(5.5e14 umc = 设备激励池 55%) 由 tokenomics 在 InitGenesis 全额拨付到
// depin 模块账户（见 x/tokenomics）。本测试验证 depin InitGenesis 运行后：参数正确写入，且不铸造。
func TestInitGenesisDoesNotMint(t *testing.T) {
	bank := &mockBankKeeper{}
	k, ctx := newDePinKeeperForGenesis(t, bank)

	genState := types.DefaultGenesis()
	require.Equal(t, uint64(550_000_000_000_000), genState.Params.InitialPool)
	require.Equal(t, "umc", genState.Params.RewardDenom)

	InitGenesis(ctx, *k, *genState)

	// Q7：depin 不再自铸，bank 不应被调用铸造。
	require.Empty(t, bank.mintedModule, "depin must not mint after Q7")
	// 参数应已正确写入。
	require.Equal(t, genState.Params, k.GetParams(ctx))
}
