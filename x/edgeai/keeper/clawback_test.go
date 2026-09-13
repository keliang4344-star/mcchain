package keeper

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"mcchain/x/edgeai/types"
	tokenomicstypes "mcchain/x/tokenomics/types"
)

// mockBankBurnCap records module->account sends, module->module sends and burns,
// so we can assert the clawback path routes the submitter's 85% escrow to the
// staking security pool (NOT burned) — per the "作恶者损失=诚实者收益" policy.
type mockBankBurnCap struct {
	modToAcct []bankSend
	modToMod  []bankSend
	burned    []uint64
}

func (m *mockBankBurnCap) SpendableCoins(_ sdk.Context, _ sdk.AccAddress) sdk.Coins {
	return sdk.NewCoins(sdk.NewInt64Coin("umc", 1e15))
}
func (m *mockBankBurnCap) SendCoinsFromAccountToModule(_ sdk.Context, _ sdk.AccAddress, _ string, _ sdk.Coins) error {
	return nil
}
func (m *mockBankBurnCap) SendCoinsFromModuleToAccount(_ sdk.Context, module string, to sdk.AccAddress, amt sdk.Coins) error {
	m.modToAcct = append(m.modToAcct, bankSend{module: module, to: to.String(), amount: amt.AmountOf("umc").Uint64()})
	return nil
}
func (m *mockBankBurnCap) SendCoinsFromModuleToModule(_ sdk.Context, from, to string, amt sdk.Coins) error {
	m.modToMod = append(m.modToMod, bankSend{module: from, to: to, amount: amt.AmountOf("umc").Uint64()})
	return nil
}
func (m *mockBankBurnCap) BurnCoins(_ sdk.Context, _ string, amt sdk.Coins) error {
	m.burned = append(m.burned, amt.AmountOf("umc").Uint64())
	return nil
}

// TestClawbackOnCheatResolution 验证：仲裁裁定 cheat 时，提交者托管中的
// 85% 奖励被回收（clawback）至质押安全池，且提交者被 slash、不收到任何拨付。
func TestClawbackOnCheatResolution(t *testing.T) {
	pn := &mockPhonenode{}
	bk := &mockBankBurnCap{}
	k, ctx := setupEdgeaiWith(t, pn, nil, bk)
	ms := NewMsgServerImpl(*k)

	arb := addrOf(t)
	submitter := addrOf(t)
	params := types.DefaultParams()
	params.Arbitrator = arb
	k.SetParams(ctx, params)

	require.NoError(t, k.SetTask(ctx, &Task{Id: "1", Status: types.TaskStatusOpen, Reward: 500}))
	require.NoError(t, k.SetResult(ctx, &Result{TaskId: "1", Submitter: submitter, Status: types.ResultStatusPending, SubmittedAtBlock: 1}))
	require.NoError(t, k.SetDispute(ctx, &Dispute{TaskId: "1", Challenger: addrOf(t), Submitter: submitter, Status: "open", Resolution: "none", OpenedAtBlock: 1}))

	_, err := ms.ResolveDispute(sdk.WrapSDKContext(ctx), &types.MsgResolveDispute{Creator: arb, TaskId: "1", Resolution: "cheat"})
	require.NoError(t, err)

	// 85% of 500 = 425 must be routed to the staking security pool (clawed back
	// from escrow) — per the "作恶者的损失，变成诚实者的收益" policy. It is a 罚没
	// (slash), NOT a 通缩销毁, so it must NOT be burned.
	require.Empty(t, bk.burned, "作弊罚没不应销毁，应回流质押安全池")
	var routedToSecurity uint64
	for _, s := range bk.modToMod {
		if s.to == tokenomicstypes.StakingSecurityPoolName {
			routedToSecurity += s.amount
		}
	}
	require.Equal(t, uint64(425), routedToSecurity, "cheat 裁定应将提交者 85% 托管奖励(425)回流质押安全池")
	// 提交者不应收到任何拨付。
	require.Empty(t, bk.modToAcct, "cheat 裁定不应拨付提交者")
	// 提交者被 slash。
	require.Contains(t, pn.slashed, submitter, "cheat 裁定应 slash 提交者")

	task, _ := k.GetTask(ctx, "1")
	require.Equal(t, types.TaskStatusCheated, task.Status)
}

