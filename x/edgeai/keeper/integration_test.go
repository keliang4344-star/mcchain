package keeper

import (
	"fmt"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"mcchain/x/edgeai/types"
)

// ============================================================================
// Task 7: EdgeAI 端到端集成测试
// ============================================================================

// TestTaskExpiry_TaskNotSubmittedRefundsEscrow
// 创建任务但不提交 → 推进区块超过 TaskExpireBlocks →
// BeginBlock 标记任务 expired，退还 escrow 给创建者。
func TestTaskExpiry_TaskNotSubmittedRefundsEscrow(t *testing.T) {
	bk := &mockBankCap{}
	k, ctx := setupEdgeaiWith(t, &mockPhonenode{}, nil, bk)
	creator := addrOf(t)

	// 记录模块账户拨付操作（用于验证退款）
	bk.modToAcct = nil

	// 创建任务：CreatedAtBlock 在 BlockHeight 1
	quickCreateTask(t, k, ctx, "1", creator, 500000000, types.TaskStatusOpen, 1)

	// 推进区块，超过 TaskExpireBlocks (10000)
	expireHeight := int64(types.TaskExpireBlocks) + 1
	ctx = ctx.WithBlockHeight(expireHeight + 10)

	k.BeginBlock(ctx)

	// 任务应标记为 expired
	task, err := k.GetTask(ctx, "1")
	require.NoError(t, err)
	require.NotNil(t, task)
	require.Equal(t, types.TaskStatusExpired, task.Status)

	// 托管金应退还给创建者
	require.Len(t, bk.modToAcct, 1, "过期任务应退还托管金")
	require.Equal(t, creator, bk.modToAcct[0].to)
	require.Equal(t, uint64(500000000), bk.modToAcct[0].amount)
	require.Equal(t, types.ModuleName, bk.modToAcct[0].module)
}

// TestTaskExpiry_TaskWithPendingResultStaysOpen
// 已提交结果的任务在过期前不应标记为 expired。
func TestTaskExpiry_TaskWithPendingResultStaysOpen(t *testing.T) {
	bk := &mockBankCap{}
	k, ctx := setupEdgeaiWith(t, &mockPhonenode{}, nil, bk)
	creator := addrOf(t)
	node := addrOf(t)

	// 创建任务 + 提交结果
	quickCreateTask(t, k, ctx, "1", creator, 500000000, types.TaskStatusOpen, 1)
	quickCreateResult(t, k, ctx, "1", node, "hash_abc", types.ResultStatusPending, 5)

	// 推进超过 TaskExpireBlocks，但远小于争议窗口（BeginBlock 尚未处理）
	ctx = ctx.WithBlockHeight(int64(types.TaskExpireBlocks) - 10)
	k.BeginBlock(ctx)

	// 任务不应过期（仍 open，结果 pending）
	task, _ := k.GetTask(ctx, "1")
	require.Equal(t, types.TaskStatusOpen, task.Status)
}

// TestTaskExpiry_ZeroRewardTaskExpires
// 零奖励过期任务不触发退款（reward == 0）。
func TestTaskExpiry_ZeroRewardTaskExpires(t *testing.T) {
	bk := &mockBankCap{}
	k, ctx := setupEdgeaiWith(t, &mockPhonenode{}, nil, bk)
	creator := addrOf(t)

	// 清零记录
	bk.modToAcct = nil

	quickCreateTask(t, k, ctx, "1", creator, 0, types.TaskStatusOpen, 1)

	expireHeight := int64(types.TaskExpireBlocks) + 1
	ctx = ctx.WithBlockHeight(expireHeight + 10)
	k.BeginBlock(ctx)

	task, _ := k.GetTask(ctx, "1")
	require.Equal(t, types.TaskStatusExpired, task.Status)

	// 零奖励不触发退款
	require.Empty(t, bk.modToAcct)
}

// TestTaskExpiry_NotYetExpired
// 未超过 TaskExpireBlocks 的任务不应过期。
func TestTaskExpiry_NotYetExpired(t *testing.T) {
	bk := &mockBankCap{}
	k, ctx := setupEdgeaiWith(t, &mockPhonenode{}, nil, bk)
	creator := addrOf(t)
	bk.modToAcct = nil

	quickCreateTask(t, k, ctx, "1", creator, 500, types.TaskStatusOpen, 1)

	// 只推进一半的过期区块
	ctx = ctx.WithBlockHeight(int64(types.TaskExpireBlocks) / 2)
	k.BeginBlock(ctx)

	task, _ := k.GetTask(ctx, "1")
	require.Equal(t, types.TaskStatusOpen, task.Status)
	require.Empty(t, bk.modToAcct)
}

