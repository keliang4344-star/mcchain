package keeper

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
	typesparams "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/stretchr/testify/require"

	"mcchain/x/dex/types"
)

// ---------------------------------------------------------------------------
// Mock bank keeper for DEX
// ---------------------------------------------------------------------------

// mockDexBank implements types.BankKeeper for DEX integration tests.
//
// 这是一个「严格复式记账」的账本 mock：每一笔转账都同时扣付款方、记收款方，
// 余额不足直接返回错误。早期版本只给收款方加钱、不扣模块账户，导致模块账户
// 余额恒为 0 也能「清算成功」——偿付能力类缺陷在测试里完全隐形（假绿）。
// 模块账户地址与生产保持一致（authtypes.NewModuleAddress），否则 keeper 里
// 按生产算法查询到的余额与测试预置的余额不是同一个账户。
type mockDexBank struct {
	sentFromMod  []sendRecord
	sentFromAcct []sendRecord
	minted       []mintBurnRecord
	burned       []mintBurnRecord
	balances     map[string]map[string]sdk.Coin // addr → denom → coin
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

// moduleAddrOf 返回与生产一致的模块账户地址字符串。
func moduleAddrOf(name string) string { return authtypes.NewModuleAddress(name).String() }

// setBalance sets the initial spendable balance for a given address and denom.
func (m *mockDexBank) setBalance(addr string, denom string, amount int64) {
	if _, ok := m.balances[addr]; !ok {
		m.balances[addr] = make(map[string]sdk.Coin)
	}
	m.balances[addr][denom] = sdk.NewCoin(denom, sdk.NewInt(amount))
}

// setModuleBalance 给模块账户预置余额（使用生产地址算法）。
func (m *mockDexBank) setModuleBalance(module string, denom string, amount int64) {
	m.setBalance(moduleAddrOf(module), denom, amount)
}

// credit 记账收款。
func (m *mockDexBank) credit(addr string, amt sdk.Coins) {
	if _, ok := m.balances[addr]; !ok {
		m.balances[addr] = make(map[string]sdk.Coin)
	}
	for _, c := range amt {
		if existing, ok := m.balances[addr][c.Denom]; ok {
			m.balances[addr][c.Denom] = sdk.NewCoin(c.Denom, existing.Amount.Add(c.Amount))
		} else {
			m.balances[addr][c.Denom] = c
		}
	}
}

// debit 记账付款；余额不足返回错误（与真实 bank keeper 语义一致）。
func (m *mockDexBank) debit(addr string, amt sdk.Coins) error {
	for _, c := range amt {
		bal, ok := m.balances[addr][c.Denom]
		if !ok || bal.Amount.LT(c.Amount) {
			have := sdk.ZeroInt()
			if ok {
				have = bal.Amount
			}
			return fmt.Errorf("mockbank: insufficient funds for %s: have %s%s, need %s%s",
				addr, have, c.Denom, c.Amount, c.Denom)
		}
	}
	for _, c := range amt {
		bal := m.balances[addr][c.Denom]
		m.balances[addr][c.Denom] = sdk.NewCoin(c.Denom, bal.Amount.Sub(c.Amount))
	}
	return nil
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
	if err := m.debit(a, amt); err != nil {
		return err
	}
	m.credit(moduleAddrOf(recipientModule), amt)
	m.sentFromAcct = append(m.sentFromAcct, sendRecord{from: a, to: recipientModule, amount: amt})
	return nil
}

func (m *mockDexBank) SendCoinsFromModuleToAccount(
	ctx sdk.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins,
) error {
	if err := m.debit(moduleAddrOf(senderModule), amt); err != nil {
		return err
	}
	m.credit(recipientAddr.String(), amt)
	m.sentFromMod = append(m.sentFromMod, sendRecord{from: senderModule, to: recipientAddr.String(), amount: amt})
	return nil
}

func (m *mockDexBank) SendCoinsFromModuleToModule(
	ctx sdk.Context, senderModule, recipientModule string, amt sdk.Coins,
) error {
	if err := m.debit(moduleAddrOf(senderModule), amt); err != nil {
		return err
	}
	m.credit(moduleAddrOf(recipientModule), amt)
	m.sentFromMod = append(m.sentFromMod, sendRecord{from: senderModule, to: recipientModule, amount: amt})
	return nil
}

func (m *mockDexBank) MintCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error {
	m.credit(moduleAddrOf(moduleName), amt)
	m.minted = append(m.minted, mintBurnRecord{recipient: moduleName, amount: amt})
	return nil
}

func (m *mockDexBank) BurnCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error {
	if err := m.debit(moduleAddrOf(moduleName), amt); err != nil {
		return err
	}
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

// GetModuleAddress 必须与生产（authtypes.NewModuleAddress）一致，否则 keeper 中
// 通过两条不同路径拿到的模块地址不是同一个账户，余额校验会出现假阴/假阳。
func (m *mockDexAccountKeeper) GetModuleAddress(name string) sdk.AccAddress {
	return authtypes.NewModuleAddress(name)
}

// HasAccount always returns true for any address.
func (m *mockDexAccountKeeper) HasAccount(ctx sdk.Context, addr sdk.AccAddress) bool {
	return true
}

func (m *mockDexAccountKeeper) GetModuleAccount(ctx sdk.Context, name string) authtypes.ModuleAccountI {
	addr := authtypes.NewModuleAddress(name)
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
	// The params subspace needs a SEPARATE transient store key. Reusing storeKey
	// for both key and tkey causes Subspace.Set to write the transient marker
	// (empty bytes) into the same underlying KVStore, clobbering the real param
	// value and making GetParamSet read empty bytes.
	tkey := sdk.NewTransientStoreKey("transient_dex_params")
	db := tmdb.NewMemDB()
	cs := store.NewCommitMultiStore(db)
	cs.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	cs.MountStoreWithDB(tkey, storetypes.StoreTypeTransient, db)
	require.NoError(t, cs.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	ps := typesparams.NewSubspace(cdc, types.Amino, storeKey, tkey, "DexParams")

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
