package keeper

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"mcchain/x/dex/types"
)

// TestMsgServerSettlementBatchFlow 通过链上消息完成离链批处理：提交批次 → 清算拨付。
func TestMsgServerSettlementBatchFlow(t *testing.T) {
	k, ctx, bk := setupDex(t)
	ms := NewMsgServerImpl(*k)
	goCtx := sdk.WrapSDKContext(ctx)

	// 将结算授权方配置为测试提交者（默认是治理模块账户）。
	submitter := addrOfDex(t)
	k.SetSettlementConfig(ctx, SettlementConfig{Authority: submitter})
	r1 := addrOfDex(t)
	r2 := addrOfDex(t)

	submitRes, err := ms.SubmitSettlementBatch(goCtx, &types.MsgSubmitSettlementBatch{
		Creator:    submitter,
		BatchId:    "msg-batch-1",
		MerkleRoot: "deadbeef",
		Entries: []*types.SettlementEntry{
			{Recipient: r1, AmountUmc: 100_000_000},
			{Recipient: r2, AmountUmc: 50_000_000},
		},
	})
	require.NoError(t, err)
	require.Equal(t, uint64(150_000_000), submitRes.TotalUmc)
	require.Equal(t, uint64(2), submitRes.EntryCount)

	b, ok := k.GetBatch(ctx, "msg-batch-1")
	require.True(t, ok)
	require.Equal(t, "pending", b.Status)

	_, err = ms.FinalizeSettlementBatch(goCtx, &types.MsgFinalizeSettlementBatch{
		Creator: submitter,
		BatchId: "msg-batch-1",
	})
	require.NoError(t, err)

	require.Equal(t, int64(100_000_000), bk.balances[r1]["umc"].Amount.Int64())
	require.Equal(t, int64(50_000_000), bk.balances[r2]["umc"].Amount.Int64())

	b, _ = k.GetBatch(ctx, "msg-batch-1")
	require.Equal(t, "settled", b.Status)
}

// TestMsgServerSettlementBatchRejectsInvalid 非法批次经 msg server 必须被拒。
func TestMsgServerSettlementBatchRejectsInvalid(t *testing.T) {
	k, ctx, _ := setupDex(t)
	ms := NewMsgServerImpl(*k)
	goCtx := sdk.WrapSDKContext(ctx)

	// 空条目
	_, err := ms.SubmitSettlementBatch(goCtx, &types.MsgSubmitSettlementBatch{
		Creator: addrOfDex(t), BatchId: "b-empty", MerkleRoot: "root",
	})
	require.Error(t, err)

	// 非法接收方
	_, err = ms.SubmitSettlementBatch(goCtx, &types.MsgSubmitSettlementBatch{
		Creator: addrOfDex(t), BatchId: "b-bad", MerkleRoot: "root",
		Entries: []*types.SettlementEntry{{Recipient: "not-an-address", AmountUmc: 10}},
	})
	require.Error(t, err)

	// 清算一个不存在的批次
	_, err = ms.FinalizeSettlementBatch(goCtx, &types.MsgFinalizeSettlementBatch{
		Creator: addrOfDex(t), BatchId: "no-such-batch",
	})
	require.Error(t, err)

	_, ok := k.GetBatch(ctx, "b-empty")
	require.False(t, ok)
}

// TestMsgServerSettlementUnauthorized 越权与熔断必须被拒（A1 / A4）。
func TestMsgServerSettlementUnauthorized(t *testing.T) {
	k, ctx, _ := setupDex(t)
	ms := NewMsgServerImpl(*k)
	goCtx := sdk.WrapSDKContext(ctx)

	// 授权方设为与提交者不同的地址。
	authority := addrOfDex(t)
	attacker := addrOfDex(t)
	k.SetSettlementConfig(ctx, SettlementConfig{Authority: authority})

	// 越权提交
	_, err := ms.SubmitSettlementBatch(goCtx, &types.MsgSubmitSettlementBatch{
		Creator: attacker, BatchId: "b-attack", MerkleRoot: "root",
		Entries: []*types.SettlementEntry{{Recipient: addrOfDex(t), AmountUmc: 1}},
	})
	require.Error(t, err)

	// 越权清算
	_, err = ms.FinalizeSettlementBatch(goCtx, &types.MsgFinalizeSettlementBatch{
		Creator: attacker, BatchId: "b-attack",
	})
	require.Error(t, err)

	// 熔断：即使授权方提交也被拒
	k.SetSettlementConfig(ctx, SettlementConfig{Authority: authority, Halted: true})
	_, err = ms.SubmitSettlementBatch(goCtx, &types.MsgSubmitSettlementBatch{
		Creator: authority, BatchId: "b-halt", MerkleRoot: "root",
		Entries: []*types.SettlementEntry{{Recipient: addrOfDex(t), AmountUmc: 1}},
	})
	require.Error(t, err)
}

// TestSettlementMsgValidateBasic 消息层面基础校验。
func TestSettlementMsgValidateBasic(t *testing.T) {
	creator := addrOfDex(t)
	recipient := addrOfDex(t)

	valid := &types.MsgSubmitSettlementBatch{
		Creator: creator, BatchId: "b1", MerkleRoot: "root",
		Entries: []*types.SettlementEntry{{Recipient: recipient, AmountUmc: 1}},
	}
	require.NoError(t, valid.ValidateBasic())

	require.Error(t, (&types.MsgSubmitSettlementBatch{
		Creator: "bad", BatchId: "b1", MerkleRoot: "root",
		Entries: []*types.SettlementEntry{{Recipient: recipient, AmountUmc: 1}},
	}).ValidateBasic())

	require.Error(t, (&types.MsgSubmitSettlementBatch{
		Creator: creator, BatchId: "", MerkleRoot: "root",
		Entries: []*types.SettlementEntry{{Recipient: recipient, AmountUmc: 1}},
	}).ValidateBasic())

	require.Error(t, (&types.MsgSubmitSettlementBatch{
		Creator: creator, BatchId: "b1", MerkleRoot: "root",
		Entries: []*types.SettlementEntry{{Recipient: recipient, AmountUmc: 0}},
	}).ValidateBasic())

	require.NoError(t, (&types.MsgFinalizeSettlementBatch{Creator: creator, BatchId: "b1"}).ValidateBasic())
	require.Error(t, (&types.MsgFinalizeSettlementBatch{Creator: creator, BatchId: ""}).ValidateBasic())
}
