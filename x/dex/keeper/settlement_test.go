package keeper

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSettlementBatchEndToEnd 离链批处理：提交批次 → 链上最终清算一次性拨付各接收方。
func TestSettlementBatchEndToEnd(t *testing.T) {
	k, ctx, bk := setupDex(t)
	r1 := addrOfDex(t)
	r2 := addrOfDex(t)

	entries := []BatchEntry{
		{Recipient: r1, Amount: 100_000_000}, // 100 MC
		{Recipient: r2, Amount: 50_000_000},  // 50 MC
	}
	require.NoError(t, k.SubmitBatch(ctx, "batch-1", "deadbeef", addrOfDex(t), entries))

	b, ok := k.GetBatch(ctx, "batch-1")
	require.True(t, ok)
	require.Equal(t, uint64(150_000_000), b.Total)
	require.Equal(t, "pending", b.Status)

	// 最终清算
	require.NoError(t, k.FinalizeBatch(ctx, "batch-1"))

	// 各接收方收到对应金额（mock 在 SendCoinsFromModuleToAccount 时累加）
	require.Equal(t, int64(100_000_000), bk.balances[r1]["umc"].Amount.Int64())
	require.Equal(t, int64(50_000_000), bk.balances[r2]["umc"].Amount.Int64())

	b, _ = k.GetBatch(ctx, "batch-1")
	require.Equal(t, "settled", b.Status)

	// 幂等：再次清算不改变状态
	require.NoError(t, k.FinalizeBatch(ctx, "batch-1"))
}

// TestSettlementBatchValidation 校验非法批次被拒。
func TestSettlementBatchValidation(t *testing.T) {
	k, ctx, _ := setupDex(t)
	// 空条目
	require.Error(t, k.SubmitBatch(ctx, "b2", "root", addrOfDex(t), nil))
	// 非法接收方
	require.Error(t, k.SubmitBatch(ctx, "b3", "root", addrOfDex(t),
		[]BatchEntry{{Recipient: "not-an-address", Amount: 10}}))
}
