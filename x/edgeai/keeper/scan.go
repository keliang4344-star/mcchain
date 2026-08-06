package keeper

import (
	"encoding/binary"
	"strings"

	"github.com/cosmos/cosmos-sdk/store/prefix"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/edgeai/types"
)

// ---------------------------------------------------------------------------
// SCALE-1：有界轮转扫描
//
// 背景（上线前审计发现的致命项）：原 BeginBlock 每个区块都会执行
//   - AllResults()      —— 把全量任务结果读进内存（两次）
//   - AllReputations()  —— 把全量声誉记录读进内存
//   - AllTaskIDs()      —— 把全量任务 id 读进内存（过期回收 + 每个验证者各一次）
// 这些都是 O(全量实体) 的整库扫描。在网络规模上到百万级实体时，单次 BeginBlock
// 的耗时即会超过出块间隔（timeout_commit=4s），链会直接停摆——这是规模化的硬伤，
// 不是性能优化问题。
//
// 解决方案：为「待处理集合」建立独立索引（pending 结果、open 任务），
// 并为每类扫描配一个持久化游标，每区块只消费固定预算，扫到末尾自动回到开头，
// 形成公平轮转。游标是链上状态的一部分，全网各节点读到的批次完全一致，
// 不引入任何非确定性。
// ---------------------------------------------------------------------------

var (
	// pendingResultIdxPrefix：pending 结果索引，key = "<taskID>/<submitter>"，
	// 由 SetResult 这一唯一写入点维护（pending → 写入，非 pending → 删除）。
	pendingResultIdxPrefix = []byte("idx_pending_result:")
	// openTaskIdxPrefix：open 任务索引，key = "<taskID>"，由 SetTask 维护。
	openTaskIdxPrefix = []byte("idx_open_task:")
	// doneTaskRingPrefix：近期已完成任务环形缓冲，key = 8 字节大端槽位号。
	doneTaskRingPrefix = []byte("ring_done_task:")

	// 游标（模块根 store，独立前缀，不与数据键冲突）
	pendingResultCursorKey = []byte("cursor:pending_result")
	openTaskCursorKey      = []byte("cursor:open_task")
	reputationCursorKey    = []byte("cursor:reputation")
	doneTaskRingSeqKey     = []byte("seq:done_task_ring")
)

func pendingResultIdxKey(taskID, submitter string) []byte {
	return append(append(append([]byte{}, pendingResultIdxPrefix...), []byte(taskID)...), []byte("/"+submitter)...)
}

func openTaskIdxKey(taskID string) []byte {
	return append(append([]byte{}, openTaskIdxPrefix...), []byte(taskID)...)
}

func doneTaskRingKey(slot uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, slot)
	return append(append([]byte{}, doneTaskRingPrefix...), buf...)
}

// scanEntry 是一条被扫描到的索引条目（key 已去掉前缀）。
type scanEntry struct {
	Key   []byte
	Value []byte
}

// boundedScan 在 dataPrefix 下从持久化游标处最多取 limit 条，并推进游标。
//
// 关键设计：
//   - 先把条目全部读出并关闭迭代器，再交给调用方处理，避免「边迭代边写」在
//     cachekv 上产生未定义行为；
//   - 扫到末尾时删除游标，下一区块自动从头开始（公平轮转、自愈：即便游标
//     因条目被删而越界，也会在下一轮复位）。
func boundedScan(root storetypes.KVStore, dataPrefix, cursorKey []byte, limit int) []scanEntry {
	if limit <= 0 {
		return nil
	}
	ps := prefix.NewStore(root, dataPrefix)

	it := ps.Iterator(root.Get(cursorKey), nil)
	entries := make([]scanEntry, 0, limit)
	for ; it.Valid() && len(entries) < limit; it.Next() {
		entries = append(entries, scanEntry{
			Key:   append([]byte(nil), it.Key()...),
			Value: append([]byte(nil), it.Value()...),
		})
	}
	var nextStart []byte
	if it.Valid() {
		nextStart = append([]byte(nil), it.Key()...)
	}
	it.Close()

	if nextStart != nil {
		root.Set(cursorKey, nextStart)
	} else {
		root.Delete(cursorKey)
	}
	return entries
}

