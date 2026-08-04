package keeper

import (
	"testing"
	"time"

	"cosmossdk.io/math"
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
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	"mcchain/x/liquidstaking/types"
)

// ---------------------------------------------------------------------------
// Mock bank keeper
// ---------------------------------------------------------------------------

type mockLSBank struct {
	balances map[string]map[string]math.Int
	minted   map[string]math.Int
	burned   map[string]math.Int
}

func newMockLSBank() *mockLSBank {
	return &mockLSBank{
		balances: map[string]map[string]math.Int{},
		minted:   map[string]math.Int{},
		burned:   map[string]math.Int{},
	}
}

func (m *mockLSBank) add(addr, denom string, amt math.Int) {
	if _, ok := m.balances[addr]; !ok {
		m.balances[addr] = map[string]math.Int{}
	}
	cur, ok := m.balances[addr][denom]
	if !ok {
		cur = math.ZeroInt()
	}
	m.balances[addr][denom] = cur.Add(amt)
}

func (m *mockLSBank) sub(addr, denom string, amt math.Int) {
	cur := m.amountOf(addr, denom)
	m.balances[addr][denom] = cur.Sub(amt)
}

func (m *mockLSBank) amountOf(addr, denom string) math.Int {
	if b, ok := m.balances[addr]; ok {
		if c, ok2 := b[denom]; ok2 {
			return c
		}
	}
	if _, ok := m.balances[addr]; !ok {
		m.balances[addr] = map[string]math.Int{}
	}
	m.balances[addr][denom] = math.ZeroInt()
	return math.ZeroInt()
}

func (m *mockLSBank) setBalance(addr, denom string, amount int64) {
	if _, ok := m.balances[addr]; !ok {
		m.balances[addr] = map[string]math.Int{}
	}
	m.balances[addr][denom] = math.NewInt(amount)
}

func moduleAddrString(name string) string { return sdk.AccAddress([]byte(name)).String() }

func (m *mockLSBank) SendCoinsFromAccountToModule(_ sdk.Context, sender sdk.AccAddress, module string, amt sdk.Coins) error {
	for _, c := range amt {
		if m.amountOf(sender.String(), c.Denom).LT(c.Amount) {
			return types.ErrInsufficientShares.Wrapf("mock bank: %s lacks %s", sender.String(), c.String())
		}
		m.sub(sender.String(), c.Denom, c.Amount)
		m.add(moduleAddrString(module), c.Denom, c.Amount)
	}
	return nil
}

func (m *mockLSBank) SendCoinsFromModuleToAccount(_ sdk.Context, module string, recipient sdk.AccAddress, amt sdk.Coins) error {
	for _, c := range amt {
		if m.amountOf(moduleAddrString(module), c.Denom).LT(c.Amount) {
			return types.ErrEmptyPool.Wrapf("mock bank: module %s lacks %s", module, c.String())
		}
		m.sub(moduleAddrString(module), c.Denom, c.Amount)
		m.add(recipient.String(), c.Denom, c.Amount)
	}
	return nil
}

func (m *mockLSBank) MintCoins(_ sdk.Context, module string, amt sdk.Coins) error {
	for _, c := range amt {
		m.add(moduleAddrString(module), c.Denom, c.Amount)
		cur, ok := m.minted[c.Denom]
		if !ok {
			cur = math.ZeroInt()
		}
		m.minted[c.Denom] = cur.Add(c.Amount)
	}
	return nil
}

func (m *mockLSBank) BurnCoins(_ sdk.Context, module string, amt sdk.Coins) error {
	for _, c := range amt {
		if m.amountOf(moduleAddrString(module), c.Denom).LT(c.Amount) {
			return types.ErrInsufficientShares.Wrap("mock bank: burn exceeds module balance")
		}
		m.sub(moduleAddrString(module), c.Denom, c.Amount)
		cur, ok := m.burned[c.Denom]
		if !ok {
			cur = math.ZeroInt()
		}
		m.burned[c.Denom] = cur.Add(c.Amount)
	}
	return nil
}

func (m *mockLSBank) GetBalance(_ sdk.Context, addr sdk.AccAddress, denom string) sdk.Coin {
	return sdk.NewCoin(denom, m.amountOf(addr.String(), denom))
}

func (m *mockLSBank) SpendableCoins(_ sdk.Context, addr sdk.AccAddress) sdk.Coins {
	denoms := m.balances[addr.String()]
	coins := make([]sdk.Coin, 0, len(denoms))
	for d, amt := range denoms {
		coins = append(coins, sdk.NewCoin(d, amt))
	}
	return sdk.NewCoins(coins...)
}

