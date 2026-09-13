package keeper_test

import (
	"encoding/binary"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	keepertest "mcchain/testutil/keeper"
	"mcchain/x/phonenode/keeper"
	"mcchain/x/phonenode/types"
)

// ---------------------------------------------------------------------------
// 离线检测出队语义回归测试
//
// 有界索引扫描解决了「全量入内存」，但没有解决「出队」：
//  1. SlashIfBad 经 SetNode 回写时 LastProofBlock 不变 → 索引条目不被移除；
//  2. 扫描中遇到 attestation 失效 / 节点已删的条目只 continue 不出队。
// 两者叠加使索引头部堆积僵尸条目、反复消耗每块预算，真离线节点排队无上界增长，
// 而节点津贴照发。以下测试锁定修复后的严格 FIFO 语义与容量性质。
// ---------------------------------------------------------------------------

// offlineTestAddr 生成确定性测试地址：同样的 i 永远得到同样的地址，
// 便于构造稳定的索引顺序与可复算的批量结果。
func offlineTestAddr(i int) string {
	bz := make([]byte, 20)
	binary.BigEndian.PutUint32(bz[16:], uint32(i))
	return sdk.AccAddress(bz).String()
}

// seedNode 注册节点并写入一条有效 attestation。
// 注册会把 LastProofBlock 置为 ctx 的当前高度（入网即起算在线宽限）。
func seedNode(t *testing.T, k *keeper.Keeper, ctx sdk.Context, addr string) {
	t.Helper()
	_, err := k.RegisterNode(ctx, addr, "pixel8", "android", "contributor")
	require.NoError(t, err)
	expiry := ctx.BlockTime().Unix() + types.DefaultParams().AttestationValidity
	k.SetAttestation(ctx, addr, types.NewValidAttestation("root", "nonce", "dev-"+addr, expiry))
}

// isOfflineSlashed 判断节点是否已被离线检测判罚。
func isOfflineSlashed(t *testing.T, k *keeper.Keeper, ctx sdk.Context, addr string) bool {
	t.Helper()
	if att, ok := k.GetAttestation(ctx, addr); ok && att.Status == types.AttestationStatusValid {
		return false
	}
	for _, r := range k.GetSlashes(ctx, addr) {
		if r.Reason == "offline" {
			return true
		}
	}
	return false
}

// countOfflineSlashed 统计 [0,total) 这批测试地址中已被离线判罚的数量。
func countOfflineSlashed(t *testing.T, k *keeper.Keeper, ctx sdk.Context, total int) int {
	t.Helper()
	n := 0
	for i := 0; i < total; i++ {
		if isOfflineSlashed(t, k, ctx, offlineTestAddr(i)) {
			n++
		}
	}
	return n
}

// TestDetectOfflineSlashesStaleAndKeepsFresh 基本判据：
// 超宽限未心跳的节点必须被判离线；宽限期内有心跳的节点不得被误判。
func TestDetectOfflineSlashesStaleAndKeepsFresh(t *testing.T) {
	k, ctx := keepertest.PhonenodeKeeper(t)
	params := types.DefaultParams()

	const base = int64(1000)
	ctx = ctx.WithBlockHeight(base)

	stale := offlineTestAddr(1)
	fresh := offlineTestAddr(2)
	seedNode(t, k, ctx, stale)
	seedNode(t, k, ctx, fresh)

	now := base + params.OfflineGraceBlocks + 1
	ctx = ctx.WithBlockHeight(now)

	// fresh 在宽限期内再次心跳。
	st, err := k.GetNode(ctx, fresh)
	require.NoError(t, err)
	st.LastProofBlock = now
	require.NoError(t, k.SetNode(ctx, st))

	k.DetectOffline(ctx)

	require.True(t, isOfflineSlashed(t, k, ctx, stale),
		"超过 OfflineGraceBlocks 未心跳的节点必须被判离线")
	require.False(t, isOfflineSlashed(t, k, ctx, fresh),
		"宽限期内有心跳的节点不得被误判离线")
}

// TestDetectOfflineDrainsProcessedEntries 锁定「处理过必出队」+ 积压自愈。
//
// 构造 128（稳态预算）+ 512（积压放大预算）个同时到期的离线节点：
//
//	第一轮：无积压标记 → 预算 128 → 处理 128 条，置位积压标记；
//	第二轮：标记生效 → 预算放大到 512 → 处理剩余 512 条。
//
// 若被处理的条目不出队，第二轮会从索引头部重新扫到同一批
// 已吊销的僵尸条目并全部 continue，剩余节点永远得不到处理，本测试即失败。
func TestDetectOfflineDrainsProcessedEntries(t *testing.T) {
	k, ctx := keepertest.PhonenodeKeeper(t)
	params := types.DefaultParams()

	const base = int64(1000)
	ctx = ctx.WithBlockHeight(base)

	total := types.MaxOfflineScanPerBlock + types.MaxOfflineScanPerBlockBacklog
	for i := 0; i < total; i++ {
		seedNode(t, k, ctx, offlineTestAddr(i))
	}

	// 全体同时到期。
	ctx = ctx.WithBlockHeight(base + params.OfflineGraceBlocks + 1)

	k.DetectOffline(ctx)
	first := countOfflineSlashed(t, k, ctx, total)
	require.Equal(t, types.MaxOfflineScanPerBlock, first,
		"第一轮受稳态预算限制，应恰好处理 MaxOfflineScanPerBlock 条")

	k.DetectOffline(ctx)
	all := countOfflineSlashed(t, k, ctx, total)
	require.Equal(t, total, all,
		"被处理的条目必须离开索引（否则僵尸条目反复吃掉预算），"+
			"且积压时下一区块应放大预算完成追赶")
}

