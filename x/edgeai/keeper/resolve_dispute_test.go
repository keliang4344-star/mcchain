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
	typesparams "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/stretchr/testify/require"
	"mcchain/x/edgeai/types"
)

// mockPhonenode 记录 slash 调用，供 B3.1 cheat 裁定断言。
type mockPhonenode struct {
	slashed []string
}

func (m *mockPhonenode) HasNode(ctx sdk.Context, addr string) bool    { return true }
func (m *mockPhonenode) IsAttested(ctx sdk.Context, addr string) bool { return true }
func (m *mockPhonenode) SlashIfBad(ctx sdk.Context, addr, reason string, bps uint32) error {
	m.slashed = append(m.slashed, addr)
	return nil
}
func (m *mockPhonenode) GetVerifierNodes(ctx sdk.Context) []string { return nil }

func setupEdgeai(t *testing.T) (*Keeper, sdk.Context, *mockPhonenode) {
	storeKey := sdk.NewKVStoreKey(types.StoreKey)
	memKey := storetypes.NewMemoryStoreKey(types.MemStoreKey)
	db := tmdb.NewMemDB()
	cs := store.NewCommitMultiStore(db)
	cs.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	cs.MountStoreWithDB(memKey, storetypes.StoreTypeMemory, nil)
	require.NoError(t, cs.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	ps := typesparams.NewSubspace(cdc, types.Amino, storeKey, memKey, "EdgeaiParams")
	m := &mockPhonenode{}
	k := NewKeeper(cdc, storeKey, memKey, ps, m, mockBank{}, nil)
	ctx := sdk.NewContext(cs, tmproto.Header{}, false, log.NewNopLogger())
	k.SetParams(ctx, types.DefaultParams())
	return k, ctx, m
}

func addrOf(t *testing.T) string {
	priv := secp256k1.GenPrivKey()
	return sdk.AccAddress(priv.PubKey().Address()).String()
}

// TestResolveDisputeCheat B3.1：仲裁者裁定 cheat → 任务标记作弊 + 提交者被 slash + 不发币。
func TestResolveDisputeCheat(t *testing.T) {
	k, ctx, m := setupEdgeai(t)
	arb := addrOf(t)
	submitter := addrOf(t)
	challenger := addrOf(t)

	params := types.DefaultParams()
	params.Arbitrator = arb
	k.SetParams(ctx, params)

	require.NoError(t, k.SetTask(ctx, &Task{Id: "t1", Status: types.TaskStatusOpen}))
	require.NoError(t, k.SetResult(ctx, &Result{TaskId: "t1", Submitter: submitter, Status: types.ResultStatusPending, SubmittedAtBlock: 1}))
	require.NoError(t, k.SetDispute(ctx, &Dispute{TaskId: "t1", Challenger: challenger, Submitter: submitter, Status: "open", Resolution: "none", OpenedAtBlock: 1}))

	ms := NewMsgServerImpl(*k)
	_, err := ms.ResolveDispute(sdk.WrapSDKContext(ctx), &types.MsgResolveDispute{Creator: arb, TaskId: "t1", Resolution: "cheat"})
	require.NoError(t, err)

	task, err := k.GetTask(ctx, "t1")
	require.NoError(t, err)
	require.Equal(t, types.TaskStatusCheated, task.Status, "cheat 裁定应标记任务作弊，阻止拨付")

	d, err := k.GetDispute(ctx, "t1")
	require.NoError(t, err)
	require.Equal(t, "cheat", d.Resolution)
	require.Equal(t, submitter, d.Submitter)
	require.Contains(t, m.slashed, submitter, "cheat 裁定应 slash 结果提交者")
}

// TestResolveDisputeHonest B3.1：仲裁者裁定 honest → 争议结案，任务不标记作弊（BeginBlock 将照常拨付）。
func TestResolveDisputeHonest(t *testing.T) {
	k, ctx, m := setupEdgeai(t)
	arb := addrOf(t)

	params := types.DefaultParams()
	params.Arbitrator = arb
	k.SetParams(ctx, params)

	require.NoError(t, k.SetTask(ctx, &Task{Id: "t1", Status: types.TaskStatusOpen}))
	require.NoError(t, k.SetResult(ctx, &Result{TaskId: "t1", Submitter: addrOf(t), Status: types.ResultStatusPending, SubmittedAtBlock: 1}))
	require.NoError(t, k.SetDispute(ctx, &Dispute{TaskId: "t1", Challenger: addrOf(t), Submitter: addrOf(t), Status: "open", Resolution: "none", OpenedAtBlock: 1}))

	ms := NewMsgServerImpl(*k)
	_, err := ms.ResolveDispute(sdk.WrapSDKContext(ctx), &types.MsgResolveDispute{Creator: arb, TaskId: "t1", Resolution: "honest"})
	require.NoError(t, err)

	task, _ := k.GetTask(ctx, "t1")
	require.Equal(t, types.TaskStatusOpen, task.Status, "honest 裁定不应标记作弊")
	d, _ := k.GetDispute(ctx, "t1")
	require.Equal(t, "honest", d.Resolution)
	require.Empty(t, m.slashed, "honest 裁定不应 slash")
}

// TestResolveDisputeAuthz 校验仲裁者授权与参数配置。
func TestResolveDisputeAuthz(t *testing.T) {
	k, ctx, _ := setupEdgeai(t)
	arb := addrOf(t)
	intruder := addrOf(t)

	require.NoError(t, k.SetTask(ctx, &Task{Id: "t1", Status: types.TaskStatusOpen}))
	require.NoError(t, k.SetDispute(ctx, &Dispute{TaskId: "t1", Status: "open", Resolution: "none", OpenedAtBlock: 1}))

	ms := NewMsgServerImpl(*k)

	// 仲裁者未配置 → 拒绝
	_, err := ms.ResolveDispute(sdk.WrapSDKContext(ctx), &types.MsgResolveDispute{Creator: arb, TaskId: "t1", Resolution: "honest"})
	require.ErrorIs(t, err, types.ErrArbitratorNotSet)

	// 配置后，非仲裁者 → 拒绝
	params := types.DefaultParams()
	params.Arbitrator = arb
	k.SetParams(ctx, params)
	_, err = ms.ResolveDispute(sdk.WrapSDKContext(ctx), &types.MsgResolveDispute{Creator: intruder, TaskId: "t1", Resolution: "honest"})
	require.Error(t, err)

	// 非法 resolution
	_, err = ms.ResolveDispute(sdk.WrapSDKContext(ctx), &types.MsgResolveDispute{Creator: arb, TaskId: "t1", Resolution: "bogus"})
	require.ErrorIs(t, err, types.ErrInvalidResolution)
}
