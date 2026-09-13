package keeper

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCorruptResultEntrySelfHeals 是 自愈语义的回归闸门。
//
// 回归守则：AllResults / AllDisputes / ResultsByTask 在反序列化失败时
// 直接 panic。这三个函数都在 BeginBlock 的争议超时结案路径上，一条损坏条目会让
// **每个**节点在**每一块**都 panic —— 链停摆；而因为状态没被修改，重启后必然
// 再次 panic，属于不可自愈的永久停链。
//
// 现在改为「告警 + 删除该条目 + 继续」，链照常出块，损坏可观测、可告警。
func TestCorruptResultEntrySelfHeals(t *testing.T) {
	k, ctx, _ := setupEdgeai(t)
	store := ctx.KVStore(k.storeKey)

	// 一条正常结果 + 一条无法反序列化的损坏条目。
	r := &Result{TaskId: "task-ok", Submitter: "node-a"}
	require.NoError(t, k.SetResult(ctx, r))
	store.Set(resultKey("task-bad/node-b"), []byte{0xff, 0xff, 0xff, 0xff})

	require.NotPanics(t, func() {
		out := k.AllResults(ctx)
		require.Len(t, out, 1, "损坏条目必须被丢弃，而不是让整条链 panic")
	})

	// 损坏条目已被删除，下一块不会再重复扫到它。
	require.Nil(t, store.Get(resultKey("task-bad/node-b")), "损坏条目必须被清除，否则每块重复告警并白耗扫描预算")
}

// TestCorruptDisputeEntrySelfHeals 同 Result，覆盖 Dispute 侧。
func TestCorruptDisputeEntrySelfHeals(t *testing.T) {
	k, ctx, _ := setupEdgeai(t)
	store := ctx.KVStore(k.storeKey)

	store.Set(disputeKey("task-bad"), []byte{0xff, 0xff, 0xff, 0xff})

	require.NotPanics(t, func() {
		out := k.AllDisputes(ctx)
		require.Empty(t, out, "损坏争议记录必须被丢弃")
	})
	require.Nil(t, store.Get(disputeKey("task-bad")))
}
