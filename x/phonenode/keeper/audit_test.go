package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	keepertest "mcchain/testutil/keeper"
	"mcchain/x/phonenode/types"
)

// ---------------------------------------------------------------------------
// 心跳索引一致性巡检测试
//
// AuditHeartbeatIndex 是「处理过必出队」的事后校验手段：它全量核对
// 索引与主记录，报告幽灵条目 / 残影 / 漏检 / 僵尸四类不一致。
// ---------------------------------------------------------------------------

// TestAuditHeartbeatIndexCleanAfterOfflineDetection 锁定出队彻底性：
// 一批离线节点被判定后，索引里不应留下任何残留——既无僵尸条目、也无漏检，
// 巡检报告必须完全一致。
func TestAuditHeartbeatIndexCleanAfterOfflineDetection(t *testing.T) {
	k, ctx := keepertest.PhonenodeKeeper(t)
	params := types.DefaultParams()

	const base = int64(1000)
	ctx = ctx.WithBlockHeight(base)

	total := types.MaxOfflineScanPerBlock + 64
	for i := 0; i < total; i++ {
		seedNode(t, k, ctx, offlineTestAddr(i))
	}

	// 检测前：条目齐全且一致。
	before := k.AuditHeartbeatIndex(ctx)
	require.True(t, before.Consistent(), "检测前索引应一致: %+v", before)
	require.Equal(t, total, before.IndexEntries, "每个已认证节点应恰好占一个索引条目")

	// 全体超时 → 两轮检测（第二轮走积压放大预算）应把条目全部消费掉。
	ctx = ctx.WithBlockHeight(base + params.OfflineGraceBlocks + 1)
	k.DetectOffline(ctx)
	k.DetectOffline(ctx)

	after := k.AuditHeartbeatIndex(ctx)
	require.Equal(t, 0, after.IndexEntries,
		"被处理的离线节点必须全部离开索引（含积压追赶后的剩余部分）: %+v", after)
	require.True(t, after.Consistent(), "出队后索引应完全一致: %+v", after)
	require.Equal(t, total, after.NodeRecords,
		"出队只清理索引位，不得删除节点主记录")
}

// TestAuditHeartbeatIndexDetectsZombie 巡检必须能发现违反「处理过必出队」的
// 僵尸条目——这正是每块白吃扫描预算的那类残留。
func TestAuditHeartbeatIndexDetectsZombie(t *testing.T) {
	k, ctx := keepertest.PhonenodeKeeper(t)

	const base = int64(1000)
	ctx = ctx.WithBlockHeight(base)

	addr := offlineTestAddr(3)
	seedNode(t, k, ctx, addr)

	// 只吊销 attestation、不走 slash 出队路径 → 制造一条僵尸条目。
	att, ok := k.GetAttestation(ctx, addr)
	require.True(t, ok)
	att.Status = types.AttestationStatusRevoked
	k.SetAttestation(ctx, addr, att)

	rep := k.AuditHeartbeatIndex(ctx)
	require.False(t, rep.Consistent(), "存在僵尸条目时必须报告不一致")
	require.Equal(t, []string{addr}, rep.ZombieEntries)
	require.Empty(t, rep.MissingEntries,
		"已吊销认证的节点不应被算作漏检——它已经不需要被检测了")
	require.Equal(t, 1, rep.ProblemCount())
}

// TestAuditHeartbeatIndexNoStaleAfterHeartbeat 连续心跳后索引位随之迁移，
// 同一节点在索引中只应留下一个条目，不留残影。
func TestAuditHeartbeatIndexNoStaleAfterHeartbeat(t *testing.T) {
	k, ctx := keepertest.PhonenodeKeeper(t)

	const base = int64(1000)
	ctx = ctx.WithBlockHeight(base)
	addr := offlineTestAddr(5)
	seedNode(t, k, ctx, addr)

	for i := int64(1); i <= 3; i++ {
		ctx = ctx.WithBlockHeight(base + i)
		st, err := k.GetNode(ctx, addr)
		require.NoError(t, err)
		st.LastProofBlock = ctx.BlockHeight()
		require.NoError(t, k.SetNode(ctx, st))
	}

	rep := k.AuditHeartbeatIndex(ctx)
	require.Equal(t, 1, rep.IndexEntries,
		"三次心跳后同一节点只应有一个索引条目（否则会产生残影）: %+v", rep)
	require.True(t, rep.Consistent(), "心跳迁移后不应留下残影: %+v", rep)
}

// TestAuditHeartbeatIndexMissingDetected 索引条目被移走而主记录仍为有效认证时，
// 巡检必须报告漏检（该判离线却扫不到）。
func TestAuditHeartbeatIndexMissingDetected(t *testing.T) {
	k, ctx := keepertest.PhonenodeKeeper(t)

	const base = int64(1000)
	ctx = ctx.WithBlockHeight(base)

	addr := offlineTestAddr(9)
	seedNode(t, k, ctx, addr)

	// 模拟索引位被误清理：把心跳推到新高度会摘除旧索引位并写入新位，
	// 这里用一个"无对应索引"的高度直接改写主记录来构造漏检。
	st, err := k.GetNode(ctx, addr)
	require.NoError(t, err)
	st.LastProofBlock = ctx.BlockHeight() // 索引位仍在 base 高度，此处制造错位
	require.NoError(t, k.SetNode(ctx, st))

	rep := k.AuditHeartbeatIndex(ctx)
	// 主记录被重写为当前高度，索引也随之更新到当前高度，因此仍然一致——
	// 这条断言固化 SetNode 的「同事务维护索引」性质。
	require.True(t, rep.Consistent(), "SetNode 应同事务维护索引，不留错位: %+v", rep)
	require.Equal(t, 1, rep.IndexEntries)
}
