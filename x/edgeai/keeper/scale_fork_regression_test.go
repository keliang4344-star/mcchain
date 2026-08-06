package keeper

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"mcchain/x/edgeai/types"
)

// ---------------------------------------------------------------------------
// 上线前审计致命项回归测试
//
// 本文件锁定四项已修复的致命缺陷，防止日后被静默改回：
//   FORK-4  共识作弊判定的 slash 写入顺序必须确定（原实现依赖 Go map 随机迭代序 → 分叉）
//   REP-1   声誉衰减每周期至多 -1（原实现跨过阈值后每区块都扣 → 百余区块清零）
//   SCALE-1 每区块扫描必须有界且能公平轮转（原实现整库扫描 → 规模化后停块）
//   SCALE-3 按任务取结果必须限定在该任务前缀内（原实现走全量结果扫描）
// ---------------------------------------------------------------------------

// TestFORK4CheatSlashOrderIsDeterministic 锁定 FORK-4：
// 同样的链上状态，反复执行一致性作弊判定，slash 的写入顺序必须字字相同。
//
// 构造：3 个任务，每个任务 5 份 pending 结果 —— 3 份同哈希构成 60% 多数派，
// 另 2 份各持不同哈希构成少数派。少数派分属两个不同的哈希分组，
// 原实现遍历 hashGroups 这张 Go map 来驱动 SlashIfBad，两名少数派谁先被罚
// 每次运行都可能不同；各验证者写入的 slash 记录顺序不一致 → AppHash 分叉。
func TestFORK4CheatSlashOrderIsDeterministic(t *testing.T) {
	// 任务前缀 t1<t2<t3，提交者前缀 a<b<c，均为字典序，便于断言期望顺序。
	taskPrefixes := []string{"t1", "t2", "t3"}
	submitterPrefixes := []string{"a", "b", "c"}

	// 期望顺序：任务按字典序，任务内按「哈希首次出现顺序」→ 少数派 4 号先于 5 号。
	want := []string{"a4", "a5", "b4", "b5", "c4", "c5"}

	const runs = 20
	for run := 0; run < runs; run++ {
		k, ctx, ph := setupEdgeaiFull(t, nil)

		for ti, tp := range taskPrefixes {
			sp := submitterPrefixes[ti]
			quickCreateTask(t, k, ctx, tp, "creator", 1000, types.TaskStatusDone, 1)
			// 多数派：3 份相同哈希（3/5 = 60% > 50% 阈值）
			for i := 1; i <= 3; i++ {
				quickCreateResult(t, k, ctx, tp, fmt.Sprintf("%s%d", sp, i),
					"hashMAJORITY", types.ResultStatusPending, 1)
			}
			// 少数派：两份互不相同的哈希，落入不同 hash 分组
			quickCreateResult(t, k, ctx, tp, sp+"4", "hashDEVIANT_B", types.ResultStatusPending, 1)
			quickCreateResult(t, k, ctx, tp, sp+"5", "hashDEVIANT_C", types.ResultStatusPending, 1)
		}

		taskIDs, byTask := k.PendingResultBatch(ctx, int(types.MaxTasksPerBlock))
		require.Equal(t, taskPrefixes, taskIDs, "任务顺序必须来自 KVStore 字典序")

		k.detectCheatByConsensus(ctx, taskIDs, byTask)

		require.Equal(t, want, ph.slashed,
			"第 %d 次执行的 slash 顺序与期望不符：作弊判定不得依赖 Go map 迭代序", run)
	}
}

// TestFORK4MajorityTieIsDeterministic 锁定多数派并列时的裁决确定性：
// 两个哈希各占一半时不得触发自动 slash（未超过 50% 阈值），
// 且无论运行多少次结果都一致。
func TestFORK4MajorityTieIsDeterministic(t *testing.T) {
	for run := 0; run < 10; run++ {
		k, ctx, ph := setupEdgeaiFull(t, nil)
		quickCreateTask(t, k, ctx, "tie", "creator", 1000, types.TaskStatusDone, 1)
		quickCreateResult(t, k, ctx, "tie", "s1", "hashA", types.ResultStatusPending, 1)
		quickCreateResult(t, k, ctx, "tie", "s2", "hashB", types.ResultStatusPending, 1)

		taskIDs, byTask := k.PendingResultBatch(ctx, int(types.MaxTasksPerBlock))
		k.detectCheatByConsensus(ctx, taskIDs, byTask)

		require.Empty(t, ph.slashed, "50% 并列未超过阈值，不得自动罚没")
	}
}

