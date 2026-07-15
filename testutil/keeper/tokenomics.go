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
	"github.com/stretchr/testify/require"

	tokenomicskeeper "mcchain/x/tokenomics/keeper"
	"mcchain/x/tokenomics/types"
)

// MockBankKeeper 是内存版 bank keeper，仅用于 tokenomics 单测：
// 以 bech32 地址为键追踪各账户（含模块账户）的 umc 余额，精确模拟
// MintCoins / Send 的净效果。
//
// 注意：与 cosmos-sdk v0.47.3 bank.BaseKeeper.SendCoinsFromModuleToModule 行为
// 一致（仅校验模块账户存在，不校验 blockedAddrs）；SendCoinsFromModuleToAccount
// 的接收方为团队多签 vesting 账户（非模块账户），亦不受 blocked 限制。
type MockBankKeeper struct {
	balances     map[string]sdk.Int // key: bech32 地址
	mintedModule string
	mintedCoins  sdk.Coins
}

// MintCoins 向指定模块账户铸造并累加余额（模拟唯一铸币入口）。
func (m *MockBankKeeper) MintCoins(_ sdk.Context, moduleName string, amt sdk.Coins) error {
	addr := authtypes.NewModuleAddress(moduleName).String()
	if _, ok := m.balances[addr]; !ok {
		m.balances[addr] = sdk.ZeroInt()
	}
	for _, c := range amt {
		m.balances[addr] = m.balances[addr].Add(c.Amount)
	}
	m.mintedModule = moduleName
	m.mintedCoins = amt
	return nil
}

// SendCoinsFromModuleToAccount 从模块账户拨付到外部地址（团队 vesting 账户）。
func (m *MockBankKeeper) SendCoinsFromModuleToAccount(_ sdk.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	from := authtypes.NewModuleAddress(senderModule).String()
	return m.send(from, recipientAddr.String(), amt)
}

// SendCoinsFromModuleToModule 模块账户间转账（社区/生态/生态→depin 切片）。
func (m *MockBankKeeper) SendCoinsFromModuleToModule(_ sdk.Context, senderModule, recipientModule string, amt sdk.Coins) error {
	from := authtypes.NewModuleAddress(senderModule).String()
	to := authtypes.NewModuleAddress(recipientModule).String()
	return m.send(from, to, amt)
}

// send 在 from/to 之间转移 amt（按 denom 内部仅有 umc）。
func (m *MockBankKeeper) send(from, to string, amt sdk.Coins) error {
	if _, ok := m.balances[from]; !ok {
		m.balances[from] = sdk.ZeroInt()
	}
	if _, ok := m.balances[to]; !ok {
		m.balances[to] = sdk.ZeroInt()
	}
	for _, c := range amt {
		m.balances[from] = m.balances[from].Sub(c.Amount)
		m.balances[to] = m.balances[to].Add(c.Amount)
	}
	return nil
}

// GetBalance 返回指定地址某 denom 的余额（查询 allocations 当前余额用）。
func (m *MockBankKeeper) GetBalance(_ sdk.Context, addr sdk.AccAddress, denom string) sdk.Coin {
	bal, ok := m.balances[addr.String()]
	if !ok {
		return sdk.NewCoin(denom, sdk.ZeroInt())
	}
	return sdk.NewCoin(denom, bal)
}

// GetModuleAddress 返回模块账户地址（社区/生态池地址解析，Q5）。
func (m *MockBankKeeper) GetModuleAddress(moduleName string) sdk.AccAddress {
	return authtypes.NewModuleAddress(moduleName)
}

// MockAccountKeeper 是内存版 account keeper，足以支撑 InitGenesis 创建团队 vesting 账户。
type MockAccountKeeper struct {
	accounts map[string]authtypes.AccountI
}

// GetAccount 返回指定地址的账户（不存在返回 nil）。
func (m *MockAccountKeeper) GetAccount(_ sdk.Context, addr sdk.AccAddress) authtypes.AccountI {
	return m.accounts[addr.String()]
}

// SetAccount 持久化账户。
func (m *MockAccountKeeper) SetAccount(_ sdk.Context, acc authtypes.AccountI) {
	m.accounts[acc.GetAddress().String()] = acc
}

// NewAccountWithAddress 按地址创建新账户。返回 *BaseAccount 以满足
// CreateTeamVestingAccount 中的类型断言（baseAcc.(*authtypes.BaseAccount)）。
func (m *MockAccountKeeper) NewAccountWithAddress(_ sdk.Context, addr sdk.AccAddress) authtypes.AccountI {
	return authtypes.NewBaseAccount(addr, nil, 0, 0)
}

// TokenomicsKeeper 构造一个 tokenomics keeper + 内存 store + mock bank/account keepers，
// 供 InitGenesis / Query / Invariant / Vesting 单测使用。返回 mock 以便断言余额与账户。
func TokenomicsKeeper(t testing.TB) (*tokenomicskeeper.Keeper, sdk.Context, *MockBankKeeper, *MockAccountKeeper) {
	storeKey := sdk.NewKVStoreKey(types.StoreKey)

	db := tmdb.NewMemDB()
	stateStore := store.NewCommitMultiStore(db)
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	ctx := sdk.NewContext(stateStore, tmproto.Header{}, false, log.NewNopLogger())

	ak := &MockAccountKeeper{accounts: make(map[string]authtypes.AccountI)}
	bk := &MockBankKeeper{balances: make(map[string]sdk.Int)}

	// 生态切片拨付目标模块账户固定为 depin（C2：编译期常量，见 types.DepinModuleName）。
	k := tokenomicskeeper.NewKeeper(cdc, storeKey, ak, bk)

	return k, ctx, bk, ak
}
