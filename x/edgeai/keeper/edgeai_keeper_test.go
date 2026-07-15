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
	typesparams "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/stretchr/testify/require"
	"mcchain/x/edgeai/types"
)

// ---------------------------------------------------------------------------
// 测试基础设施：可插拔的 mock keeper（phonenode / payout / bank）
// ---------------------------------------------------------------------------

// mockPhonenodeFalse：用于验证 SubmitResult 的 attested 闸口（返回未认证）。
type mockPhonenodeFalse struct{ slashed []string }

func (m *mockPhonenodeFalse) HasNode(ctx sdk.Context, addr string) bool    { return true }
func (m *mockPhonenodeFalse) IsAttested(ctx sdk.Context, addr string) bool { return false }
func (m *mockPhonenodeFalse) SlashIfBad(ctx sdk.Context, addr, reason string, bps uint32) error {
	m.slashed = append(m.slashed, addr)
	return nil
}

// mockPayout：记录拨付调用，便于断言「贡献即挖矿」经济闭环。
type mockPayout struct {
	calls []payoutCall
}
type payoutCall struct {
	addr   string
	amount uint64
}

func (m *mockPayout) PayoutReward(_ sdk.Context, addr sdk.AccAddress, amount uint64) error {
	m.calls = append(m.calls, payoutCall{addr: addr.String(), amount: amount})
	return nil
}

// mockBankZero：零余额 bank mock，用于测试余额不足场景。
type mockBankZero struct{}

func (mockBankZero) SpendableCoins(_ sdk.Context, _ sdk.AccAddress) sdk.Coins {
	return sdk.NewCoins()
}
func (mockBankZero) SendCoinsFromAccountToModule(_ sdk.Context, _ sdk.AccAddress, _ string, _ sdk.Coins) error {
	return nil
}
func (mockBankZero) SendCoinsFromModuleToAccount(_ sdk.Context, _ string, _ sdk.AccAddress, _ sdk.Coins) error {
	return nil
}

// mockBank：满足 BankKeeper 接口的最小实现（托管/拨付均 no-op，余额充足）。
type mockBank struct{}

func (mockBank) SpendableCoins(_ sdk.Context, _ sdk.AccAddress) sdk.Coins {
	return sdk.NewCoins(sdk.NewInt64Coin("umc", 1e15))
}
func (mockBank) SendCoinsFromAccountToModule(_ sdk.Context, _ sdk.AccAddress, _ string, _ sdk.Coins) error {
	return nil
}
func (mockBank) SendCoinsFromModuleToAccount(_ sdk.Context, _ string, _ sdk.AccAddress, _ sdk.Coins) error {
	return nil
}

// mockBankCap：记录「模块账户→账户」拨付，用于断言需求方付费（escrow）经济闭环。
type mockBankCap struct {
	modToAcct []bankSend
}
type bankSend struct {
	module string
	to     string
	amount uint64
}

func (m *mockBankCap) SpendableCoins(_ sdk.Context, _ sdk.AccAddress) sdk.Coins {
	return sdk.NewCoins(sdk.NewInt64Coin("umc", 1e15))
}
func (m *mockBankCap) SendCoinsFromAccountToModule(_ sdk.Context, _ sdk.AccAddress, _ string, _ sdk.Coins) error {
	return nil
}
func (m *mockBankCap) SendCoinsFromModuleToAccount(_ sdk.Context, module string, to sdk.AccAddress, amt sdk.Coins) error {
	m.modToAcct = append(m.modToAcct, bankSend{module: module, to: to.String(), amount: amt.AmountOf("umc").Uint64()})
	return nil
}

// setupEdgeaiWith 构造一个可配置依赖的 edgeai keeper（用于 BeginBlock 等集成路径测试）。
func setupEdgeaiWith(t *testing.T, pn types.PhonenodeKeeper, pay types.PayoutKeeper, bk types.BankKeeper) (*Keeper, sdk.Context) {
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
	k := NewKeeper(cdc, storeKey, memKey, ps, pn, bk, pay)
	ctx := sdk.NewContext(cs, tmproto.Header{}, false, log.NewNopLogger())
	k.SetParams(ctx, types.DefaultParams())
	return k, ctx
}

// ---------------------------------------------------------------------------
// CreateTask
// ---------------------------------------------------------------------------

func TestCreateTask(t *testing.T) {
	k, ctx, _ := setupEdgeai(t)
	ms := NewMsgServerImpl(*k)
	creator := addrOf(t)

	_, err := ms.CreateTask(sdk.WrapSDKContext(ctx), &types.MsgCreateTask{Creator: creator, Description: "run inference", Reward: 500})
	require.NoError(t, err)

	task, err := k.GetTask(ctx, "1")
	require.NoError(t, err)
	require.NotNil(t, task)
	require.Equal(t, creator, task.Creator)
	require.Equal(t, "run inference", task.Description)
	require.Equal(t, uint64(500), task.Reward)
	require.Equal(t, types.TaskStatusOpen, task.Status)
}

