package keeper

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"mcchain/x/edgeai/types"
)

// TestMsgServerSubmitRecomputeCheat 通过链上消息提交重算：与原始结果不一致 → 返回 cheat_detected=true。
func TestMsgServerSubmitRecomputeCheat(t *testing.T) {
	k, ctx, m := setupEdgeaiFull(t, nil)
	ms := NewMsgServerImpl(*k)

	submitter := newAddr()
	challenger := newAddr()

	quickCreateTask(t, k, ctx, "mt1", submitter, 1_000_000, types.TaskStatusDisputed, 1)
	quickCreateResult(t, k, ctx, "mt1", submitter, "originalhash", "valid", 1)
	require.NoError(t, k.SetDispute(ctx, &types.Dispute{
		TaskId: "mt1", Challenger: challenger, Submitter: submitter, Status: "open", Resolution: "none",
	}))

	res, err := ms.SubmitRecompute(sdk.WrapSDKContext(ctx), types.NewMsgSubmitRecompute(challenger, "mt1", "different_hash"))
	require.NoError(t, err)
	require.True(t, res.CheatDetected)
	require.Contains(t, m.slashed, submitter)

	task, err := k.GetTask(ctx, "mt1")
	require.NoError(t, err)
	require.Equal(t, types.TaskStatusCheated, task.Status)
}

// TestMsgServerSubmitRecomputeHonest 重算与原始一致 → cheat_detected=false，挑战方承担误告声誉扣减。
func TestMsgServerSubmitRecomputeHonest(t *testing.T) {
	k, ctx, _ := setupEdgeaiFull(t, nil)
	ms := NewMsgServerImpl(*k)

	submitter := newAddr()
	challenger := newAddr()

	quickCreateTask(t, k, ctx, "mt2", submitter, 1_000_000, types.TaskStatusDisputed, 1)
	quickCreateResult(t, k, ctx, "mt2", submitter, "samehash", "valid", 1)
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
