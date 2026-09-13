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

// TestRecomputeMismatchConfirmsCheat 第二验证层：重算结果与原始不一致 →
// 判定作弊、拒绝拨付，但**不**在自动路径上执行不可逆罚没。
//
// 链上无法验证「验证人自报的重算哈希」是否
// 真的经过正确计算，而 verification 由 BeginBlock 自动指派。若自动路径直接
// SlashIfBad，单个验证人「OpenDispute + SubmitRecompute(任意不同哈希)」即可
// 让诚实提交者被罚没 10% 质押（bonded 还会被 Jail），且不可逆、证据仅来自攻击者。
//
// 现在的分工：
//   - 自动路径只做可逆动作：任务标 Cheated（拒绝拨付）、pending 结果落 Rejected；
//   - 不可逆的 slash / 回收 / 声誉扣减交由仲裁人在 ResolveDispute(cheat) 执行。
//
// 本用例同时验证后半段：仲裁裁定后 slash 确实发生（威慑力未被削弱）。
func TestRecomputeMismatchConfirmsCheat(t *testing.T) {
	k, ctx, m := setupEdgeaiFull(t, nil)
	submitter := newAddr()
	challenger := newAddr()

	quickCreateTask(t, k, ctx, "t1", submitter, 1_000_000, types.TaskStatusDisputed, 1)
	quickCreateResult(t, k, ctx, "t1", submitter, "originalhash", "valid", 1)
	// 重算提交者必须是该任务的被指派验证者。
	_, err0 := k.AssignVerification(ctx, "t1", challenger)
	require.NoError(t, err0)
	require.NoError(t, k.SetDispute(ctx, &types.Dispute{
		TaskId: "t1", Challenger: challenger, Submitter: submitter, Status: "open", Resolution: "none",
	}))

	// 记录一次与原始不一致的重算
	require.NoError(t, k.RecordRecompute(ctx, "t1", challenger, "different_hash"))
	cheat, err := k.EvaluateRecompute(ctx, "t1")
	require.NoError(t, err)
	require.True(t, cheat, "重算不一致应判定作弊")

	// 自动路径不得执行不可逆罚没 —— 交由仲裁人裁定。
	require.NotContains(t, m.slashed, submitter,
		"自动重算结论不足以罚没质押；slash 必须由仲裁人裁定后执行")
	rep, _ := k.GetReputation(ctx, submitter)
	require.Equal(t, types.DefaultReputationScore, rep.Score,
		"自动路径不应扣减声誉（不可逆惩戒同样交由仲裁）")

	// 任务标记为作弊；结果被拒绝（清出 pending 索引，不再白耗结算预算）
	task, err := k.GetTask(ctx, "t1")
	require.NoError(t, err)
	require.Equal(t, types.TaskStatusCheated, task.Status)

	// 威慑力校验：仲裁人裁定 cheat 后，slash 必须真实发生。
	//
	// 注意必须走 msg server（仲裁入口）—— slash 只在这一条人工路径上执行，
	// keeper 内部的 resolveDispute 只负责状态与资金处置。
	params := k.GetParams(ctx)
	params.Arbitrator = challenger
	k.SetParams(ctx, params)
	ms := NewMsgServerImpl(*k)
	_, rerr := ms.ResolveDispute(sdk.WrapSDKContext(ctx),
		types.NewMsgResolveDispute(challenger, "t1", "cheat"))
	require.NoError(t, rerr)
	require.Contains(t, m.slashed, submitter, "仲裁裁定 cheat 后必须罚没提交者")
}

// TestRecomputeMatchPenalizesChallenger 第二验证层：重算与原始一致 → 质疑不成立，挑战方轻度扣声誉。
func TestRecomputeMatchPenalizesChallenger(t *testing.T) {
	k, ctx, _ := setupEdgeaiFull(t, nil)
	submitter := newAddr()
	challenger := newAddr()

	quickCreateTask(t, k, ctx, "t2", submitter, 1_000_000, types.TaskStatusDisputed, 1)
	quickCreateResult(t, k, ctx, "t2", submitter, "samehash", "valid", 1)
	// 重算提交者必须是该任务的被指派验证者。
	_, err0 := k.AssignVerification(ctx, "t2", challenger)
	require.NoError(t, err0)
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
