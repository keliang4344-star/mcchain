package keeper

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"mcchain/x/edgeai/types"
)

// ============================================================================
// Task 9: Phonenode + EdgeAI 跨模块集成测试
// ============================================================================

// TestAttestationGate_BlocksUnattestedNode
// 未认证节点尝试 SubmitResult 被拒。
// （此测试在 edgeai_keeper_test.go 中已有完整覆盖，此处补充边界场景）
func TestAttestationGate_BlocksUnattestedNode(t *testing.T) {
	pn := &mockPhonenodeFalse{}
	k, ctx := setupEdgeaiWith(t, pn, nil, &mockBankCap{})
	ms := NewMsgServerImpl(*k)
	node := addrOf(t)

	_, err := ms.CreateTask(sdk.WrapSDKContext(ctx),
		&types.MsgCreateTask{Creator: node, Reward: 500})
	require.NoError(t, err)

	_, err = ms.SubmitResult(sdk.WrapSDKContext(ctx),
		&types.MsgSubmitResult{Creator: node, TaskId: "1", ResultHash: "hash", AttestationNonce: "n"})
	require.Error(t, err, "未认证节点提交结果应被闸口拒绝")
	require.Contains(t, err.Error(), "not attested")
}

// TestAttestationGate_UnknownNode
// HasNode 返回 false 的节点（未注册节点）提交结果被拒。
func TestAttestationGate_UnknownNode(t *testing.T) {
	pn := &mockPhonenodeNoNode{}
	k, ctx := setupEdgeaiWith(t, pn, nil, &mockBankCap{})
	ms := NewMsgServerImpl(*k)
	node := addrOf(t)

	_, err := ms.CreateTask(sdk.WrapSDKContext(ctx),
		&types.MsgCreateTask{Creator: node, Reward: 500})
	require.NoError(t, err)

	_, err = ms.SubmitResult(sdk.WrapSDKContext(ctx),
		&types.MsgSubmitResult{Creator: node, TaskId: "1", ResultHash: "hash", AttestationNonce: "n"})
	require.Error(t, err, "未注册节点提交结果应被拒绝")
	require.Contains(t, err.Error(), "not attested")
}

// ---------------------------------------------------------------------------
// Verifier Sampling 集成测试
// ---------------------------------------------------------------------------

// TestVerifier_SampleAndVerify_Basic
// 创建任务 → 提交结果 → 结算（done 状态）→ 注册验证节点 →
// BeginBlock Phase 3 抽检 → 验证 Verification 记录创建 + 验证者收到 1 MC 奖励。
func TestVerifier_SampleAndVerify_Basic(t *testing.T) {
	verifierAddr := addrOf(t)
	bk := &mockBankCap{}

	k, ctx, m := setupEdgeaiWithBankFull(t, []string{verifierAddr}, bk)
	ms := NewMsgServerImpl(*k)

	// Step 1: 创建任务
	node := addrOf(t)
	_, err := ms.CreateTask(sdk.WrapSDKContext(ctx.WithBlockHeight(1)),
		&types.MsgCreateTask{Creator: node, Description: "inference task", Reward: 500})
	require.NoError(t, err)

	// Step 2: 提交结果
	_, err = ms.SubmitResult(sdk.WrapSDKContext(ctx.WithBlockHeight(2)),
		&types.MsgSubmitResult{Creator: node, TaskId: "1", ResultHash: "hash_abc", AttestationNonce: "n1"})
	require.NoError(t, err)

	// Step 3: 推进过争议窗口 → 结算
	bk.modToAcct = nil
	ctx = ctx.WithBlockHeight(int64(types.DefaultParams().DisputePeriodBlocks) + 10)
	k.BeginBlock(ctx)

	// 验证任务已结束
	task, _ := k.GetTask(ctx, "1")
	require.Equal(t, types.TaskStatusDone, task.Status)

	// Step 4: 再次调用 BeginBlock → Phase 3 抽检
	// 先记录当前块高 + 时间以触发随机种子
	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1)
	k.BeginBlock(ctx)

	// Step 5: 验证 Verification 记录创建
	v, err := k.GetVerification(ctx, "1", verifierAddr)
	require.NoError(t, err)
	require.NotNil(t, v, "应创建 Verification 记录")
	require.Equal(t, "1", v.TaskId)
	require.Equal(t, verifierAddr, v.Verifier)
	require.True(t, v.IsHonest)
	require.True(t, v.Rewarded)
}

