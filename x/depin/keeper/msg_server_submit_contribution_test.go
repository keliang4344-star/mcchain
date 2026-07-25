package keeper_test

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

// mockBankKeeper records payouts from the DePIN pool without real chain state.
type mockBankKeeper struct {
	sentModule string
	sentTo     sdk.AccAddress
	sentAmount sdk.Coins
}

func (m *mockBankKeeper) SpendableCoins(ctx sdk.Context, addr sdk.AccAddress) sdk.Coins {
	return nil
}

func (m *mockBankKeeper) SendCoinsFromModuleToAccount(ctx sdk.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	m.sentModule = senderModule
	m.sentTo = recipientAddr
	m.sentAmount = amt
	return nil
}

func (m *mockBankKeeper) MintCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error {
	return nil
}

// mockPhonenodeKeeper lets tests control whether a device is registered as a node
// and whether it holds a valid attestation (B2 IsAttested gate).
type mockPhonenodeKeeper struct {
	registered map[string]bool
	attested   map[string]bool
}

func (m *mockPhonenodeKeeper) HasNode(ctx sdk.Context, addr string) bool {
	return m.registered[addr]
}

func (m *mockPhonenodeKeeper) IsAttested(ctx sdk.Context, addr string) bool {
	return m.attested[addr]
}

// newSubmitTestSetup builds a depin keeper (with mock bank + phonenode keepers)
// backed by an in-memory store, with default params set.
func newSubmitTestSetup(t *testing.T, bank types.BankKeeper, phone types.PhonenodeKeeper) (*keeper.Keeper, sdk.Context) {
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
	k := keeper.NewKeeper(cdc, storeKey, memStoreKey, paramsSubspace, bank, phone)
	ctx := sdk.NewContext(stateStore, tmproto.Header{}, false, log.NewNopLogger())
	k.SetParams(ctx, types.DefaultParams())
	return k, ctx
}

// P2-1 / D5: a contribution from a device NOT registered as a phonenode must be
// rejected with ErrPhonenodeNotRegistered and must NOT pay out.
func TestSubmitContribution_UnregisteredPhonenode_Rejected(t *testing.T) {
	bank := &mockBankKeeper{}
	phone := &mockPhonenodeKeeper{registered: map[string]bool{}, attested: map[string]bool{}}
	k, ctx := newSubmitTestSetup(t, bank, phone)

	deviceAddr := sdk.AccAddress([]byte("1234567890abcdef1234")).String()

	// device is registered and attested in depin, but NOT in phonenode
	require.NoError(t, k.SetDevice(ctx, &keeper.DeviceState{Address: deviceAddr, Attested: true}))

	msg := types.NewMsgSubmitContribution(deviceAddr, "task-p2a", keeper.TaskTypeInference, "80")
	msgServer := keeper.NewMsgServerImpl(*k)

	_, err := msgServer.SubmitContribution(sdk.WrapSDKContext(ctx), msg)
	require.ErrorIs(t, err, types.ErrPhonenodeNotRegistered)
	// no module-to-account payout should have happened
	require.Empty(t, bank.sentModule)
}

// P2-1 / D5: a contribution from a device registered as a phonenode must pay out
// the computed reward (400umc for score 80 inference) from the depin pool.
func TestSubmitContribution_RegisteredPhonenode_Paid(t *testing.T) {
	bank := &mockBankKeeper{}
	phone := &mockPhonenodeKeeper{registered: map[string]bool{}, attested: map[string]bool{}}
	k, ctx := newSubmitTestSetup(t, bank, phone)

	deviceAddr := sdk.AccAddress([]byte("1234567890abcdef1234")).String()
	expectedAddr := sdk.AccAddress([]byte("1234567890abcdef1234"))

	require.NoError(t, k.SetDevice(ctx, &keeper.DeviceState{Address: deviceAddr, Attested: true}))
	// register the device as a phonenode AND attest it (B2 gate)
	phone.registered[deviceAddr] = true
	phone.attested[deviceAddr] = true

	msg := types.NewMsgSubmitContribution(deviceAddr, "task-p2b", keeper.TaskTypeInference, "80")
	msgServer := keeper.NewMsgServerImpl(*k)

	_, err := msgServer.SubmitContribution(sdk.WrapSDKContext(ctx), msg)
	require.NoError(t, err)

	expected := sdk.NewCoins(sdk.NewCoin("umc", sdk.NewInt(400)))
	require.Equal(t, types.ModuleName, bank.sentModule)
	require.True(t, bank.sentTo.Equals(expectedAddr), "paid to %s, want %s", bank.sentTo, expectedAddr)
	require.True(t, bank.sentAmount.IsEqual(expected), "paid %s, want %s", bank.sentAmount, expected)
}
