package keeper

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"mcchain/x/edgeai/types"
)

func newAddr() string {
	return sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
}

// TestRecomputeMismatchConfirmsCheat 第二验证层：重算结果与原始不一致 → 作弊确认。
func TestRecomputeMismatchConfirmsCheat(t *testing.T) {
	k, ctx, m := setupEdgeaiFull(t, nil)
	submitter := newAddr()
	challenger := newAddr()

	quickCreateTask(t, k, ctx, "t1", submitter, 1_000_000, types.TaskStatusDisputed, 1)
	quickCreateResult(t, k, ctx, "t1", submitter, "originalhash", "valid", 1)
	require.NoError(t, k.SetDispute(ctx, &types.Dispute{
		TaskId: "t1", Challenger: challenger, Submitter: submitter, Status: "open", Resolution: "none",
	}))

	// 记录一次与原始不一致的重算
	require.NoError(t, k.RecordRecompute(ctx, "t1", challenger, "different_hash"))
	cheat, err := k.EvaluateRecompute(ctx, "t1")
	require.NoError(t, err)
	require.True(t, cheat, "重算不一致应判定作弊")

	// 提交者被 slash + 声誉扣减
	require.Contains(t, m.slashed, submitter)
	rep, _ := k.GetReputation(ctx, submitter)
	require.Equal(t, types.DefaultReputationScore-types.ReputationCheatDecrease, rep.Score)

	// 任务标记为作弊
	task, err := k.GetTask(ctx, "t1")
	require.NoError(t, err)
	require.Equal(t, types.TaskStatusCheated, task.Status)
}

// TestRecomputeMatchPenalizesChallenger 第二验证层：重算与原始一致 → 质疑不成立，挑战方轻度扣声誉。
func TestRecomputeMatchPenalizesChallenger(t *testing.T) {
	k, ctx, _ := setupEdgeaiFull(t, nil)
	submitter := newAddr()
	challenger := newAddr()

	quickCreateTask(t, k, ctx, "t2", submitter, 1_000_000, types.TaskStatusDisputed, 1)
	quickCreateResult(t, k, ctx, "t2", submitter, "samehash", "valid", 1)
	require.NoError(t, k.SetDispute(ctx, &types.Dispute{
		TaskId: "t2", Challenger: challenger, Submitter: submitter, Status: "open", Resolution: "none",
	}))

	// 重算与原始一致
	require.NoError(t, k.RecordRecompute(ctx, "t2", challenger, "samehash"))
	cheat, err := k.EvaluateRecompute(ctx, "t2")
	require.NoError(t, err)
	require.False(t, cheat, "重算一致不应判定作弊")

	// 挑战方声誉轻度扣减（误告惩戒）
	crep, _ := k.GetReputation(ctx, challenger)
	require.Equal(t, types.DefaultReputationScore-types.ReputationFrivolousDecrease, crep.Score)
	// 提交者声誉不受影响
	srep, _ := k.GetReputation(ctx, submitter)
	require.Equal(t, types.DefaultReputationScore, srep.Score)
}

// TestRecomputeRequiresDispute 非争议态任务不可记录重算。
func TestRecomputeRequiresDispute(t *testing.T) {
	k, ctx, _ := setupEdgeaiFull(t, nil)
	submitter := newAddr()
	quickCreateTask(t, k, ctx, "t3", submitter, 1_000_000, types.TaskStatusOpen, 1)
	quickCreateResult(t, k, ctx, "t3", submitter, "h", "valid", 1)
	err := k.RecordRecompute(ctx, "t3", newAddr(), "h2")
	require.Error(t, err, "非争议态不应允许记录重算")
}