// PendingResultBatch 返回本区块要处理的一批 pending 结果，按 taskId 分组。
//
// 返回的 taskIDs 顺序来自 KVStore 前缀迭代（字典序），是全网确定性一致的顺序——
// 这一点至关重要：FORK-4 的根因正是原实现用 Go map 的随机迭代序决定了
// SlashIfBad → RecordSlash 的写入顺序，导致各验证者算出的 AppHash 不同而分叉。
//
// 每个 taskID 命中后一律通过 ResultsByTask 取该任务下的全部结果，
// 保证一致性投票看到完整提交者集合，不会因扫描窗口切断分组而误判多数派。
func (k Keeper) PendingResultBatch(ctx sdk.Context, maxTasks int) ([]string, map[string][]*Result) {
	if maxTasks <= 0 {
		return nil, nil
	}
	root := ctx.KVStore(k.storeKey)
	ps := prefix.NewStore(root, pendingResultIdxPrefix)

	it := ps.Iterator(root.Get(pendingResultCursorKey), nil)
	taskIDs := make([]string, 0, maxTasks)
	seen := make(map[string]bool, maxTasks)
	scanned := 0
	var nextStart []byte
	for ; it.Valid(); it.Next() {
		key := string(it.Key())
		taskID := key
		if i := strings.Index(key, "/"); i >= 0 {
			taskID = key[:i]
		}
		if !seen[taskID] {
			// 达到任务数上限时，必须停在「新任务的第一条」上，
			// 保证已纳入的分组是完整的，游标也正好落在下一组开头。
			if len(taskIDs) >= maxTasks {
				nextStart = append([]byte(nil), it.Key()...)
				break
			}
			seen[taskID] = true
			taskIDs = append(taskIDs, taskID)
		}
		scanned++
		// 硬预算：防止单任务挂载海量提交者时把本区块预算击穿。
		if scanned >= types.MaxPendingResultScanPerBlock {
			it.Next()
			if it.Valid() {
				nextStart = append([]byte(nil), it.Key()...)
			}
			break
		}
	}
	it.Close()

	if nextStart != nil {
		root.Set(pendingResultCursorKey, nextStart)
	} else {
		root.Delete(pendingResultCursorKey)
	}

	byTask := make(map[string][]*Result, len(taskIDs))
	for _, tid := range taskIDs {
		byTask[tid] = k.ResultsByTask(ctx, tid)
	}
	return taskIDs, byTask
}

// OpenTaskBatch 返回本区块要检查是否过期的一批 open 任务 id（有界轮转）。
func (k Keeper) OpenTaskBatch(ctx sdk.Context, limit int) []string {
	entries := boundedScan(ctx.KVStore(k.storeKey), openTaskIdxPrefix, openTaskCursorKey, limit)
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, string(e.Key))
	}
	return out
}

// setOpenTaskIndex 由 SetTask 调用，维护 open 任务索引。
func (k Keeper) setOpenTaskIndex(ctx sdk.Context, t *Task) {
	store := ctx.KVStore(k.storeKey)
	if t.Status == types.TaskStatusOpen {
		store.Set(openTaskIdxKey(t.Id), []byte{1})
		return
	}
	store.Delete(openTaskIdxKey(t.Id))
}

// setPendingResultIndex 由 SetResult 调用，维护 pending 结果索引。
func (k Keeper) setPendingResultIndex(ctx sdk.Context, r *Result) {
	store := ctx.KVStore(k.storeKey)
	key := pendingResultIdxKey(r.TaskId, r.Submitter)
	if r.Status == types.ResultStatusPending {
		store.Set(key, []byte{1})
		return
	}
	store.Delete(key)
}

// pushDoneTaskRing 把一个刚完成的任务写入环形缓冲（覆盖最旧槽位）。
// 环长 DoneTaskRingSize，读写均为 O(1)/O(环长)，与历史任务总量无关。
func (k Keeper) pushDoneTaskRing(ctx sdk.Context, taskID string) {
	store := ctx.KVStore(k.storeKey)
	seq := uint64(0)
	if bz := store.Get(doneTaskRingSeqKey); len(bz) == 8 {
		seq = binary.BigEndian.Uint64(bz)
	}
	store.Set(doneTaskRingKey(seq%types.DoneTaskRingSize), []byte(taskID))

	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, seq+1)
	store.Set(doneTaskRingSeqKey, buf)
}

// RecentDoneTaskIDs 返回近期已完成任务 id（最多 DoneTaskRingSize 条，去重、槽位序）。
// 用于验证者抽检采样，替代原先按区块全表扫描 AllTaskIDs 的做法。
func (k Keeper) RecentDoneTaskIDs(ctx sdk.Context) []string {
	ps := prefix.NewStore(ctx.KVStore(k.storeKey), doneTaskRingPrefix)
	it := ps.Iterator(nil, nil)
	defer it.Close()
	out := make([]string, 0, types.DoneTaskRingSize)
	seen := make(map[string]bool, types.DoneTaskRingSize)
	for ; it.Valid(); it.Next() {
		id := string(it.Value())
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