func TestCreateTaskInvalidCreator(t *testing.T) {
	k, ctx, _ := setupEdgeai(t)
	ms := NewMsgServerImpl(*k)
	_, err := ms.CreateTask(sdk.WrapSDKContext(ctx), &types.MsgCreateTask{Creator: "not-an-address", Reward: 100})
	require.Error(t, err)
}

func TestCreateTaskIDIncrements(t *testing.T) {
	k, ctx, _ := setupEdgeai(t)
	ms := NewMsgServerImpl(*k)
	creator := addrOf(t)
	_, err := ms.CreateTask(sdk.WrapSDKContext(ctx), &types.MsgCreateTask{Creator: creator, Reward: 1})
	require.NoError(t, err)
	_, err = ms.CreateTask(sdk.WrapSDKContext(ctx), &types.MsgCreateTask{Creator: creator, Reward: 2})
	require.NoError(t, err)
	require.NotNil(t, mustGetTask(t, k, ctx, "2"))
}

// ---------------------------------------------------------------------------
// SubmitResult
// ---------------------------------------------------------------------------

func TestSubmitResultRejectedWhenNotAttested(t *testing.T) {
	k, ctx, _ := setupEdgeai(t)
	pn := &mockPhonenodeFalse{}
	k2 := NewKeeper(k.cdc, k.storeKey, k.memKey, k.paramstore, pn, mockBank{}, &mockPayout{})
	ms := NewMsgServerImpl(*k2)
	require.NoError(t, k.SetTask(ctx, &Task{Id: "1", Status: types.TaskStatusOpen}))

	_, err := ms.SubmitResult(sdk.WrapSDKContext(ctx), &types.MsgSubmitResult{Creator: addrOf(t), TaskId: "1", ResultHash: "h", AttestationNonce: "n"})
	require.Error(t, err, "未认证节点提交结果应被闸口拒绝")
}

func TestSubmitResultOK(t *testing.T) {
	k, ctx, _ := setupEdgeai(t)
	ms := NewMsgServerImpl(*k)
	node := addrOf(t)
	require.NoError(t, k.SetTask(ctx, &Task{Id: "1", Status: types.TaskStatusOpen}))

	_, err := ms.SubmitResult(sdk.WrapSDKContext(ctx), &types.MsgSubmitResult{Creator: node, TaskId: "1", ResultHash: "h", AttestationNonce: "n"})
	require.NoError(t, err)
	require.True(t, k.HasResult(ctx, "1", node))
}

func TestSubmitResultTaskNotOpen(t *testing.T) {
	k, ctx, _ := setupEdgeai(t)
	ms := NewMsgServerImpl(*k)
	require.NoError(t, k.SetTask(ctx, &Task{Id: "1", Status: types.TaskStatusDone}))

	_, err := ms.SubmitResult(sdk.WrapSDKContext(ctx), &types.MsgSubmitResult{Creator: addrOf(t), TaskId: "1", ResultHash: "h", AttestationNonce: "n"})
	require.ErrorIs(t, err, types.ErrTaskNotOpen)
}

func TestSubmitResultDuplicate(t *testing.T) {
	k, ctx, _ := setupEdgeai(t)
	ms := NewMsgServerImpl(*k)
	node := addrOf(t)
	require.NoError(t, k.SetTask(ctx, &Task{Id: "1", Status: types.TaskStatusOpen}))
	_, err := ms.SubmitResult(sdk.WrapSDKContext(ctx), &types.MsgSubmitResult{Creator: node, TaskId: "1", ResultHash: "h", AttestationNonce: "n"})
	require.NoError(t, err)
	_, err = ms.SubmitResult(sdk.WrapSDKContext(ctx), &types.MsgSubmitResult{Creator: node, TaskId: "1", ResultHash: "h2", AttestationNonce: "n2"})
	require.ErrorIs(t, err, types.ErrDuplicateResult)
}

func TestSubmitResultAssigneeGate(t *testing.T) {
	k, ctx, _ := setupEdgeai(t)
	ms := NewMsgServerImpl(*k)
	owner := addrOf(t)
	assignee := addrOf(t)
	intruder := addrOf(t)
	// 任务指定 Assignee
	require.NoError(t, k.SetTask(ctx, &Task{Id: "1", Status: types.TaskStatusOpen, Assignee: assignee}))

	// 非领取人提交 → 拒绝
	_, err := ms.SubmitResult(sdk.WrapSDKContext(ctx), &types.MsgSubmitResult{Creator: intruder, TaskId: "1", ResultHash: "h", AttestationNonce: "n"})
	require.ErrorIs(t, err, types.ErrNotAssigned)

	// 领取人本人提交 → 成功（owner 仅作为占位，实际提交者为 assignee）
	_, err = ms.SubmitResult(sdk.WrapSDKContext(ctx), &types.MsgSubmitResult{Creator: assignee, TaskId: "1", ResultHash: "h", AttestationNonce: "n"})
	require.NoError(t, err)

	_ = owner
}