// TestVerificationRealMatchVsCheat 验证真实验证：提交者哈希与验证者哈希一致 →
// verified；不一致 → cheat（创建争议）。
func TestVerificationRealMatchVsCheat(t *testing.T) {
	bk := &mockBankCap{}
	k, ctx, _ := setupEdgeaiWithBankFull(t, []string{addrOf(t)}, bk)

	submitter := addrOf(t)
	verifier := addrOf(t)
	require.NoError(t, k.SetTask(ctx, &Task{Id: "1", Status: types.TaskStatusDone, Reward: 500}))
	require.NoError(t, k.SetResult(ctx, &Result{TaskId: "1", Submitter: submitter, ResultHash: "hash_abc", Status: types.ResultStatusValid}))
	// 验证者奖励只能来自该任务已托管的 15% 预留池。预留池为空时不得发放
	// （朴素实现会回退到固定 1 MC 兜底奖励，实为挪用其他任务的托管金）。
	k.SetVerifierReserve(ctx, "1", 75)
	_, err := k.AssignVerification(ctx, "1", verifier)
	require.NoError(t, err)

	// 一致 → verified
	require.NoError(t, k.SubmitVerification(ctx, "1", verifier, "hash_abc"))
	v, _ := k.GetVerification(ctx, "1", verifier)
	require.True(t, v.IsHonest)
	require.True(t, v.Rewarded)
	require.Zero(t, k.GetVerifierReserve(ctx, "1"), "预留池应被一次性领空")

	// 不一致 → cheat（新验证者提交不同哈希）
	verifier2 := addrOf(t)
	_, err = k.AssignVerification(ctx, "1", verifier2)
	require.NoError(t, err)
	require.NoError(t, k.SubmitVerification(ctx, "1", verifier2, "hash_xyz"))
	d, _ := k.GetDispute(ctx, "1")
	require.NotNil(t, d, "哈希不一致应创建争议")
	require.Equal(t, "open", d.Status)
}

// TestVerifierRewardSkippedWithoutReserve 验证 任务没有托管出验证者预留池时，
// 验证通过也不得发放任何奖励——无托管即无奖励，绝不允许挪用其他任务的托管金。
func TestVerifierRewardSkippedWithoutReserve(t *testing.T) {
	bk := &mockBankCap{}
	k, ctx, _ := setupEdgeaiWithBankFull(t, []string{addrOf(t)}, bk)

	submitter := addrOf(t)
	verifier := addrOf(t)
	require.NoError(t, k.SetTask(ctx, &Task{Id: "1", Status: types.TaskStatusDone, Reward: 500}))
	require.NoError(t, k.SetResult(ctx, &Result{TaskId: "1", Submitter: submitter, ResultHash: "hash_abc", Status: types.ResultStatusValid}))
	_, err := k.AssignVerification(ctx, "1", verifier)
	require.NoError(t, err)

	// 预留池为空（未调用 SetVerifierReserve）
	require.NoError(t, k.SubmitVerification(ctx, "1", verifier, "hash_abc"))

	v, _ := k.GetVerification(ctx, "1", verifier)
	require.True(t, v.IsHonest, "哈希一致仍应判定诚实")
	require.False(t, v.Rewarded, "无托管预留时不得标记为已发放")

	var skipped bool
	for _, e := range ctx.EventManager().Events() {
		if e.Type == "edgeai.VerifierRewardSkipped" {
			skipped = true
		}
		require.NotEqual(t, "edgeai.VerifierRewarded", e.Type, "无预留时不应发出发放事件")
	}
	require.True(t, skipped, "应发出 VerifierRewardSkipped 事件以便链下追踪")
}
