package keeper

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"mcchain/x/edgeai/types"
)

// TestMsgServerSubmitRecomputeCheat 通过链上消息提交重算：与原始结果不一致 →
// 返回 cheat_detected=true、拒绝拨付；但不可逆罚没延后到仲裁裁定。
//
// 自动路径只做可逆动作（任务标 Cheated +
// 结果落 Rejected），slash / 回收 / 声誉扣减由仲裁人在 ResolveDispute(cheat)
// 执行。理由见 keeper/recompute.go 的 applyCheatOutcome 注释。
func TestMsgServerSubmitRecomputeCheat(t *testing.T) {
	k, ctx, m := setupEdgeaiFull(t, nil)
	ms := NewMsgServerImpl(*k)

	submitter := newAddr()
	challenger := newAddr()

	quickCreateTask(t, k, ctx, "mt1", submitter, 1_000_000, types.TaskStatusDisputed, 1)
	quickCreateResult(t, k, ctx, "mt1", submitter, "originalhash", "valid", 1)
	// 重算提交者必须是该任务的被指派验证者。
	_, err0 := k.AssignVerification(ctx, "mt1", challenger)
	require.NoError(t, err0)
	require.NoError(t, k.SetDispute(ctx, &types.Dispute{
		TaskId: "mt1", Challenger: challenger, Submitter: submitter, Status: "open", Resolution: "none",
	}))

	res, err := ms.SubmitRecompute(sdk.WrapSDKContext(ctx), types.NewMsgSubmitRecompute(challenger, "mt1", "different_hash"))
	require.NoError(t, err)
	require.True(t, res.CheatDetected)
	require.NotContains(t, m.slashed, submitter,
		"提交重算不得直接罚没他人质押，slash 由仲裁人裁定后执行")

	task, err := k.GetTask(ctx, "mt1")
	require.NoError(t, err)
	require.Equal(t, types.TaskStatusCheated, task.Status, "任务应立即标记作弊（拒绝拨付）")
}

// TestMsgServerSubmitRecomputeHonest 重算与原始一致 → cheat_detected=false，挑战方承担误告声誉扣减。
func TestMsgServerSubmitRecomputeHonest(t *testing.T) {
	k, ctx, _ := setupEdgeaiFull(t, nil)
	ms := NewMsgServerImpl(*k)

	submitter := newAddr()
	challenger := newAddr()

	quickCreateTask(t, k, ctx, "mt2", submitter, 1_000_000, types.TaskStatusDisputed, 1)
	quickCreateResult(t, k, ctx, "mt2", submitter, "samehash", "valid", 1)
	// 重算提交者必须是该任务的被指派验证者。
	_, err0 := k.AssignVerification(ctx, "mt2", challenger)
	require.NoError(t, err0)
	require.NoError(t, k.SetDispute(ctx, &types.Dispute{
		TaskId: "mt2", Challenger: challenger, Submitter: submitter, Status: "open", Resolution: "none",
	}))

	res, err := ms.SubmitRecompute(sdk.WrapSDKContext(ctx), types.NewMsgSubmitRecompute(challenger, "mt2", "samehash"))
	require.NoError(t, err)
	require.False(t, res.CheatDetected)

	crep, _ := k.GetReputation(ctx, challenger)
	require.Equal(t, types.DefaultReputationScore-types.ReputationFrivolousDecrease, crep.Score)
}

// TestMsgServerSubmitRecomputeRequiresDispute 未处于争议态的任务不接受重算。
func TestMsgServerSubmitRecomputeRequiresDispute(t *testing.T) {
	k, ctx, _ := setupEdgeaiFull(t, nil)
	ms := NewMsgServerImpl(*k)

	submitter := newAddr()
	challenger := newAddr()
	quickCreateTask(t, k, ctx, "mt3", submitter, 1_000_000, types.TaskStatusOpen, 1)

	_, err := ms.SubmitRecompute(sdk.WrapSDKContext(ctx), types.NewMsgSubmitRecompute(challenger, "mt3", "hash"))
	require.Error(t, err)
}

// TestMsgSubmitRecomputeValidateBasic 消息层面基础校验。
func TestMsgSubmitRecomputeValidateBasic(t *testing.T) {
	addr := newAddr()
	require.NoError(t, types.NewMsgSubmitRecompute(addr, "t", "h").ValidateBasic())
	require.Error(t, types.NewMsgSubmitRecompute("bad", "t", "h").ValidateBasic())
	require.Error(t, types.NewMsgSubmitRecompute(addr, "", "h").ValidateBasic())
	require.Error(t, types.NewMsgSubmitRecompute(addr, "t", "").ValidateBasic())
}