// ---------------------------------------------------------------------------
// OpenDispute
// ---------------------------------------------------------------------------

func TestOpenDispute(t *testing.T) {
	k, ctx, _ := setupEdgeai(t)
	ms := NewMsgServerImpl(*k)
	challenger := addrOf(t)
	require.NoError(t, k.SetTask(ctx, &Task{Id: "1", Status: types.TaskStatusOpen}))

	_, err := ms.OpenDispute(sdk.WrapSDKContext(ctx), &types.MsgOpenDispute{Creator: challenger, TaskId: "1", Reason: "bad result"})
	require.NoError(t, err)

	task, _ := k.GetTask(ctx, "1")
	require.Equal(t, types.TaskStatusDisputed, task.Status)
	d, err := k.GetDispute(ctx, "1")
	require.NoError(t, err)
	require.NotNil(t, d)
	require.Equal(t, challenger, d.Challenger)
	require.Equal(t, "open", d.Status)
}

func TestOpenDisputeTaskNotFound(t *testing.T) {
	k, ctx, _ := setupEdgeai(t)
	ms := NewMsgServerImpl(*k)
	_, err := ms.OpenDispute(sdk.WrapSDKContext(ctx), &types.MsgOpenDispute{Creator: addrOf(t), TaskId: "999", Reason: "x"})
	require.ErrorIs(t, err, types.ErrTaskNotFound)
}

func TestOpenDisputeDuplicate(t *testing.T) {
	k, ctx, _ := setupEdgeai(t)
	ms := NewMsgServerImpl(*k)
	challenger := addrOf(t)
	require.NoError(t, k.SetTask(ctx, &Task{Id: "1", Status: types.TaskStatusOpen}))
	_, err := ms.OpenDispute(sdk.WrapSDKContext(ctx), &types.MsgOpenDispute{Creator: challenger, TaskId: "1", Reason: "x"})
	require.NoError(t, err)
	_, err = ms.OpenDispute(sdk.WrapSDKContext(ctx), &types.MsgOpenDispute{Creator: challenger, TaskId: "1", Reason: "y"})
	require.ErrorIs(t, err, types.ErrDisputeExists)
}

// ---------------------------------------------------------------------------
// BeginBlock（B3.1 R4 贡献即挖矿结算）
// ---------------------------------------------------------------------------

func TestBeginBlockPayoutAfterWindow(t *testing.T) {
	bk := &mockBankCap{}
	k, ctx := setupEdgeaiWith(t, &mockPhonenode{}, nil, bk)
	ms := NewMsgServerImpl(*k)
	node := addrOf(t)
	_, err := ms.CreateTask(sdk.WrapSDKContext(ctx), &types.MsgCreateTask{Creator: node, Reward: 500})
	require.NoError(t, err)
	_, err = ms.SubmitResult(sdk.WrapSDKContext(ctx.WithBlockHeight(1)), &types.MsgSubmitResult{Creator: node, TaskId: "1", ResultHash: "h", AttestationNonce: "n"})
	require.NoError(t, err)

	// 推进区块高度超过争议窗口（默认 100，提交于高度 1）
	ctx = ctx.WithBlockHeight(int64(types.DefaultParams().DisputePeriodBlocks) + 10)
	k.BeginBlock(ctx)

	// 需求方付费（escrow）：由 edgeai 模块账户（托管金）向 submitter 拨付。
	require.Len(t, bk.modToAcct, 1, "窗口过后应拨付一笔奖励")
	require.Equal(t, types.ModuleName, bk.modToAcct[0].module)
	require.Equal(t, node, bk.modToAcct[0].to)
	require.Equal(t, uint64(500), bk.modToAcct[0].amount)

	task, _ := k.GetTask(ctx, "1")
	require.Equal(t, types.TaskStatusDone, task.Status)
	res, _ := k.GetResult(ctx, "1", node)
	require.Equal(t, types.ResultStatusValid, res.Status)
}

func TestBeginBlockNoPayoutBeforeWindow(t *testing.T) {
	bk := &mockBankCap{}
	k, ctx := setupEdgeaiWith(t, &mockPhonenode{}, nil, bk)
	ms := NewMsgServerImpl(*k)
	node := addrOf(t)
	_, err := ms.CreateTask(sdk.WrapSDKContext(ctx), &types.MsgCreateTask{Creator: node, Reward: 500})
	require.NoError(t, err)
	_, err = ms.SubmitResult(sdk.WrapSDKContext(ctx), &types.MsgSubmitResult{Creator: node, TaskId: "1", ResultHash: "h", AttestationNonce: "n"})
	require.NoError(t, err)

	ctx = ctx.WithBlockHeight(50) // < 100 窗口
	k.BeginBlock(ctx)
	require.Empty(t, bk.modToAcct, "窗口内不应拨付")
}