// TestDetectOfflineCapacityIndependentOfNodeCount 锁定容量性质：
// 检测成本只与「本区块真正离线」的节点数有关，与已注册设备总数解耦。
//
// 这里让 2000 台设备全部在线、3 台真正离线，检测应恰好处理那 3 台，
// 一台在线设备都不得被误判，也不因总量 2003 而漏检。
func TestDetectOfflineCapacityIndependentOfNodeCount(t *testing.T) {
	k, ctx := keepertest.PhonenodeKeeper(t)
	params := types.DefaultParams()

	const base = int64(1000)
	const online = 2000

	ctx = ctx.WithBlockHeight(base)
	for i := 0; i < online; i++ {
		seedNode(t, k, ctx, offlineTestAddr(i))
	}

	// 3 台在更早的高度心跳，对当前高度而言已经离线。
	old := base - params.OfflineGraceBlocks - 10
	ctxOld := ctx.WithBlockHeight(old)
	for i := online; i < online+3; i++ {
		seedNode(t, k, ctxOld, offlineTestAddr(i))
	}

	ctx = ctx.WithBlockHeight(base + 1)
	k.DetectOffline(ctx)

	total := online + 3
	require.Equal(t, 3, countOfflineSlashed(t, k, ctx, total),
		"只有真正离线的那 3 台应被处理，2000 台在线设备不消耗检测预算")
	for i := 0; i < online; i++ {
		require.False(t, isOfflineSlashed(t, k, ctx, offlineTestAddr(i)),
			"在线设备不得被误判离线")
	}
}

// countOfflineSlashes 统计某地址累计被「离线」判罚的次数。
// 用计数而非布尔，才能区分「首轮判罚」与「重新入队后再次判罚」。
func countOfflineSlashes(t *testing.T, k *keeper.Keeper, ctx sdk.Context, addr string) int {
	t.Helper()
	n := 0
	for _, r := range k.GetSlashes(ctx, addr) {
		if r.Reason == "offline" {
			n++
		}
	}
	return n
}

// TestDetectOfflineReattestedNodeReentersQueue 出队不等于永久移除：
// 节点重新入网（写回有效 attestation + 心跳推到当前高度）后索引位随之更新，
// 享有完整宽限；若其后再次长期离线，必须能重新被检测到。
func TestDetectOfflineReattestedNodeReentersQueue(t *testing.T) {
	k, ctx := keepertest.PhonenodeKeeper(t)
	params := types.DefaultParams()

	const base = int64(1000)
	addr := offlineTestAddr(7)

	ctx = ctx.WithBlockHeight(base)
	seedNode(t, k, ctx, addr)

	// 第一次离线：被判罚并出队。
	ctx = ctx.WithBlockHeight(base + params.OfflineGraceBlocks + 1)
	k.DetectOffline(ctx)
	require.Equal(t, 1, countOfflineSlashes(t, k, ctx, addr), "首轮应判一次离线")

	// 节点重新入网。
	reattest := ctx.BlockHeight()
	att, ok := k.GetAttestation(ctx, addr)
	require.True(t, ok)
	att.Status = types.AttestationStatusValid
	k.SetAttestation(ctx, addr, att)

	st, err := k.GetNode(ctx, addr)
	require.NoError(t, err)
	st.LastProofBlock = reattest
	st.VerifierStatus = types.VerifierStatusActive
	require.NoError(t, k.SetNode(ctx, st))

	// 重新入网即在索引中重新入队，宽限期内不得被判罚。
	ctx = ctx.WithBlockHeight(reattest + params.OfflineGraceBlocks)
	k.DetectOffline(ctx)
	require.Equal(t, 1, countOfflineSlashes(t, k, ctx, addr),
		"刚重新入网的节点应享有完整宽限，不得在宽限内被判罚")

	// 再次长期离线 → 必须能重新被检测到（出队不是永久移除）。
	ctx = ctx.WithBlockHeight(reattest + params.OfflineGraceBlocks + 1)
	k.DetectOffline(ctx)
	require.Equal(t, 2, countOfflineSlashes(t, k, ctx, addr),
		"重新入网的节点必须重新进入检测队列，出队不等于永久移除")
}