func (m *mockLSBank) HasBalance(_ sdk.Context, addr sdk.AccAddress, amt sdk.Coin) bool {
	return m.amountOf(addr.String(), amt.Denom).GTE(amt.Amount)
}

// ---------------------------------------------------------------------------
// Mock account keeper
// ---------------------------------------------------------------------------

type mockLSAccount struct{}

func (mockLSAccount) GetModuleAddress(name string) sdk.AccAddress { return sdk.AccAddress([]byte(name)) }

func (mockLSAccount) GetModuleAccount(_ sdk.Context, name string) authtypes.ModuleAccountI {
	base := authtypes.NewBaseAccount(sdk.AccAddress([]byte(name)), nil, 0, 0)
	return authtypes.NewModuleAccount(base, name)
}

func (mockLSAccount) GetAccount(_ sdk.Context, addr sdk.AccAddress) authtypes.AccountI {
	return authtypes.NewBaseAccount(addr, nil, 0, 0)
}

// ---------------------------------------------------------------------------
// Mock staking keeper
// ---------------------------------------------------------------------------

// mockLSStaking simulates just enough of x/staking: a validator registry, a
// delegation ledger keyed by (delegator, validator) and a 21-day unbonding
// period. Escrowed umc is moved out of the module account on Delegate and
// returned on unbonding maturity, mirroring the real bonded-pool flow.
type mockLSStaking struct {
	bank        *mockLSBank
	validators  map[string]stakingtypes.Validator
	delegations map[string]map[string]math.Int // delegator → validator → umc
	unbondSecs  int64
	failNext    bool
}

func newMockLSStaking(bank *mockLSBank) *mockLSStaking {
	return &mockLSStaking{
		bank:        bank,
		validators:  map[string]stakingtypes.Validator{},
		delegations: map[string]map[string]math.Int{},
		unbondSecs:  21 * 24 * 60 * 60,
	}
}

func (m *mockLSStaking) addValidator(t *testing.T, jailed bool) sdk.ValAddress {
	t.Helper()
	priv := secp256k1.GenPrivKey()
	valAddr := sdk.ValAddress(priv.PubKey().Address())
	pkAny, err := codectypes.NewAnyWithValue(priv.PubKey())
	require.NoError(t, err)
	m.validators[valAddr.String()] = stakingtypes.Validator{
		OperatorAddress: valAddr.String(),
		ConsensusPubkey: pkAny,
		Jailed:          jailed,
		Status:          stakingtypes.Bonded,
		Tokens:          math.NewInt(1_000_000_000),
		DelegatorShares: sdk.NewDec(1_000_000_000),
	}
	return valAddr
}

func (m *mockLSStaking) BondDenom(sdk.Context) string { return types.BondDenom }

func (m *mockLSStaking) GetValidator(_ sdk.Context, addr sdk.ValAddress) (stakingtypes.Validator, bool) {
	v, ok := m.validators[addr.String()]
	return v, ok
}

func (m *mockLSStaking) Delegate(
	_ sdk.Context, delAddr sdk.AccAddress, bondAmt math.Int,
	_ stakingtypes.BondStatus, validator stakingtypes.Validator, subtractAccount bool,
) (sdk.Dec, error) {
	if m.failNext {
		m.failNext = false
		return sdk.ZeroDec(), types.ErrValidatorNotFound.Wrap("mock forced failure")
	}
	if subtractAccount {
		// Real x/staking moves the coins into the bonded pool.
		if m.bank.amountOf(delAddr.String(), types.BondDenom).LT(bondAmt) {
			return sdk.ZeroDec(), types.ErrEmptyPool.Wrap("mock staking: delegator lacks umc")
		}
		m.bank.sub(delAddr.String(), types.BondDenom, bondAmt)
	}
	d := m.delegations[delAddr.String()]
	if d == nil {
		d = map[string]math.Int{}
		m.delegations[delAddr.String()] = d
	}
	cur, ok := d[validator.OperatorAddress]
	if !ok {
		cur = math.ZeroInt()
	}
	d[validator.OperatorAddress] = cur.Add(bondAmt)
	return sdk.NewDecFromInt(bondAmt), nil
}