func TestBeginBlockCheatedSkipsPayout(t *testing.T) {
	pn := &mockPhonenode{}
	bk := &mockBankCap{}
	k, ctx := setupEdgeaiWith(t, pn, nil, bk)
	ms := NewMsgServerImpl(*k)
	arb := addrOf(t)
	submitter := addrOf(t)
	params := types.DefaultParams()
	params.Arbitrator = arb
	k.SetParams(ctx, params)

	require.NoError(t, k.SetTask(ctx, &Task{Id: "1", Status: types.TaskStatusOpen}))
	require.NoError(t, k.SetResult(ctx, &Result{TaskId: "1", Submitter: submitter, Status: types.ResultStatusPending, SubmittedAtBlock: 1}))
	require.NoError(t, k.SetDispute(ctx, &Dispute{TaskId: "1", Challenger: addrOf(t), Submitter: submitter, Status: "open", Resolution: "none", OpenedAtBlock: 1}))

	_, err := ms.ResolveDispute(sdk.WrapSDKContext(ctx), &types.MsgResolveDispute{Creator: arb, TaskId: "1", Resolution: "cheat"})
	require.NoError(t, err)

	ctx = ctx.WithBlockHeight(200)
	k.BeginBlock(ctx)
	require.Empty(t, bk.modToAcct, "裁定作弊的任务不应拨付")
	task, _ := k.GetTask(ctx, "1")
	require.Equal(t, types.TaskStatusCheated, task.Status)
}

// ---------------------------------------------------------------------------
// Params
// ---------------------------------------------------------------------------