// TestVerifier_SampleAndVerify_RewardPaid
// 验证者应收到 1 MC (1000000 umc) 奖励。
func TestVerifier_SampleAndVerify_RewardPaid(t *testing.T) {
	verifierAddr := addrOf(t)
	bk := &mockBankCap{}

	k, ctx, _ := setupEdgeaiWithBankFull(t, []string{verifierAddr}, bk)
	ms := NewMsgServerImpl(*k)

	node := addrOf(t)
	_, err := ms.CreateTask(sdk.WrapSDKContext(ctx.WithBlockHeight(1)),
		&types.MsgCreateTask{Creator: node, Description: "task", Reward: 500})
	require.NoError(t, err)

	_, err = ms.SubmitResult(sdk.WrapSDKContext(ctx.WithBlockHeight(2)),
		&types.MsgSubmitResult{Creator: node, TaskId: "1", ResultHash: "h", AttestationNonce: "n"})
	require.NoError(t, err)

	// 结算
	bk.modToAcct = nil
	ctx = ctx.WithBlockHeight(int64(types.DefaultParams().DisputePeriodBlocks) + 10)
	k.BeginBlock(ctx)

	// 抽检
	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1)
	k.BeginBlock(ctx)

	// 验证者奖励（1 MC = 1000000 umc）应在 modToAcct 记录中
	foundVerifierReward := false
	for _, send := range bk.modToAcct {
		if send.to == verifierAddr && send.amount == types.VerifierRewardPerSample {
			foundVerifierReward = true
			break
		}
	}
	require.True(t, foundVerifierReward, "验证者应收到 VerifierRewardPerSample 奖励")
}

// TestVerifier_NoEligibleVerifier_SkipsSampling
// 无合格验证节点时，Phase 3 跳过抽检。
func TestVerifier_NoEligibleVerifier_SkipsSampling(t *testing.T) {
	bk := &mockBankCap{}
	k, ctx, _ := setupEdgeaiWithBankFull(t, []string{}, bk)
	ms := NewMsgServerImpl(*k)

	node := addrOf(t)
	_, err := ms.CreateTask(sdk.WrapSDKContext(ctx.WithBlockHeight(1)),
		&types.MsgCreateTask{Creator: node, Description: "task", Reward: 500})
	require.NoError(t, err)

	_, err = ms.SubmitResult(sdk.WrapSDKContext(ctx.WithBlockHeight(2)),
		&types.MsgSubmitResult{Creator: node, TaskId: "1", ResultHash: "h", AttestationNonce: "n"})
	require.NoError(t, err)

	ctx = ctx.WithBlockHeight(int64(types.DefaultParams().DisputePeriodBlocks) + 10)
	k.BeginBlock(ctx)

	// 抽检阶段：无合格验证节点
	bk.modToAcct = nil
	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1)
	k.BeginBlock(ctx)

	// 不应创建 Verification 记录
	allV := k.AllVerifications(ctx)
	require.Empty(t, allV, "无验证节点时不应创建 Verification")
}

// TestVerifier_NoDoneTask_SkipsSampling
// 无 done 状态任务时，验证节点不会被分配。
func TestVerifier_NoDoneTask_SkipsSampling(t *testing.T) {
	verifierAddr := addrOf(t)
	bk := &mockBankCap{}
	k, ctx, _ := setupEdgeaiWithBankFull(t, []string{verifierAddr}, bk)

	// 创建任务但不提交（仍 open）
	node := addrOf(t)
	quickCreateTask(t, k, ctx, "1", node, 500, types.TaskStatusOpen, 1)

	ctx = ctx.WithBlockHeight(200)
	k.BeginBlock(ctx)

	allV := k.AllVerifications(ctx)
	require.Empty(t, allV, "无 done 任务时不应分配验证")
}