func (m *mockLSStaking) ValidateUnbondAmount(
	_ sdk.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress, amt math.Int,
) (sdk.Dec, error) {
	bonded, ok := m.delegations[delAddr.String()][valAddr.String()]
	if !ok || bonded.LT(amt) {
		return sdk.ZeroDec(), types.ErrInsufficientShares.Wrap("mock staking: unbond exceeds delegation")
	}
	return sdk.NewDecFromInt(amt), nil
}

func (m *mockLSStaking) Undelegate(
	ctx sdk.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress, shares sdk.Dec,
) (time.Time, error) {
	amt := shares.TruncateInt()
	bonded, ok := m.delegations[delAddr.String()][valAddr.String()]
	if !ok || bonded.LT(amt) {
		return time.Time{}, types.ErrInsufficientShares.Wrap("mock staking: undelegate exceeds delegation")
	}
	m.delegations[delAddr.String()][valAddr.String()] = bonded.Sub(amt)
	// x/staking releases the coins back to the delegator (here: the module
	// account) when the unbonding entry matures. The test fast-forwards this by
	// crediting the module account immediately; ClaimMatured is still gated on
	// the completion timestamp.
	m.bank.add(delAddr.String(), types.BondDenom, amt)
	return ctx.BlockTime().Add(time.Duration(m.unbondSecs) * time.Second), nil
}

func (m *mockLSStaking) GetDelegatorDelegations(
	_ sdk.Context, delegator sdk.AccAddress, _ uint16,
) []stakingtypes.Delegation {
	out := []stakingtypes.Delegation{}
	for val, amt := range m.delegations[delegator.String()] {
		if !amt.IsPositive() {
			continue
		}
		out = append(out, stakingtypes.Delegation{
			DelegatorAddress: delegator.String(),
			ValidatorAddress: val,
			Shares:           sdk.NewDecFromInt(amt),
		})
	}
	return out
}

// ---------------------------------------------------------------------------
// Mock distribution keeper
// ---------------------------------------------------------------------------

type mockLSDistr struct {
	bank    *mockLSBank
	rewards map[string]math.Int // validator → umc reward pending
}

func newMockLSDistr(bank *mockLSBank) *mockLSDistr {
	return &mockLSDistr{bank: bank, rewards: map[string]math.Int{}}
}

func (m *mockLSDistr) setReward(valAddr string, amount int64) {
	m.rewards[valAddr] = math.NewInt(amount)
}

func (m *mockLSDistr) WithdrawDelegationRewards(
	_ sdk.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress,
) (sdk.Coins, error) {
	amt, ok := m.rewards[valAddr.String()]
	if !ok || !amt.IsPositive() {
		return sdk.NewCoins(), nil
	}
	delete(m.rewards, valAddr.String())
	m.bank.add(delAddr.String(), types.BondDenom, amt)
	return sdk.NewCoins(sdk.NewCoin(types.BondDenom, amt)), nil
}

// ---------------------------------------------------------------------------
// Shared setup
// ---------------------------------------------------------------------------

type lsFixture struct {
	k       *Keeper
	ctx     sdk.Context
	bank    *mockLSBank
	staking *mockLSStaking
	distr   *mockLSDistr
}

func setupLiquidStaking(t *testing.T) *lsFixture {
	t.Helper()

	storeKey := sdk.NewKVStoreKey(types.StoreKey)
	db := tmdb.NewMemDB()
	cs := store.NewCommitMultiStore(db)
	cs.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, cs.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	bank := newMockLSBank()
	stk := newMockLSStaking(bank)
	dist := newMockLSDistr(bank)

	k := NewKeeper(cdc, storeKey, "authority", mockLSAccount{}, bank, stk, dist)
	ctx := sdk.NewContext(cs, tmproto.Header{Time: time.Unix(1_700_000_000, 0)}, false, log.NewNopLogger())
	require.NoError(t, k.SetParams(ctx, types.DefaultParams()))

	return &lsFixture{k: k, ctx: ctx, bank: bank, staking: stk, distr: dist}
}

// fundedAddr returns a fresh account pre-loaded with `umc` of MC.
func (f *lsFixture) fundedAddr(t *testing.T, umc int64) sdk.AccAddress {
	t.Helper()
	priv := secp256k1.GenPrivKey()
	addr := sdk.AccAddress(priv.PubKey().Address())
	f.bank.setBalance(addr.String(), types.BondDenom, umc)
	return addr
}

// advance moves block time forward by d.
func (f *lsFixture) advance(d time.Duration) {
	f.ctx = f.ctx.WithBlockTime(f.ctx.BlockTime().Add(d))
}