func TestParamsValidate(t *testing.T) {
	p := types.DefaultParams()
	require.NoError(t, p.Validate())

	p2 := types.DefaultParams()
	p2.DisputePeriodBlocks = 0
	require.Error(t, p2.Validate())

	p3 := types.DefaultParams()
	p3.AntiCheatThresholdBps = 10001
	require.Error(t, p3.Validate())
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func mustGetTask(t *testing.T, k *Keeper, ctx sdk.Context, id string) *Task {
	t.Helper()
	task, err := k.GetTask(ctx, id)
	require.NoError(t, err)
	return task
}

// ---------------------------------------------------------------------------
// Results / Disputes 查询
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Anti-Cheat Consensus Voting（多节点结果一致性投票）
// ---------------------------------------------------------------------------

// TestConsensusCheatDetected_ThreeNodes_OneCheat：3 节点提交同一任务，
// 2 个相同 hash（67%）+ 1 个不同 hash（33%）→ 多数派 > 50% → 少数派被 slash + rejected。
// TODO: 待 validate.go ConsensusCheatCheck 实现后启用。
func TestConsensusCheatDetected_ThreeNodes_OneCheat(t *testing.T) {
	pn := &mockPhonenode{}
	bk := &mockBankCap{}
	k, ctx := setupEdgeaiWith(t, pn, nil, bk)
	ms := NewMsgServerImpl(*k)

	nodeA := addrOf(t)
	nodeB := addrOf(t)
	nodeC := addrOf(t)

	// 走 MsgServer 路径创建任务（与已有测试一致）
	_, err := ms.CreateTask(sdk.WrapSDKContext(ctx), &types.MsgCreateTask{Creator: nodeA, Reward: 500})
	require.NoError(t, err)
	taskID := "1"

	// 3 个节点提交结果：A/B 相同 hash，C 不同
	_, err = ms.SubmitResult(sdk.WrapSDKContext(ctx.WithBlockHeight(1)), &types.MsgSubmitResult{Creator: nodeA, TaskId: taskID, ResultHash: "hash_abc", AttestationNonce: "n1"})
	require.NoError(t, err)
	_, err = ms.SubmitResult(sdk.WrapSDKContext(ctx.WithBlockHeight(2)), &types.MsgSubmitResult{Creator: nodeB, TaskId: taskID, ResultHash: "hash_abc", AttestationNonce: "n2"})
	require.NoError(t, err)
	_, err = ms.SubmitResult(sdk.WrapSDKContext(ctx.WithBlockHeight(3)), &types.MsgSubmitResult{Creator: nodeC, TaskId: taskID, ResultHash: "hash_xyz", AttestationNonce: "n3"})
	require.NoError(t, err)

	ctx = ctx.WithBlockHeight(200)
	k.BeginBlock(ctx)

	// 验证 nodeC 被 slash
	require.Len(t, pn.slashed, 1, "少数派应被 slash")
	require.Equal(t, nodeC, pn.slashed[0])

	// 验证 nodeC 结果被标记 rejected
	resC, err := k.GetResult(ctx, taskID, nodeC)
	require.NoError(t, err)
	require.Equal(t, types.ResultStatusRejected, resC.Status)

	// 验证多数派中恰好一个 valid、一个 pending（AllResults 迭代顺序由 KV 决定）
	validCount := 0
	pendingCount := 0
	rejectedCount := 0
	for _, node := range []string{nodeA, nodeB, nodeC} {
		res, _ := k.GetResult(ctx, taskID, node)
		switch res.Status {
		case types.ResultStatusValid:
			validCount++
		case types.ResultStatusPending:
			pendingCount++
		case types.ResultStatusRejected:
			rejectedCount++
		}
	}
	require.Equal(t, 1, validCount, "exactly one majority result should be valid (paid)")
	require.Equal(t, 1, pendingCount, "exactly one majority result should stay pending (task done, no double payout)")
	require.Equal(t, 1, rejectedCount, "exactly one minority result should be rejected")

	// 验证仅拨付一笔
	require.Len(t, bk.modToAcct, 1)
}

// TestConsensusCheatDetection_TwoNodes_Different：2 节点提交不同 hash（各 50%），
// 都不超过 AntiCheatThresholdBps=5000 → 无共识，均不触发自动检测。
// TODO: 待 validate.go ConsensusCheatCheck 实现后启用。
func TestConsensusCheatDetection_TwoNodes_Different(t *testing.T) {
	pn := &mockPhonenode{}
	bk := &mockBankCap{}
	k, ctx := setupEdgeaiWith(t, pn, nil, bk)

	nodeA := addrOf(t)
	nodeB := addrOf(t)

	require.NoError(t, k.SetTask(ctx, &Task{Id: "1", Status: types.TaskStatusOpen, Reward: 500}))
	require.NoError(t, k.SetResult(ctx, &Result{TaskId: "1", Submitter: nodeA, ResultHash: "hash_aaa", Status: types.ResultStatusPending, SubmittedAtBlock: 1}))
	require.NoError(t, k.SetResult(ctx, &Result{TaskId: "1", Submitter: nodeB, ResultHash: "hash_bbb", Status: types.ResultStatusPending, SubmittedAtBlock: 2}))

	ctx = ctx.WithBlockHeight(200)
	k.BeginBlock(ctx)

	// 50:50 不满足 >50% 阈值，无人被 slash
	require.Empty(t, pn.slashed, "50:50 结果分歧不应触发自动 slash")

	// 两个结果仍然是 pending（窗口过后会走乐观结算，此处只验证未被自动标记）
	resA, _ := k.GetResult(ctx, "1", nodeA)
	resB, _ := k.GetResult(ctx, "1", nodeB)
	require.NotEqual(t, types.ResultStatusRejected, resA.Status)
	require.NotEqual(t, types.ResultStatusRejected, resB.Status)
	_ = bk
}

// TestConsensusCheatDetection_DisputedTaskSkip：有争议的任务不参与自动共识检测，
// 由仲裁者裁定，不触发 auto-slash。
func TestConsensusCheatDetection_DisputedTaskSkip(t *testing.T) {
	pn := &mockPhonenode{}
	k, ctx := setupEdgeaiWith(t, pn, nil, mockBank{})

	nodeA := addrOf(t)
	nodeB := addrOf(t)

	require.NoError(t, k.SetTask(ctx, &Task{Id: "1", Status: types.TaskStatusOpen, Reward: 500}))
	require.NoError(t, k.SetResult(ctx, &Result{TaskId: "1", Submitter: nodeA, ResultHash: "hash_abc", Status: types.ResultStatusPending, SubmittedAtBlock: 1}))
	require.NoError(t, k.SetResult(ctx, &Result{TaskId: "1", Submitter: nodeB, ResultHash: "hash_xyz", Status: types.ResultStatusPending, SubmittedAtBlock: 2}))

	// 有人发起争议
	require.NoError(t, k.SetDispute(ctx, &Dispute{
		TaskId: "1", Challenger: addrOf(t), Submitter: nodeA,
		Status: "open", Resolution: "none", OpenedAtBlock: 1,
	}))

	ctx = ctx.WithBlockHeight(200)
	k.BeginBlock(ctx)

	// 有争议的任务不触发 auto-slash
	require.Empty(t, pn.slashed, "有争议的任务应跳过自动共识检测")
}

// TestConsensusCheatDetection_SingleResult：单结果任务不触发一致性检测，
// 走原有乐观结算路径（依赖争议窗口）。
func TestConsensusCheatDetection_SingleResult(t *testing.T) {
	pn := &mockPhonenode{}
	bk := &mockBankCap{}
	k, ctx := setupEdgeaiWith(t, pn, nil, bk)

	node := addrOf(t)
	require.NoError(t, k.SetTask(ctx, &Task{Id: "1", Status: types.TaskStatusOpen, Reward: 500}))
	require.NoError(t, k.SetResult(ctx, &Result{TaskId: "1", Submitter: node, ResultHash: "hash", Status: types.ResultStatusPending, SubmittedAtBlock: 1}))

	ctx = ctx.WithBlockHeight(200)
	k.BeginBlock(ctx)

	// 单结果不触发 slash，正常走拨付
	require.Empty(t, pn.slashed)
	require.Len(t, bk.modToAcct, 1, "单结果超窗口应正常拨付")
}

// TestConsensusCheatDetection_AllAgree：所有节点提交相同 hash，
// 全部为多数派 → 无人被标记 cheat。
// TODO: 待 validate.go ConsensusCheatCheck 实现后启用。
func TestConsensusCheatDetection_AllAgree(t *testing.T) {
	pn := &mockPhonenode{}
	bk := &mockBankCap{}
	k, ctx := setupEdgeaiWith(t, pn, nil, bk)

	nodeA := addrOf(t)
	nodeB := addrOf(t)
	nodeC := addrOf(t)

	require.NoError(t, k.SetTask(ctx, &Task{Id: "1", Status: types.TaskStatusOpen, Reward: 500}))
	require.NoError(t, k.SetResult(ctx, &Result{TaskId: "1", Submitter: nodeA, ResultHash: "hash_same", Status: types.ResultStatusPending, SubmittedAtBlock: 1}))
	require.NoError(t, k.SetResult(ctx, &Result{TaskId: "1", Submitter: nodeB, ResultHash: "hash_same", Status: types.ResultStatusPending, SubmittedAtBlock: 2}))
	require.NoError(t, k.SetResult(ctx, &Result{TaskId: "1", Submitter: nodeC, ResultHash: "hash_same", Status: types.ResultStatusPending, SubmittedAtBlock: 3}))

	ctx = ctx.WithBlockHeight(200)
	k.BeginBlock(ctx)

	// 全票一致，无人被 slash
	require.Empty(t, pn.slashed)

	// 仅有首个有效结果拨付，任务标记 done
	require.Len(t, bk.modToAcct, 1, "全票一致应正常拨付")
}

// TestConsensusCheatDetection_ThresholdZero：AntiCheatThresholdBps=0 时禁用自动检测。
func TestConsensusCheatDetection_ThresholdZero(t *testing.T) {
	pn := &mockPhonenode{}
	k, ctx := setupEdgeaiWith(t, pn, nil, mockBank{})

	params := types.DefaultParams()
	params.AntiCheatThresholdBps = 0
	k.SetParams(ctx, params)

	nodeA := addrOf(t)
	nodeB := addrOf(t)

	require.NoError(t, k.SetTask(ctx, &Task{Id: "1", Status: types.TaskStatusOpen, Reward: 500}))
	require.NoError(t, k.SetResult(ctx, &Result{TaskId: "1", Submitter: nodeA, ResultHash: "hash_abc", Status: types.ResultStatusPending, SubmittedAtBlock: 1}))
	require.NoError(t, k.SetResult(ctx, &Result{TaskId: "1", Submitter: nodeB, ResultHash: "hash_xyz", Status: types.ResultStatusPending, SubmittedAtBlock: 2}))

	ctx = ctx.WithBlockHeight(200)
	k.BeginBlock(ctx)

	// 阈值为 0 → 禁用，不 slash
	require.Empty(t, pn.slashed)
}

func TestQueryResultsAndDisputes(t *testing.T) {
	k, ctx := setupEdgeaiWith(t, &mockPhonenode{}, nil, mockBank{})
	submitter := addrOf(t)
	require.NoError(t, k.SetResult(ctx, &Result{TaskId: "1", Submitter: submitter, Status: types.ResultStatusPending, SubmittedAtBlock: 1}))
	require.NoError(t, k.SetDispute(ctx, &Dispute{TaskId: "1", Challenger: addrOf(t), Submitter: submitter, Status: "open", Resolution: "none", OpenedAtBlock: 1}))

	resR, err := k.Results(sdk.WrapSDKContext(ctx), &types.QueryResultsRequest{})
	require.NoError(t, err)
	require.Len(t, resR.ResultsJson, 1)

	resD, err := k.Disputes(sdk.WrapSDKContext(ctx), &types.QueryDisputesRequest{})
	require.NoError(t, err)
	require.Len(t, resD.DisputesJson, 1)
}

// =====================
// 全链路集成测试
// =====================

// TestFullLifecycle_CreateSubmitSettle：完整 Happy Path。
// 创建任务 → 提交结果 → 过争议窗口 → BeginBlock 自动结算拨付。
func TestFullLifecycle_CreateSubmitSettle(t *testing.T) {
	bk := &mockBankCap{}
	k, ctx := setupEdgeaiWith(t, &mockPhonenode{}, nil, bk)
	ms := NewMsgServerImpl(*k)
	node := addrOf(t)

	// Step 1: 创建任务（需求方付费 escrow）
	_, err := ms.CreateTask(sdk.WrapSDKContext(ctx), &types.MsgCreateTask{Creator: node, Reward: 500})
	require.NoError(t, err)

	// Step 2: 提交结果
	_, err = ms.SubmitResult(sdk.WrapSDKContext(ctx.WithBlockHeight(1)),
		&types.MsgSubmitResult{Creator: node, TaskId: "1", ResultHash: "hash_abc", AttestationNonce: "n1"})
	require.NoError(t, err)

	// Step 3: 推过争议窗口（默认 100 块）
	ctx = ctx.WithBlockHeight(200)
	k.BeginBlock(ctx)

	// Step 4: 验证拨付
	require.Len(t, bk.modToAcct, 1)
	require.Equal(t, node, bk.modToAcct[0].to)
	require.Equal(t, uint64(500), bk.modToAcct[0].amount)

	// Step 5: 验证状态
	task, _ := k.GetTask(ctx, "1")
	require.Equal(t, types.TaskStatusDone, task.Status)
	res, _ := k.GetResult(ctx, "1", node)
	require.Equal(t, types.ResultStatusValid, res.Status)
}

// TestFullLifecycle_MultiSubmitFirstWins：多节点提交同一任务，
// 首个有效结果拨付后任务 done，后续跳过。
func TestFullLifecycle_MultiSubmitFirstWins(t *testing.T) {
	bk := &mockBankCap{}
	k, ctx := setupEdgeaiWith(t, &mockPhonenode{}, nil, bk)
	ms := NewMsgServerImpl(*k)
	nodeA := addrOf(t)
	nodeB := addrOf(t)

	_, err := ms.CreateTask(sdk.WrapSDKContext(ctx), &types.MsgCreateTask{Creator: nodeA, Reward: 500})
	require.NoError(t, err)

	_, err = ms.SubmitResult(sdk.WrapSDKContext(ctx.WithBlockHeight(1)),
		&types.MsgSubmitResult{Creator: nodeA, TaskId: "1", ResultHash: "hash_abc", AttestationNonce: "n1"})
	require.NoError(t, err)
	_, err = ms.SubmitResult(sdk.WrapSDKContext(ctx.WithBlockHeight(2)),
		&types.MsgSubmitResult{Creator: nodeB, TaskId: "1", ResultHash: "hash_abc", AttestationNonce: "n2"})
	require.NoError(t, err)

	ctx = ctx.WithBlockHeight(200)
	k.BeginBlock(ctx)

	// 仅拨一笔（AllResults 迭代顺序由 KV 存储决定，不假设具体节点）
	require.Len(t, bk.modToAcct, 1)

	task, _ := k.GetTask(ctx, "1")
	require.Equal(t, types.TaskStatusDone, task.Status)

	// 两个结果中恰好一个 valid、另一个 pending（不重复拨付）
	validCount := 0
	pendingCount := 0
	for _, node := range []string{nodeA, nodeB} {
		res, _ := k.GetResult(ctx, "1", node)
		switch res.Status {
		case types.ResultStatusValid:
			validCount++
		case types.ResultStatusPending:
			pendingCount++
		}
	}
	require.Equal(t, 1, validCount, "exactly one result should be valid")
	require.Equal(t, 1, pendingCount, "exactly one result should stay pending (no double payout)")
}

// TestFullLifecycle_UnattestedSubmitRejected：未认证节点提交被拒。
func TestFullLifecycle_UnattestedSubmitRejected(t *testing.T) {
	k, ctx := setupEdgeaiWith(t, &mockPhonenodeFalse{}, nil, &mockBankCap{})
	ms := NewMsgServerImpl(*k)
	node := addrOf(t)

	_, err := ms.CreateTask(sdk.WrapSDKContext(ctx), &types.MsgCreateTask{Creator: node, Reward: 500})
	require.NoError(t, err)

	_, err = ms.SubmitResult(sdk.WrapSDKContext(ctx),
		&types.MsgSubmitResult{Creator: node, TaskId: "1", ResultHash: "x", AttestationNonce: "n"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not attested")
}

// TestFullLifecycle_EscrowInsufficient：托管金不足时创建失败。
func TestFullLifecycle_EscrowInsufficient(t *testing.T) {
	k, ctx := setupEdgeaiWith(t, &mockPhonenode{}, nil, &mockBankZero{})
	ms := NewMsgServerImpl(*k)
	node := addrOf(t)

	_, err := ms.CreateTask(sdk.WrapSDKContext(ctx), &types.MsgCreateTask{Creator: node, Reward: 500})
	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient")
}

// TestFullLifecycle_ZeroRewardTask：零奖励任务可创建可提交，BeginBlock 不拨付。
func TestFullLifecycle_ZeroRewardTask(t *testing.T) {
	bk := &mockBankCap{}
	k, ctx := setupEdgeaiWith(t, &mockPhonenode{}, nil, bk)
	ms := NewMsgServerImpl(*k)
	node := addrOf(t)

	_, err := ms.CreateTask(sdk.WrapSDKContext(ctx), &types.MsgCreateTask{Creator: node, Reward: 0})
	require.NoError(t, err)

	_, err = ms.SubmitResult(sdk.WrapSDKContext(ctx.WithBlockHeight(1)),
		&types.MsgSubmitResult{Creator: node, TaskId: "1", ResultHash: "hash", AttestationNonce: "n"})
	require.NoError(t, err)

	ctx = ctx.WithBlockHeight(200)
	k.BeginBlock(ctx)

	// 零奖励：拨付金额为 0，不产生实际资金流动
	require.Len(t, bk.modToAcct, 1)
	require.Equal(t, uint64(0), bk.modToAcct[0].amount, "zero reward task should pay 0")
	task, _ := k.GetTask(ctx, "1")
	require.Equal(t, types.TaskStatusDone, task.Status)
}

// TestFullLifecycle_DisputeHonestSettle：发起争议 → 仲裁裁定 honest → BeginBlock 正常拨付。
func TestFullLifecycle_DisputeHonestSettle(t *testing.T) {
	bk := &mockBankCap{}
	k, ctx := setupEdgeaiWith(t, &mockPhonenode{}, nil, bk)
	ms := NewMsgServerImpl(*k)
	submitter := addrOf(t)
	challenger := addrOf(t)
	arbitrator := addrOf(t)

	params := types.DefaultParams()
	params.Arbitrator = arbitrator
	k.SetParams(ctx, params)

	_, err := ms.CreateTask(sdk.WrapSDKContext(ctx), &types.MsgCreateTask{Creator: arbitrator, Reward: 500})
	require.NoError(t, err)
	_, err = ms.SubmitResult(sdk.WrapSDKContext(ctx.WithBlockHeight(1)),
		&types.MsgSubmitResult{Creator: submitter, TaskId: "1", ResultHash: "hash", AttestationNonce: "n"})
	require.NoError(t, err)
	_, err = ms.OpenDispute(sdk.WrapSDKContext(ctx.WithBlockHeight(50)),
		&types.MsgOpenDispute{Creator: challenger, TaskId: "1", Reason: "suspicious"})
	require.NoError(t, err)
	_, err = ms.ResolveDispute(sdk.WrapSDKContext(ctx.WithBlockHeight(80)),
		&types.MsgResolveDispute{Creator: arbitrator, TaskId: "1", Resolution: "honest"})
	require.NoError(t, err)

	ctx = ctx.WithBlockHeight(200)
	k.BeginBlock(ctx)

	// 争议裁定 honest → 应拨付
	require.Len(t, bk.modToAcct, 1)
	require.Equal(t, submitter, bk.modToAcct[0].to)
}

// TestFullLifecycle_DisputeCheatNoPayout：发起争议 → 仲裁裁定 cheat → BeginBlock 已 slash，跳过拨付。
func TestFullLifecycle_DisputeCheatNoPayout(t *testing.T) {
	pn := &mockPhonenode{}
	bk := &mockBankCap{}
	k, ctx := setupEdgeaiWith(t, pn, nil, bk)
	ms := NewMsgServerImpl(*k)
	submitter := addrOf(t)
	challenger := addrOf(t)
	arbitrator := addrOf(t)

	params := types.DefaultParams()
	params.Arbitrator = arbitrator
	k.SetParams(ctx, params)

	_, err := ms.CreateTask(sdk.WrapSDKContext(ctx), &types.MsgCreateTask{Creator: arbitrator, Reward: 500})
	require.NoError(t, err)
	_, err = ms.SubmitResult(sdk.WrapSDKContext(ctx.WithBlockHeight(1)),
		&types.MsgSubmitResult{Creator: submitter, TaskId: "1", ResultHash: "hash", AttestationNonce: "n"})
	require.NoError(t, err)
	_, err = ms.OpenDispute(sdk.WrapSDKContext(ctx.WithBlockHeight(50)),
		&types.MsgOpenDispute{Creator: challenger, TaskId: "1", Reason: "suspicious"})
	require.NoError(t, err)
	_, err = ms.ResolveDispute(sdk.WrapSDKContext(ctx.WithBlockHeight(80)),
		&types.MsgResolveDispute{Creator: arbitrator, TaskId: "1", Resolution: "cheat"})
	require.NoError(t, err)

	ctx = ctx.WithBlockHeight(200)
	k.BeginBlock(ctx)

	// 争议裁定 cheat + slash → 无拨付
	require.Empty(t, bk.modToAcct)
	require.Len(t, pn.slashed, 1)
	require.Equal(t, submitter, pn.slashed[0])

	task, _ := k.GetTask(ctx, "1")
	require.Equal(t, types.TaskStatusCheated, task.Status)
}
