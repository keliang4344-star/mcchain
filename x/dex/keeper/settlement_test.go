package keeper

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"mcchain/x/dex/types"
)

// TestSettlementBatchEndToEnd 离链批处理：提交批次 → 链上最终清算一次性拨付各接收方。
func TestSettlementBatchEndToEnd(t *testing.T) {
	k, ctx, bk := setupDex(t)
	r1 := addrOfDex(t)
	r2 := addrOfDex(t)

	// 结算源模块账户必须真实持有可动用资金，否则清算应被拒（偿付能力校验）。
	bk.setModuleBalance(types.ModuleName, "umc", 200_000_000)

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

	// 各接收方收到对应金额
	require.Equal(t, int64(100_000_000), bk.balances[r1]["umc"].Amount.Int64())
	require.Equal(t, int64(50_000_000), bk.balances[r2]["umc"].Amount.Int64())

	// 资金守恒：模块账户被实际扣减了批次总额
	require.Equal(t, int64(50_000_000),
		bk.GetBalance(ctx, sdk.MustAccAddressFromBech32(moduleAddrOf(types.ModuleName)), "umc").Amount.Int64())

	b, _ = k.GetBatch(ctx, "batch-1")
	require.Equal(t, "settled", b.Status)

	// 幂等：再次清算不改变状态、不二次拨付
	require.NoError(t, k.FinalizeBatch(ctx, "batch-1"))
	require.Equal(t, int64(100_000_000), bk.balances[r1]["umc"].Amount.Int64())
	require.Equal(t, int64(50_000_000),
		bk.GetBalance(ctx, sdk.MustAccAddressFromBech32(moduleAddrOf(types.ModuleName)), "umc").Amount.Int64())
}

// TestSettlementBatchValidation 校验非法批次被拒。
func TestSettlementBatchValidation(t *testing.T) {
	k, ctx, _ := setupDex(t)
	// 空条目
	require.Error(t, k.SubmitBatch(ctx, "b2", "root", addrOfDex(t), nil))
	// 非法接收方
	require.Error(t, k.SubmitBatch(ctx, "b3", "root", addrOfDex(t),
		[]BatchEntry{{Recipient: "not-an-address", Amount: 10}}))
	// 缺少 merkle root
	require.Error(t, k.SubmitBatch(ctx, "b4", "", addrOfDex(t),
		[]BatchEntry{{Recipient: addrOfDex(t), Amount: 10}}))
	// 零金额条目
	require.Error(t, k.SubmitBatch(ctx, "b5", "root", addrOfDex(t),
		[]BatchEntry{{Recipient: addrOfDex(t), Amount: 0}}))
}

// TestSettlementBatchIDIsUnique 同一 batch_id 不可复用：否则运营方可覆写已清算批次并重复拨付。
func TestSettlementBatchIDIsUnique(t *testing.T) {
	k, ctx, bk := setupDex(t)
	r1 := addrOfDex(t)
	bk.setModuleBalance(types.ModuleName, "umc", 1_000_000)

	entries := []BatchEntry{{Recipient: r1, Amount: 100_000}}
	require.NoError(t, k.SubmitBatch(ctx, "dup", "root", addrOfDex(t), entries))

	// pending 状态下重复提交被拒
	require.Error(t, k.SubmitBatch(ctx, "dup", "root", addrOfDex(t), entries))

	require.NoError(t, k.FinalizeBatch(ctx, "dup"))

	// settled 之后仍不可复用（防重放拨付）
	require.Error(t, k.SubmitBatch(ctx, "dup", "root", addrOfDex(t), entries))
	require.Equal(t, int64(100_000), bk.balances[r1]["umc"].Amount.Int64())
}

// TestSettlementBatchRejectsDuplicateRecipient 同批次内接收方去重。
func TestSettlementBatchRejectsDuplicateRecipient(t *testing.T) {
	k, ctx, _ := setupDex(t)
	r := addrOfDex(t)
	require.Error(t, k.SubmitBatch(ctx, "b-dup-r", "root", addrOfDex(t), []BatchEntry{
		{Recipient: r, Amount: 10},
		{Recipient: r, Amount: 20},
	}))
}

// TestSettlementBatchTotalOverflowRejected 构造 uint64 溢出的批次必须被拒，
// 否则链上记录的总额会远小于实际拨付额。
func TestSettlementBatchTotalOverflowRejected(t *testing.T) {
	k, ctx, _ := setupDex(t)
	const maxU64 = ^uint64(0)
	require.Error(t, k.SubmitBatch(ctx, "b-of", "root", addrOfDex(t), []BatchEntry{
		{Recipient: addrOfDex(t), Amount: maxU64},
		{Recipient: addrOfDex(t), Amount: 2},
	}))
}

