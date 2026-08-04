package keeper

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"mcchain/x/edgeai/types"
)

// mockBankBurnCap records both module->account sends and burns, so we can
// assert the clawback path burns exactly the submitter's 80% escrow.
type mockBankBurnCap struct {
	modToAcct []bankSend
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
func (m *mockBankBurnCap) SendCoinsFromModuleToModule(_ sdk.Context, _, _ string, _ sdk.Coins) error {
	return nil
}
func (m *mockBankBurnCap) BurnCoins(_ sdk.Context, _ string, amt sdk.Coins) error {
	m.burned = append(m.burned, amt.AmountOf("umc").Uint64())
	return nil
}

// TestClawbackOnCheatResolution 验证：仲裁裁定 cheat 时，提交者托管中的
// 80% 奖励被销毁（clawback），且提交者被 slash、不收到任何拨付。
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

	// 80% of 500 = 400 must be burned (clawed back from escrow).
	require.Contains(t, bk.burned, uint64(400), "cheat 裁定应销毁提交者 80% 托管奖励")
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
	_, err := k.AssignVerification(ctx, "1", verifier)
	require.NoError(t, err)

	// 一致 → verified
	require.NoError(t, k.SubmitVerification(ctx, "1", verifier, "hash_abc"))
	v, _ := k.GetVerification(ctx, "1", verifier)
	require.True(t, v.IsHonest)
	require.True(t, v.Rewarded)

	// 不一致 → cheat（新验证者提交不同哈希）
	verifier2 := addrOf(t)
	_, err = k.AssignVerification(ctx, "1", verifier2)
	require.NoError(t, err)
	require.NoError(t, k.SubmitVerification(ctx, "1", verifier2, "hash_xyz"))
	d, _ := k.GetDispute(ctx, "1")
	require.NotNil(t, d, "哈希不一致应创建争议")
	require.Equal(t, "open", d.Status)
}
