package keeper

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRefundExpiredVerifierReservesBudgetCountsScanned 是容量预算口径在 edgeai
// 侧的回归闸门（与 x/depin/keeper/defense_gc_test.go 同一条口径）。
//
// 回归守则：循环上界不能用「已退款条数」计（refunded < budget），否则未过期
// 条目 continue 不消耗预算，稳态下循环扫穿整个预留池前缀。BeginBlock 跑在
// InfiniteGasMeter 下，超时即全网停块且不可自愈。
//
// 判据：全部写入未过期预留，跑一轮 GC，游标必须停在第 budget 条；
// 若退化成整前缀扫描，游标会落在最后一条。
func TestRefundExpiredVerifierReservesBudgetCountsScanned(t *testing.T) {
	k, ctx, _ := setupEdgeai(t)

	// settleHeight = 当前高度 ⇒ now-settle = 0 < 过期窗口 ⇒ 全部未过期。
	const height = 1000
	ctx = ctx.WithBlockHeight(height)

	n := VerifierReserveGCPerBlock * 4
	for i := 0; i < n; i++ {
		k.SetVerifierReserveAt(ctx, fmt.Sprintf("task-%06d", i), 1000, height)
	}

	k.refundExpiredVerifierReserves(ctx)

	cursor := ctx.KVStore(k.storeKey).Get(verifierReserveCursorKey)
	require.NotNil(t, cursor, "未扫到预算耗尽就结束说明循环没有按扫描条数收敛")
	require.Equal(t, fmt.Sprintf("task-%06d", VerifierReserveGCPerBlock), string(cursor),
		"游标必须停在预算耗尽处（第 %d 条），落在末条说明已退化为整前缀扫描（停链级回归）",
		VerifierReserveGCPerBlock)

	// 未过期预留一条都不能被动。
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("task-%06d", i)
		require.Equal(t, uint64(1000), k.GetVerifierReserve(ctx, id), "第 %d 条未过期预留被误退", i)
	}
}

// TestRefundExpiredVerifierReservesRefundsAfterExpiry 证明过期预留确实会被退还，
// 且单轮处理量受预算约束。
func TestRefundExpiredVerifierReservesRefundsAfterExpiry(t *testing.T) {
	k, ctx, _ := setupEdgeai(t)

	// 结算高度远早于当前高度（超过 7 天窗口）⇒ 应被判过期。
	ctx = ctx.WithBlockHeight(int64(VerifierReserveExpiryBlocks) + 100)
	n := VerifierReserveGCPerBlock * 2
	for i := 0; i < n; i++ {
		k.SetVerifierReserveAt(ctx, fmt.Sprintf("task-%06d", i), 1000, 1)
	}

	k.refundExpiredVerifierReserves(ctx)

	cleared := 0
	for i := 0; i < n; i++ {
		if k.GetVerifierReserve(ctx, fmt.Sprintf("task-%06d", i)) == 0 {
			cleared++
		}
	}
	require.Equal(t, VerifierReserveGCPerBlock, cleared, "单轮清理量不得超过预算")
}