// TestREP1DecayAtMostOncePerPeriod 锁定 REP-1：
// 越过无贡献阈值后，每个衰减周期至多扣 1 分，而不是此后每个区块都扣。
func TestREP1DecayAtMostOncePerPeriod(t *testing.T) {
	k, ctx, _ := setupEdgeaiFull(t, nil)

	const node = "node-decay"
	require.NoError(t, k.SetReputation(ctx, &Reputation{
		NodeAddr:              node,
		Score:                 types.DefaultReputationScore,
		LastContributionBlock: 1,
	}))

	// 尚未满一个衰减周期：不扣分
	ctx = ctx.WithBlockHeight(1 + types.ReputationDecayBlocks - 1)
	k.DecayReputation(ctx)
	r, err := k.GetReputation(ctx, node)
	require.NoError(t, err)
	require.Equal(t, types.DefaultReputationScore, r.Score, "未满一个衰减周期不得扣分")

	// 刚好满一个周期：扣 1 分
	ctx = ctx.WithBlockHeight(1 + types.ReputationDecayBlocks)
	k.DecayReputation(ctx)
	r, err = k.GetReputation(ctx, node)
	require.NoError(t, err)
	require.Equal(t, types.DefaultReputationScore-1, r.Score, "满一个周期应扣 1 分")
	require.Equal(t, ctx.BlockHeight(), r.LastDecayBlock, "必须记录本次衰减高度")

	// 紧接着的 50 个区块内不得再扣（这正是 REP-1 的核心）：
	// 原实现在此会每个区块扣 1 分，约百余区块把满分节点清零。
	for i := int64(1); i <= 50; i++ {
		ctx = ctx.WithBlockHeight(1 + types.ReputationDecayBlocks + i)
		k.DecayReputation(ctx)
	}
	r, err = k.GetReputation(ctx, node)
	require.NoError(t, err)
	require.Equal(t, types.DefaultReputationScore-1, r.Score,
		"一个衰减周期内至多扣 1 分，不得逐块扣减")

	// 再过一个完整周期：再扣 1 分
	ctx = ctx.WithBlockHeight(1 + 2*types.ReputationDecayBlocks)
	k.DecayReputation(ctx)
	r, err = k.GetReputation(ctx, node)
	require.NoError(t, err)
	require.Equal(t, types.DefaultReputationScore-2, r.Score, "第二个周期应再扣 1 分")
}

// TestREP1NoDecayForNodeWithoutContribution 从未贡献过的新节点不参与衰减。
func TestREP1NoDecayForNodeWithoutContribution(t *testing.T) {
	k, ctx, _ := setupEdgeaiFull(t, nil)
	require.NoError(t, k.SetReputation(ctx, &Reputation{
		NodeAddr:              "fresh",
		Score:                 types.DefaultReputationScore,
		LastContributionBlock: 0,
	}))
	ctx = ctx.WithBlockHeight(10 * types.ReputationDecayBlocks)
	k.DecayReputation(ctx)
	r, err := k.GetReputation(ctx, "fresh")
	require.NoError(t, err)
	require.Equal(t, types.DefaultReputationScore, r.Score)
}

// TestSCALE1BoundedScanRotates 锁定 SCALE-1 的轮转语义：
// 每次至多取 limit 条；连续调用可无重复地覆盖全集；走到末尾自动回到开头。
func TestSCALE1BoundedScanRotates(t *testing.T) {
	k, ctx, _ := setupEdgeaiFull(t, nil)
	root := ctx.KVStore(k.storeKey)

	testPrefix := []byte("scan_test:")
	testCursor := []byte("cursor:scan_test")
	const total = 10
	for i := 0; i < total; i++ {
		root.Set(append(append([]byte{}, testPrefix...), []byte(fmt.Sprintf("%02d", i))...), []byte{1})
	}

	const limit = 4
	seen := make([]string, 0, total)
	for round := 0; round < 3; round++ { // 4 + 4 + 2 == 10，第三轮走到末尾
		entries := boundedScan(root, testPrefix, testCursor, limit)
		require.LessOrEqual(t, len(entries), limit, "单次扫描不得超出预算")
		for _, e := range entries {
			seen = append(seen, string(e.Key))
		}
	}
	require.Len(t, seen, total, "一轮轮转应恰好覆盖全集且不重复")
	for i := 0; i < total; i++ {
		require.Equal(t, fmt.Sprintf("%02d", i), seen[i], "轮转顺序应为 KVStore 字典序（确定性）")
	}

	// 末尾之后游标复位，下一次从头开始
	require.Nil(t, root.Get(testCursor), "扫到末尾必须清除游标以便下一轮从头开始")
	again := boundedScan(root, testPrefix, testCursor, limit)
	require.Len(t, again, limit)
	require.Equal(t, "00", string(again[0].Key), "新一轮应回到集合开头")
}

