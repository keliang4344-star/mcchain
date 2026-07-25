package keeper

import (
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
	typesparams "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/stretchr/testify/require"

	"mcchain/x/dex/types"
)

// ---------------------------------------------------------------------------
// Mock bank keeper for DEX
// ---------------------------------------------------------------------------

// mockDexBank implements types.BankKeeper for DEX integration tests.
// Tracks SendCoins*, MintCoins, BurnCoins, GetBalance, HasBalance calls.
type mockDexBank struct {
	sentFromMod    []sendRecord
	sentFromAcct   []sendRecord
	minted         []mintBurnRecord
	burned         []mintBurnRecord
	balances       map[string]map[string]sdk.Coin // addr → denom → coin
}

type sendRecord struct {
	from   string
	to     string
	amount sdk.Coins
}

type mintBurnRecord struct {
	recipient string
	amount    sdk.Coins
}

func newMockDexBank() *mockDexBank {
	return &mockDexBank{
		balances: make(map[string]map[string]sdk.Coin),
	}
}

// setBalance sets the initial spendable balance for a given address and denom.
func (m *mockDexBank) setBalance(addr string, denom string, amount int64) {
	if _, ok := m.balances[addr]; !ok {
		m.balances[addr] = make(map[string]sdk.Coin)
	}
	m.balances[addr][denom] = sdk.NewCoin(denom, sdk.NewInt(amount))
}

func (m *mockDexBank) SpendableCoins(ctx sdk.Context, addr sdk.AccAddress) sdk.Coins {
	a := addr.String()
	if _, ok := m.balances[a]; !ok {
		return sdk.NewCoins()
	}
	var coins sdk.Coins
	for _, c := range m.balances[a] {
		coins = append(coins, c)
	}
	return coins
}

func (m *mockDexBank) SendCoinsFromAccountToModule(
	ctx sdk.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins,
) error {
	a := senderAddr.String()
	for _, c := range amt {
		if bal, ok := m.balances[a][c.Denom]; ok && bal.Amount.GTE(c.Amount) {
			m.balances[a][c.Denom] = sdk.NewCoin(c.Denom, bal.Amount.Sub(c.Amount))
		}
	}
	m.sentFromAcct = append(m.sentFromAcct, sendRecord{from: a, to: recipientModule, amount: amt})
	return nil
}

func (m *mockDexBank) SendCoinsFromModuleToAccount(
	ctx sdk.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins,
) error {
	a := recipientAddr.String()
	if _, ok := m.balances[a]; !ok {
		m.balances[a] = make(map[string]sdk.Coin)
	}
	for _, c := range amt {
		if existing, ok := m.balances[a][c.Denom]; ok {
			m.balances[a][c.Denom] = sdk.NewCoin(c.Denom, existing.Amount.Add(c.Amount))
		} else {
			m.balances[a][c.Denom] = c
		}
	}
	m.sentFromMod = append(m.sentFromMod, sendRecord{from: senderModule, to: a, amount: amt})
	return nil
}

func (m *mockDexBank) SendCoinsFromModuleToModule(
	ctx sdk.Context, senderModule, recipientModule string, amt sdk.Coins,
) error {
	m.sentFromMod = append(m.sentFromMod, sendRecord{from: senderModule, to: recipientModule, amount: amt})
	return nil
}

func (m *mockDexBank) MintCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error {
	m.minted = append(m.minted, mintBurnRecord{recipient: moduleName, amount: amt})
	return nil
}

func (m *mockDexBank) BurnCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error {
	m.burned = append(m.burned, mintBurnRecord{recipient: moduleName, amount: amt})
	return nil
}

func (m *mockDexBank) GetBalance(ctx sdk.Context, addr sdk.AccAddress, denom string) sdk.Coin {
	a := addr.String()
	if bal, ok := m.balances[a]; ok {
		if c, ok2 := bal[denom]; ok2 {
			return c
		}
	}
	return sdk.NewCoin(denom, sdk.ZeroInt())
}

func (m *mockDexBank) HasBalance(ctx sdk.Context, addr sdk.AccAddress, amt sdk.Coin) bool {
	return m.GetBalance(ctx, addr, amt.Denom).Amount.GTE(amt.Amount)
}

// ---------------------------------------------------------------------------
// Mock account keeper (minimal)
// ---------------------------------------------------------------------------

type mockDexAccountKeeper struct{}

func (m *mockDexAccountKeeper) GetModuleAddress(name string) sdk.AccAddress {
	return sdk.AccAddress([]byte(name))
}

// HasAccount always returns true for any address.
func (m *mockDexAccountKeeper) HasAccount(ctx sdk.Context, addr sdk.AccAddress) bool {
	return true
}

func (m *mockDexAccountKeeper) GetModuleAccount(ctx sdk.Context, name string) authtypes.ModuleAccountI {
	addr := sdk.AccAddress([]byte(name))
	base := authtypes.NewBaseAccount(addr, nil, 0, 0)
	return authtypes.NewModuleAccount(base, name)
}

func (m *mockDexAccountKeeper) GetAccount(ctx sdk.Context, addr sdk.AccAddress) authtypes.AccountI {
	return authtypes.NewBaseAccount(addr, nil, 0, 0)
}

// ---------------------------------------------------------------------------
// Shared test setup for DEX
// ---------------------------------------------------------------------------

// setupDex creates a DEX keeper with in-memory store and a mock bank.
func setupDex(t *testing.T) (*Keeper, sdk.Context, *mockDexBank) {
	storeKey := sdk.NewKVStoreKey(types.StoreKey)
	db := tmdb.NewMemDB()
	cs := store.NewCommitMultiStore(db)
	cs.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, cs.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	ps := typesparams.NewSubspace(cdc, types.Amino, storeKey, storeKey, "DexParams")

	bk := newMockDexBank()
	acct := &mockDexAccountKeeper{}
	k := NewKeeper(cdc, storeKey, ps, bk, acct)
	ctx := sdk.NewContext(cs, tmproto.Header{}, false, log.NewNopLogger())
	k.SetParams(ctx, types.DefaultParams())
	return k, ctx, bk
}

// addrOfDex generates a fresh bech32 address for Dex tests.
func addrOfDex(t *testing.T) string {
	priv := secp256k1.GenPrivKey()
	return sdk.AccAddress(priv.PubKey().Address()).String()
}