// TestSettlementBatchTooManyEntries 超出批次条目上限必须被拒（gas griefing 防护）。
func TestSettlementBatchTooManyEntries(t *testing.T) {
	k, ctx, _ := setupDex(t)
	entries := make([]BatchEntry, types.MaxSettlementEntries+1)
	for i := range entries {
		entries[i] = BatchEntry{Recipient: addrOfDex(t), Amount: 1}
	}
	require.Error(t, k.SubmitBatch(ctx, "b-big", "root", addrOfDex(t), entries))
}

// TestSettlementCannotSpendPoolReserves 结算绝不可动用 AMM 池储备。
//
// dex 模块账户同时托管池储备与结算运营资金；若清算不区分二者，一个批次即可
// 把 LP 的钱拨走，链上池状态仍记着储备、实际代币却已不在（资不抵债）。
func TestSettlementCannotSpendPoolReserves(t *testing.T) {
	k, ctx, bk := setupDex(t)
	lp := addrOfDex(t)
	bk.setBalance(lp, "umc", 1_000_000_000)
	bk.setBalance(lp, "uusdc", 1_000_000_000)

	// 建池后，模块账户持有 500 MC 的储备（属于 LP）。
	_, err := k.CreatePool(ctx, "umc", "uusdc",
		sdk.NewInt(500_000_000), sdk.NewInt(500_000_000), 30, lp, 0)
	require.NoError(t, err)

	moduleBal := bk.GetBalance(ctx,
		sdk.MustAccAddressFromBech32(moduleAddrOf(types.ModuleName)), "umc").Amount
	require.Equal(t, int64(500_000_000), moduleBal.Int64())

	// 未被占用余额应为 0：全部 umc 都是池储备。
	require.True(t, k.UnencumberedBalance(ctx, "umc").IsZero())

	// 提交一个想动用储备的批次 → 清算必须被拒，储备分毫不动。
	r := addrOfDex(t)
	require.NoError(t, k.SubmitBatch(ctx, "raid", "root", addrOfDex(t),
		[]BatchEntry{{Recipient: r, Amount: 400_000_000}}))
	require.Error(t, k.FinalizeBatch(ctx, "raid"))

	require.Equal(t, int64(500_000_000), bk.GetBalance(ctx,
		sdk.MustAccAddressFromBech32(moduleAddrOf(types.ModuleName)), "umc").Amount.Int64())
	_, hasBal := bk.balances[r]["umc"]
	require.False(t, hasBal, "拒绝的批次不得向接收方拨付任何资金")

	batch, _ := k.GetBatch(ctx, "raid")
	require.Equal(t, "pending", batch.Status, "失败的清算不得把批次标记为 settled")

	// 追加 400 MC 运营资金后（超出储备部分），同一批次可正常清算。
	bk.credit(moduleAddrOf(types.ModuleName), sdk.NewCoins(sdk.NewCoin("umc", sdk.NewInt(400_000_000))))
	require.Equal(t, int64(400_000_000), k.UnencumberedBalance(ctx, "umc").Int64())
	require.NoError(t, k.FinalizeBatch(ctx, "raid"))
	require.Equal(t, int64(400_000_000), bk.balances[r]["umc"].Amount.Int64())
	// 池储备完好无损
	require.Equal(t, int64(500_000_000), bk.GetBalance(ctx,
		sdk.MustAccAddressFromBech32(moduleAddrOf(types.ModuleName)), "umc").Amount.Int64())
}

// TestSettlementAtomicity 任一条目失败必须整批回滚，不得部分拨付后标记 settled。
func TestSettlementAtomicity(t *testing.T) {
	k, ctx, bk := setupDex(t)
	r1 := addrOfDex(t)
	r2 := addrOfDex(t)

	// 只给 120 MC，但批次需要 150 MC。
	bk.setModuleBalance(types.ModuleName, "umc", 120_000_000)
	require.NoError(t, k.SubmitBatch(ctx, "partial", "root", addrOfDex(t), []BatchEntry{
		{Recipient: r1, Amount: 100_000_000},
		{Recipient: r2, Amount: 50_000_000},
	}))
	require.Error(t, k.FinalizeBatch(ctx, "partial"))

	// 无人收到钱，批次仍为 pending（可在补足资金后重试）。
	_, ok1 := bk.balances[r1]["umc"]
	_, ok2 := bk.balances[r2]["umc"]
	require.False(t, ok1)
	require.False(t, ok2)
	b, _ := k.GetBatch(ctx, "partial")
	require.Equal(t, "pending", b.Status)
}