// TestSCALE1PendingBatchKeepsGroupsComplete 锁定分组完整性：
// 达到任务数上限时必须停在「下一个任务的第一条」上，
// 已纳入的任务必须携带其全部提交者，否则一致性投票会误判多数派。
func TestSCALE1PendingBatchKeepsGroupsComplete(t *testing.T) {
	k, ctx, _ := setupEdgeaiFull(t, nil)

	for _, tid := range []string{"task1", "task2", "task3"} {
		quickCreateTask(t, k, ctx, tid, "creator", 1000, types.TaskStatusDone, 1)
		for i := 1; i <= 3; i++ {
			quickCreateResult(t, k, ctx, tid, fmt.Sprintf("sub%d", i),
				"hash", types.ResultStatusPending, 1)
		}
	}

	// 限 2 个任务：必须拿到 task1/task2 且各 3 条结果齐全
	taskIDs, byTask := k.PendingResultBatch(ctx, 2)
	require.Equal(t, []string{"task1", "task2"}, taskIDs)
	require.Len(t, byTask["task1"], 3, "已纳入的任务必须携带完整提交者集合")
	require.Len(t, byTask["task2"], 3)

	// 下一批从 task3 继续（游标推进），随后回到开头
	taskIDs2, byTask2 := k.PendingResultBatch(ctx, 2)
	require.Equal(t, []string{"task3"}, taskIDs2)
	require.Len(t, byTask2["task3"], 3)

	taskIDs3, _ := k.PendingResultBatch(ctx, 2)
	require.Equal(t, []string{"task1", "task2"}, taskIDs3, "走完一轮后应回到开头")
}

// TestSCALE1PendingIndexDropsSettledResults 结果一旦离开 pending，
// 必须同步从索引摘除，否则轮转会被历史结果永久占满。
func TestSCALE1PendingIndexDropsSettledResults(t *testing.T) {
	k, ctx, _ := setupEdgeaiFull(t, nil)
	quickCreateTask(t, k, ctx, "task1", "creator", 1000, types.TaskStatusDone, 1)
	quickCreateResult(t, k, ctx, "task1", "sub1", "hash", types.ResultStatusPending, 1)

	taskIDs, _ := k.PendingResultBatch(ctx, 10)
	require.Equal(t, []string{"task1"}, taskIDs)

	// 结算后重写结果 → 索引应被删除
	quickCreateResult(t, k, ctx, "task1", "sub1", "hash", types.ResultStatusValid, 1)
	taskIDs2, _ := k.PendingResultBatch(ctx, 10)
	require.Empty(t, taskIDs2, "已结算结果不得继续占用 pending 轮转预算")
}

// TestSCALE1OpenTaskIndexTracksStatus open 任务索引随状态变化增删。
func TestSCALE1OpenTaskIndexTracksStatus(t *testing.T) {
	k, ctx, _ := setupEdgeaiFull(t, nil)
	quickCreateTask(t, k, ctx, "open1", "creator", 1000, types.TaskStatusOpen, 1)
	quickCreateTask(t, k, ctx, "done1", "creator", 1000, types.TaskStatusDone, 1)

	require.Equal(t, []string{"open1"}, k.OpenTaskBatch(ctx, 10),
		"仅 open 任务进入过期回收轮转")

	// open1 完成后应退出索引，并进入「近期完成任务环」
	quickCreateTask(t, k, ctx, "open1", "creator", 1000, types.TaskStatusDone, 1)
	require.Empty(t, k.OpenTaskBatch(ctx, 10))
	require.Contains(t, k.RecentDoneTaskIDs(ctx), "open1",
		"完成的任务应进入定长抽检候选环")
}

// TestSCALE1DoneRingIsBounded 近期完成任务环长度恒定，不随历史任务累积增长。
func TestSCALE1DoneRingIsBounded(t *testing.T) {
	k, ctx, _ := setupEdgeaiFull(t, nil)
	n := int(types.DoneTaskRingSize) + 50
	for i := 0; i < n; i++ {
		quickCreateTask(t, k, ctx, fmt.Sprintf("task%04d", i), "creator", 1000, types.TaskStatusDone, 1)
	}
	recent := k.RecentDoneTaskIDs(ctx)
	require.LessOrEqual(t, len(recent), int(types.DoneTaskRingSize),
		"抽检候选环必须定长，不得随任务总量增长")
	require.NotEmpty(t, recent)
}

// TestSCALE3ResultsByTaskIsScoped 锁定 SCALE-3：
// 按任务取结果只在该任务前缀内迭代，不得触发全量结果扫描。
func TestSCALE3ResultsByTaskIsScoped(t *testing.T) {
	k, ctx, _ := setupEdgeaiFull(t, nil)
	quickCreateResult(t, k, ctx, "taskA", "s1", "hA1", types.ResultStatusPending, 1)
	quickCreateResult(t, k, ctx, "taskA", "s2", "hA2", types.ResultStatusPending, 1)
	quickCreateResult(t, k, ctx, "taskB", "s1", "hB1", types.ResultStatusPending, 1)

	a := k.ResultsByTask(ctx, "taskA")
	require.Len(t, a, 2)
	for _, r := range a {
		require.Equal(t, "taskA", r.TaskId)
	}
	require.Len(t, k.ResultsByTask(ctx, "taskB"), 1)
	require.Empty(t, k.ResultsByTask(ctx, "taskC"))

	// 前缀不得越界：taskA 的查询不能吃到 taskAA 的结果
	quickCreateResult(t, k, ctx, "taskAA", "s1", "hAA", types.ResultStatusPending, 1)
	require.Len(t, k.ResultsByTask(ctx, "taskA"), 2,
		"任务前缀查询必须以分隔符收口，不得误吞前缀相同的其它任务")
}