// TestVerifier_DuplicateSamplingSkipped
// 同一任务已被同一验证节点抽检过，不应重复分配。
func TestVerifier_DuplicateSamplingSkipped(t *testing.T) {
	verifierAddr := addrOf(t)
	bk := &mockBankCap{}
	k, ctx, _ := setupEdgeaiWithBankFull(t, []string{verifierAddr}, bk)
	ms := NewMsgServerImpl(*k)

	// 创建 + 提交 + 结算
	node := addrOf(t)
	_, err := ms.CreateTask(sdk.WrapSDKContext(ctx.WithBlockHeight(1)),
		&types.MsgCreateTask{Creator: node, Description: "task", Reward: 500})
	require.NoError(t, err)
	_, err = ms.SubmitResult(sdk.WrapSDKContext(ctx.WithBlockHeight(2)),
		&types.MsgSubmitResult{Creator: node, TaskId: "1", ResultHash: "h", AttestationNonce: "n"})
	require.NoError(t, err)

	ctx = ctx.WithBlockHeight(int64(types.DefaultParams().DisputePeriodBlocks) + 10)
	k.BeginBlock(ctx)

	// 第一次抽检
	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1)
	k.BeginBlock(ctx)

	v1, _ := k.GetVerification(ctx, "1", verifierAddr)
	require.NotNil(t, v1)
	beforeCount := len(k.AllVerifications(ctx))

	// 第二次抽检（同一任务 + 同一验证节点）
	bk.modToAcct = nil
	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 10)
	k.BeginBlock(ctx)

	afterCount := len(k.AllVerifications(ctx))
	// HasVerification 应阻止重复分配，总数不变
	require.Equal(t, beforeCount, afterCount, "同一任务+验证节点不应重复分配")
}

// TestVerifier_SlashCalledOnCheatSubmission
// SubmitVerification 传入 isHonest=false → 自动创建 Dispute。
func TestVerifier_SlashCalledOnCheatSubmission(t *testing.T) {
	verifierAddr := addrOf(t)
	bk := &mockBankCap{}
	k, ctx, _ := setupEdgeaiWithBankFull(t, []string{verifierAddr}, bk)
	ms := NewMsgServerImpl(*k)

	node := addrOf(t)
	_, err := ms.CreateTask(sdk.WrapSDKContext(ctx.WithBlockHeight(1)),
		&types.MsgCreateTask{Creator: node, Description: "task", Reward: 500})
	require.NoError(t, err)
	_, err = ms.SubmitResult(sdk.WrapSDKContext(ctx.WithBlockHeight(2)),
		&types.MsgSubmitResult{Creator: node, TaskId: "1", ResultHash: "h", AttestationNonce: "n"})
	require.NoError(t, err)

	ctx = ctx.WithBlockHeight(int64(types.DefaultParams().DisputePeriodBlocks) + 10)
	k.BeginBlock(ctx)

	// 手动创建 Verification 记录（模拟 assigned 状态）
	_, err = k.AssignVerification(ctx, "1", verifierAddr)
	require.NoError(t, err)

	// 提交 dishonest 结果
	err = k.SubmitVerification(ctx, "1", verifierAddr, false, "proof:bad")
	require.NoError(t, err)

	// 验证 Dispute 创建
	d, err := k.GetDispute(ctx, "1")
	require.NoError(t, err)
	require.NotNil(t, d, "cheat 验证应自动创建 Dispute")
	require.Equal(t, "open", d.Status)
	require.Equal(t, verifierAddr, d.Challenger)

	// 任务状态应变为 disputed
	task, _ := k.GetTask(ctx, "1")
	require.Equal(t, types.TaskStatusDisputed, task.Status)
}

// ---------------------------------------------------------------------------
// mock stubs
// ---------------------------------------------------------------------------

// mockPhonenodeNoNode: HasNode returns false (unknown node).
type mockPhonenodeNoNode struct{}

func (m *mockPhonenodeNoNode) HasNode(ctx sdk.Context, addr string) bool    { return false }
func (m *mockPhonenodeNoNode) IsAttested(ctx sdk.Context, addr string) bool { return false }
func (m *mockPhonenodeNoNode) SlashIfBad(ctx sdk.Context, addr, reason string, bps uint32) error {
	return nil
}
func (m *mockPhonenodeNoNode) GetVerifierNodes(ctx sdk.Context) []string { return nil }
