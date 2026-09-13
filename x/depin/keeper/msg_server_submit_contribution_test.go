package keeper_test

import (
	"strings"
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

func (m *mockBankKeeper) SpendableCoins(_ sdk.Context, _ sdk.AccAddress) sdk.Coins {
	// Simulate a funded DePIN reward pool so the linear-release vault
	// initializes with a positive InitialBalance (production funds this at
	// genesis via tokenomics). Without this, dailyCap=0 blocks all payouts.
	return sdk.NewCoins(sdk.NewInt64Coin("umc", 1e15))
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

func (m *mockBankKeeper) BurnCoins(_ sdk.Context, _ string, _ sdk.Coins) error {
	return nil
}

// mockPhonenodeKeeper lets tests control whether a device is registered as a node
// and whether it holds a valid attestation (IsAttested gate).
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

// PhonenodeKeeper 接口新增的两个方法。贡献拨付用例不针对层级判定，
// 因此 IsVerifiedAttested 直接与 IsAttested 同值，MarkOracleVerified 记为 no-op。
func (m *mockPhonenodeKeeper) IsVerifiedAttested(ctx sdk.Context, addr string) bool {
	return m.attested[addr]
}

func (m *mockPhonenodeKeeper) MarkOracleVerified(_ sdk.Context, _ string, _ string) error {
	return nil
}

// mockReferralKeeper is a no-op referral keeper; TrackDepinReward is required by
// the keeper signature but is not asserted on by these contribution tests.
type mockReferralKeeper struct{}

func (m *mockReferralKeeper) TrackDepinReward(_ sdk.Context, _ string, _ sdk.Int) error {
	return nil
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
	k := keeper.NewKeeper(cdc, storeKey, memStoreKey, paramsSubspace, bank, phone, &mockReferralKeeper{})
	ctx := sdk.NewContext(stateStore, tmproto.Header{}, false, log.NewNopLogger())
	k.SetParams(ctx, types.DefaultParams())
	return k, ctx
}

// a contribution from a device NOT registered as a phonenode must be
// rejected with ErrPhonenodeNotRegistered and must NOT pay out.
func TestSubmitContribution_UnregisteredPhonenode_Rejected(t *testing.T) {
	bank := &mockBankKeeper{}
	phone := &mockPhonenodeKeeper{registered: map[string]bool{}, attested: map[string]bool{}}
	k, ctx := newSubmitTestSetup(t, bank, phone)

	deviceAddr := sdk.AccAddress([]byte("1234567890abcdef1234")).String()

	// device is registered+attested in depin (passes defense L1/L2) but NOT
	// registered as a phonenode node (HasNode=false) → must hit the phonenode
	// payout gate (ErrPhonenodeNotRegistered), not a defense-layer rejection.
	require.NoError(t, k.SetDevice(ctx, &keeper.DeviceState{
		Address: deviceAddr, Registered: true, Attested: true,
		AttestedUntil: types.AttestationValiditySeconds, // 认证须仍在有效期内
	}))
	phone.attested[deviceAddr] = true // passes defense layer 2 (phonenode attestation)

	msg := types.NewMsgSubmitContribution(deviceAddr, "task-p2a", keeper.TaskTypeInference, "80")
	msgServer := keeper.NewMsgServerImpl(*k)

	_, err := msgServer.SubmitContribution(sdk.WrapSDKContext(ctx), msg)
	require.ErrorIs(t, err, types.ErrPhonenodeNotRegistered)
	// no module-to-account payout should have happened
	require.Empty(t, bank.sentModule)
}

// a contribution from a device registered as a phonenode must pay out
// the computed reward (400umc for score 80 inference) from the depin pool.
func TestSubmitContribution_RegisteredPhonenode_Paid(t *testing.T) {
	bank := &mockBankKeeper{}
	phone := &mockPhonenodeKeeper{registered: map[string]bool{}, attested: map[string]bool{}}
	k, ctx := newSubmitTestSetup(t, bank, phone)

	deviceAddr := sdk.AccAddress([]byte("1234567890abcdef1234")).String()
	expectedAddr := sdk.AccAddress([]byte("1234567890abcdef1234"))

	require.NoError(t, k.SetDevice(ctx, &keeper.DeviceState{
		Address: deviceAddr, Registered: true, Attested: true,
		AttestedUntil: types.AttestationValiditySeconds, // 认证须仍在有效期内
	}))
	// register the device as a phonenode AND attest it (attestation gate)
	phone.registered[deviceAddr] = true
	phone.attested[deviceAddr] = true

	msg := types.NewMsgSubmitContribution(deviceAddr, "task-p2b", keeper.TaskTypeInference, "80")
	msgServer := keeper.NewMsgServerImpl(*k)

	_, err := msgServer.SubmitContribution(sdk.WrapSDKContext(ctx), msg)
	require.NoError(t, err)

	// V3 经济：base = score(80) × inference rate(5) = 400；共振倍数 ≈ 1.2402
	//（taskCount=1, quality=0.8, 空闲网络）→ 496 umc 全额拨付。
	// 赏金 5% 销毁已撤销（白皮书 §24.6 否决清单）：设备劳动应得 100% 到手，链上零截留。
	expectedBase := keeper.ComputeReward(80, keeper.TaskTypeInference)
	require.Equal(t, 400, expectedBase)
	expectedAdjusted := keeper.ComputeResonanceReward(expectedBase, 1, sdk.ZeroDec(), sdk.NewDecWithPrec(8, 1))
	expected := sdk.NewCoins(sdk.NewCoin("umc", sdk.NewInt(int64(expectedAdjusted))))
	require.Equal(t, types.ModuleName, bank.sentModule)
	require.True(t, bank.sentTo.Equals(expectedAddr), "paid to %s, want %s", bank.sentTo, expectedAddr)
	require.True(t, bank.sentAmount.IsEqual(expected), "paid %s, want %s", bank.sentAmount, expected)
}

// 认证是「有期限的断言」。曾经通过（Attested=true）但未记录有效期
// （AttestedUntil=0，即升级前的历史状态）必须按失效处理，不得继续兑付。
func TestSubmitContribution_LegacyAttestationWithoutExpiry_Rejected(t *testing.T) {
	bank := &mockBankKeeper{}
	phone := &mockPhonenodeKeeper{registered: map[string]bool{}, attested: map[string]bool{}}
	k, ctx := newSubmitTestSetup(t, bank, phone)

	deviceAddr := sdk.AccAddress([]byte("1234567890abcdef1234")).String()
	// 旧口径状态：只有布尔位，没有有效期
	require.NoError(t, k.SetDevice(ctx, &keeper.DeviceState{Address: deviceAddr, Registered: true, Attested: true}))
	phone.registered[deviceAddr] = true
	phone.attested[deviceAddr] = true

	msg := types.NewMsgSubmitContribution(deviceAddr, "task-legacy", keeper.TaskTypeInference, "80")
	msgServer := keeper.NewMsgServerImpl(*k)

	_, err := msgServer.SubmitContribution(sdk.WrapSDKContext(ctx), msg)
	require.ErrorIs(t, err, types.ErrDeviceNotAttested)
	require.Empty(t, bank.sentModule, "expired attestation must not trigger any payout")
}

// 认证到期后必须重新认证，过期后提交贡献一律拒绝。
func TestSubmitContribution_ExpiredAttestation_Rejected(t *testing.T) {
	bank := &mockBankKeeper{}
	phone := &mockPhonenodeKeeper{registered: map[string]bool{}, attested: map[string]bool{}}
	k, ctx := newSubmitTestSetup(t, bank, phone)

	deviceAddr := sdk.AccAddress([]byte("1234567890abcdef1234")).String()
	// 有效期已过：AttestedUntil 落在一个过去的时间点（now=0，故用 -1）
	require.NoError(t, k.SetDevice(ctx, &keeper.DeviceState{
		Address: deviceAddr, Registered: true, Attested: true, AttestedUntil: -1,
	}))
	phone.registered[deviceAddr] = true
	phone.attested[deviceAddr] = true

	msg := types.NewMsgSubmitContribution(deviceAddr, "task-expired", keeper.TaskTypeInference, "80")
	msgServer := keeper.NewMsgServerImpl(*k)

	_, err := msgServer.SubmitContribution(sdk.WrapSDKContext(ctx), msg)
	require.ErrorIs(t, err, types.ErrDeviceNotAttested)
	require.Empty(t, bank.sentModule)
}

// task_id 直接拼成 KVStore 键，超长输入必须在状态层被拒绝，
// 否则每次提交都能确定性膨胀一次状态（且永久驻留）。
func TestSubmitContribution_OversizedTaskID_Rejected(t *testing.T) {
	bank := &mockBankKeeper{}
	phone := &mockPhonenodeKeeper{registered: map[string]bool{}, attested: map[string]bool{}}
	k, ctx := newSubmitTestSetup(t, bank, phone)

	deviceAddr := sdk.AccAddress([]byte("1234567890abcdef1234")).String()
	require.NoError(t, k.SetDevice(ctx, &keeper.DeviceState{
		Address: deviceAddr, Registered: true, Attested: true,
		AttestedUntil: types.AttestationValiditySeconds,
	}))
	phone.registered[deviceAddr] = true
	phone.attested[deviceAddr] = true

	oversized := strings.Repeat("t", types.MaxContributionTaskIDLength+1)
	msg := types.NewMsgSubmitContribution(deviceAddr, oversized, keeper.TaskTypeInference, "80")
	msgServer := keeper.NewMsgServerImpl(*k)

	_, err := msgServer.SubmitContribution(sdk.WrapSDKContext(ctx), msg)
	require.Error(t, err)
	require.Empty(t, bank.sentModule)
}

// AttestDevice 的 challenge 必须一次性消费——同一对 (challenge, signature)
// 重放时不得再次置真 Attested，也不得刷新有效期。
func TestAttestDevice_ChallengeReplayRejected(t *testing.T) {
	bank := &mockBankKeeper{}
	phone := &mockPhonenodeKeeper{registered: map[string]bool{}, attested: map[string]bool{}}
	k, ctx := newSubmitTestSetup(t, bank, phone)

	deviceAddr := sdk.AccAddress([]byte("1234567890abcdef1234")).String()
	require.NoError(t, k.SetDevice(ctx, &keeper.DeviceState{Address: deviceAddr, Registered: true}))

	msgServer := keeper.NewMsgServerImpl(*k)
	msg := types.NewMsgAttestDevice(deviceAddr, deviceAddr, "challenge-once", "sig-once")

	// 首次：SoftOracle 只要求 challenge/signature 非空 → 通过
	_, err := msgServer.AttestDevice(sdk.WrapSDKContext(ctx), msg)
	require.NoError(t, err)

	st, err := k.GetDevice(ctx, deviceAddr)
	require.NoError(t, err)
	require.True(t, st.Attested)

	// 重放同一 challenge：必须被拒
	_, err = msgServer.AttestDevice(sdk.WrapSDKContext(ctx), msg)
	require.Error(t, err, "replaying the same attestation challenge must be rejected")
}

// ConsumeAttestChallenge 是 challenge 一次性消费的唯一入口，
// 首次返回 true，之后恒为 false（同一地址 + 同一 challenge）。
func TestConsumeAttestChallenge_OneTimeUse(t *testing.T) {
	k, ctx := newSubmitTestSetup(t, &mockBankKeeper{}, &mockPhonenodeKeeper{})

	require.True(t, k.ConsumeAttestChallenge(ctx, "addr-a", "chal-1"), "first use must succeed")
	require.False(t, k.ConsumeAttestChallenge(ctx, "addr-a", "chal-1"), "replay must fail")
	// 不同 challenge 互不影响
	require.True(t, k.ConsumeAttestChallenge(ctx, "addr-a", "chal-2"))
	// 不同地址使用相同 challenge 也不互相影响（键含地址域分隔）
	require.True(t, k.ConsumeAttestChallenge(ctx, "addr-b", "chal-1"))
}

// IsDeviceAttestationValid 的边界语义（nil / 未认证 / 无有效期 / 已过期 / 有效）。
func TestIsDeviceAttestationValid(t *testing.T) {
	require.False(t, keeper.IsDeviceAttestationValid(nil, 100))
	require.False(t, keeper.IsDeviceAttestationValid(&keeper.DeviceState{}, 100))
	require.False(t, keeper.IsDeviceAttestationValid(&keeper.DeviceState{Attested: true}, 100),
		"attested without recorded expiry must be treated as invalid (fail-closed)")
	require.False(t, keeper.IsDeviceAttestationValid(&keeper.DeviceState{Attested: true, AttestedUntil: 100}, 100),
		"expiry is exclusive: now == until is expired")
	require.True(t, keeper.IsDeviceAttestationValid(&keeper.DeviceState{Attested: true, AttestedUntil: 101}, 100))
}