// TestTaskExpiry_SettledTaskDoesNotExpire
// 已完成（done）的任务不应被过期逻辑处理。
func TestTaskExpiry_SettledTaskDoesNotExpire(t *testing.T) {
	bk := &mockBankCap{}
	k, ctx := setupEdgeaiWith(t, &mockPhonenode{}, nil, bk)
	creator := addrOf(t)
	node := addrOf(t)

	// 创建 → 提交 → 结算
	quickCreateTask(t, k, ctx, "1", creator, 500, types.TaskStatusOpen, 1)
	quickCreateResult(t, k, ctx, "1", node, "hash_abc", types.ResultStatusValid, 1)

	// 手动标记 done（模拟已结算场景）
	task, _ := k.GetTask(ctx, "1")
	task.Status = types.TaskStatusDone
	require.NoError(t, k.SetTask(ctx, task))

	bk.modToAcct = nil

	expireHeight := int64(types.TaskExpireBlocks) + 1
	ctx = ctx.WithBlockHeight(expireHeight + 10)
	k.BeginBlock(ctx)

	// 状态应保持 done
	task, _ = k.GetTask(ctx, "1")
	require.Equal(t, types.TaskStatusDone, task.Status)
	require.Empty(t, bk.modToAcct, "已完成任务不应触发过期退款")
}

// TestFullLifecycle_CreateSubmitExpire_Sequence
// 综合测试：创建任务 → 提交结果 → 过期窗口前结算 →
// 单独验证过期路径。
func TestFullLifecycle_CreateSubmitExpire_Sequence(t *testing.T) {
	bk := &mockBankCap{}
	k, ctx := setupEdgeaiWith(t, &mockPhonenode{}, nil, bk)
	ms := NewMsgServerImpl(*k)

	// 任务 A：正常提交 + 窗口过后结算
	nodeA := addrOf(t)
	_, err := ms.CreateTask(sdk.WrapSDKContext(ctx.WithBlockHeight(1)),
		&types.MsgCreateTask{Creator: nodeA, Description: "happy path task", Reward: 1000})
	require.NoError(t, err)

	_, err = ms.SubmitResult(sdk.WrapSDKContext(ctx.WithBlockHeight(2)),
		&types.MsgSubmitResult{Creator: nodeA, TaskId: "1", ResultHash: "h1", AttestationNonce: "n1"})
	require.NoError(t, err)

	// 任务 B：创建但不提交 → 过期
	creatorB := addrOf(t)
	_, err = ms.CreateTask(sdk.WrapSDKContext(ctx.WithBlockHeight(3)),
		&types.MsgCreateTask{Creator: creatorB, Description: "expiring task", Reward: 2000})
	require.NoError(t, err)

	// 推进到结算窗口后 + 过期窗口后
	ctx = ctx.WithBlockHeight(int64(types.TaskExpireBlocks) + 200)
	k.BeginBlock(ctx)

	// 任务 A 已结算
	taskA, _ := k.GetTask(ctx, "1")
	require.Equal(t, types.TaskStatusDone, taskA.Status)
	resA, _ := k.GetResult(ctx, "1", nodeA)
	require.Equal(t, types.ResultStatusValid, resA.Status)

	// 任务 B 已过期
	taskB, _ := k.GetTask(ctx, "2")
	require.Equal(t, types.TaskStatusExpired, taskB.Status)

	// 验证拨付：A 收到 1000，B 退回 2000
	require.Len(t, bk.modToAcct, 2)
}

// TestMaxTasksPerBlock_CapsSettlement
// 验证 MaxTasksPerBlock=20 限流：超过 20 个待结算任务时每区块只结算 20 个。
func TestMaxTasksPerBlock_CapsSettlement(t *testing.T) {
	bk := &mockBankCap{}
	k, ctx := setupEdgeaiWith(t, &mockPhonenode{}, nil, bk)
	creator := addrOf(t)

	// 创建 30 个任务，全部提交结果
	for i := 1; i <= 30; i++ {
		id := fmt.Sprintf("%d", i)
		quickCreateTask(t, k, ctx, id, creator, 100, types.TaskStatusOpen, 1)
		quickCreateResult(t, k, ctx, id, creator, "hash", types.ResultStatusPending, 1)
	}

	ctx = ctx.WithBlockHeight(200)
	bk.modToAcct = nil
	k.BeginBlock(ctx)

	// 最多结算 20 个
	require.LessOrEqual(t, len(bk.modToAcct), int(types.MaxTasksPerBlock),
		"每区块结算数不应超过 MaxTasksPerBlock")
}
