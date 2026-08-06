package keeper

import (
	"fmt"
	"strconv"

	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/edgeai/types"
)

// Task / Result / Dispute 是 protobuf 生成类型（x/edgeai/types）的别名，
// 统一与全链一致的状态二进制编码（A1 改进：原 JSON 自管理改为 protobuf）。
type Task = types.Task
type Result = types.Result
type Dispute = types.Dispute

// task count key
var taskCountKey = []byte("task_count")

// Store prefixes
var (
	taskKeyPrefix          = []byte("task:")
	resultKeyPrefix        = []byte("result:")
	disputeKeyPrefix       = []byte("dispute:")
	verifierReservePrefix  = []byte("verifier_reserve:")
)

func taskKey(id string) []byte   { return append(taskKeyPrefix, []byte(id)...) }
func resultKey(k string) []byte  { return append(resultKeyPrefix, []byte(k)...) }
func disputeKey(k string) []byte { return append(disputeKeyPrefix, []byte(k)...) }

func (k Keeper) nextTaskID(ctx sdk.Context) string {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(taskCountKey)
	count := uint64(0)
	if bz != nil {
		count, _ = strconv.ParseUint(string(bz), 10, 64)
	}
	count++
	store.Set(taskCountKey, []byte(strconv.FormatUint(count, 10)))
	return strconv.FormatUint(count, 10)
}

// SetTask 持久化任务（protobuf 编码，与全链一致）。
//
// SCALE-1：这里同时维护两个派生索引，使 BeginBlock 不再需要整库扫描——
//   - open 任务索引：过期回收只遍历真正处于 open 的任务；
//   - 近期完成任务环：验证者抽检只在定长环上采样，与历史任务总量解耦。
//
// 本函数是任务状态的唯一写入点，索引与主记录同事务写入，不会漂移。
func (k Keeper) SetTask(ctx sdk.Context, t *Task) error {
	bz, err := k.cdc.Marshal(t)
	if err != nil {
		return fmt.Errorf("edgeai: marshal task: %w", err)
	}

	// 仅在「首次进入 done」时入环，避免同一任务重复占用环位。
	if t.Status == types.TaskStatusDone {
		prev, prevErr := k.GetTask(ctx, t.Id)
		if prevErr == nil && (prev == nil || prev.Status != types.TaskStatusDone) {
			k.pushDoneTaskRing(ctx, t.Id)
		}
	}

	ctx.KVStore(k.storeKey).Set(taskKey(t.Id), bz)
	k.setOpenTaskIndex(ctx, t)
	return nil
}

// GetTask 读取任务；不存在返回 nil。
func (k Keeper) GetTask(ctx sdk.Context, id string) (*Task, error) {
	bz := ctx.KVStore(k.storeKey).Get(taskKey(id))
	if bz == nil {
		return nil, nil
	}
	var t Task
	if err := k.cdc.Unmarshal(bz, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// AllTaskIDs 返回全部 task id。
func (k Keeper) AllTaskIDs(ctx sdk.Context) []string {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), taskKeyPrefix)
	it := store.Iterator(nil, nil)
	defer it.Close()
	ids := make([]string, 0)
	for ; it.Valid(); it.Next() {
		ids = append(ids, string(it.Key()))
	}
	return ids
}

// AllDisputes 返回全部争议记录（protobuf 编码，前缀迭代）。
func (k Keeper) AllDisputes(ctx sdk.Context) []*Dispute {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), disputeKeyPrefix)
	it := store.Iterator(nil, nil)
	defer it.Close()
	out := make([]*Dispute, 0)
	for ; it.Valid(); it.Next() {
		var d Dispute
		if err := k.cdc.Unmarshal(it.Value(), &d); err != nil {
			panic(fmt.Sprintf("edgeai: corrupt dispute entry at key %q: %v", string(it.Key()), err))
		}
		out = append(out, &d)
	}
	return out
}

// resultKeyFor(taskID, submitter)
func resultKeyFor(taskID, submitter string) []byte {
	return resultKey(taskID + "/" + submitter)
}

// SetResult 持久化结果（protobuf 编码）。
//
// SCALE-1：同步维护 pending 结果索引。BeginBlock 的结算与一致性投票只遍历
// 该索引（待办集合），不再对全量历史结果做整库扫描。
// 本函数是结果状态的唯一写入点，索引不会与主记录漂移。
func (k Keeper) SetResult(ctx sdk.Context, r *Result) error {
	bz, err := k.cdc.Marshal(r)
	if err != nil {
		return fmt.Errorf("edgeai: marshal result: %w", err)
	}
	ctx.KVStore(k.storeKey).Set(resultKeyFor(r.TaskId, r.Submitter), bz)
	k.setPendingResultIndex(ctx, r)
	return nil
}

// GetResult 按 (taskID, submitter) 读取结果。
func (k Keeper) GetResult(ctx sdk.Context, taskID, submitter string) (*Result, error) {
	bz := ctx.KVStore(k.storeKey).Get(resultKeyFor(taskID, submitter))
	if bz == nil {
		return nil, nil
	}
	var r Result
	if err := k.cdc.Unmarshal(bz, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// HasResult 是否已提交结果。
func (k Keeper) HasResult(ctx sdk.Context, taskID, submitter string) bool {
	return ctx.KVStore(k.storeKey).Has(resultKeyFor(taskID, submitter))
}

// SetDispute 持久化争议（protobuf 编码）。
func (k Keeper) SetDispute(ctx sdk.Context, d *Dispute) error {
	bz, err := k.cdc.Marshal(d)
	if err != nil {
		return fmt.Errorf("edgeai: marshal dispute: %w", err)
	}
	ctx.KVStore(k.storeKey).Set(disputeKey(d.TaskId), bz)
	return nil
}

// GetDispute 按 taskID 读取争议。
func (k Keeper) GetDispute(ctx sdk.Context, taskID string) (*Dispute, error) {
	bz := ctx.KVStore(k.storeKey).Get(disputeKey(taskID))
	if bz == nil {
		return nil, nil
	}
	var d Dispute
	if err := k.cdc.Unmarshal(bz, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// ResultsByTask 返回某任务下的全部结果，按提交者字典序（前缀迭代，确定性）。
//
// SCALE-3：结果键为 "result:<taskID>/<submitter>"，同一任务的记录在 KVStore 中
// 本就是连续的，因此这里做一次前缀迭代即可，代价与该任务的提交者数成正比，
// 与全链历史结果总量无关。原实现走 AllResults 全表扫描，是规模化下的性能陷阱。
//
// 分隔符 "/" 保证前缀不会串组：taskID "1" 的前缀为 "1/"，不会命中 "10/..."。
func (k Keeper) ResultsByTask(ctx sdk.Context, taskID string) []*Result {
	p := append(append([]byte{}, resultKeyPrefix...), []byte(taskID+"/")...)
	store := prefix.NewStore(ctx.KVStore(k.storeKey), p)
	it := store.Iterator(nil, nil)
	defer it.Close()
	out := make([]*Result, 0)
	for ; it.Valid(); it.Next() {
		var r Result
		if err := k.cdc.Unmarshal(it.Value(), &r); err != nil {
			// 关键审计路径：结果反序列化失败属状态损坏，fail-fast 而非静默吞掉数据损坏。
			panic(fmt.Sprintf("edgeai: corrupt result entry for task %q: %v", taskID, err))
		}
		rr := r
		out = append(out, &rr)
	}
	return out
}

// GetResultByTask 返回某任务下的首个结果（争议裁定时定位提交者用）。
func (k Keeper) GetResultByTask(ctx sdk.Context, taskID string) (*Result, error) {
	results := k.ResultsByTask(ctx, taskID)
	if len(results) == 0 {
		return nil, nil
	}
	return results[0], nil
}

// ---- Verifier Reserve (80/15/5 split) ----

func verifierReserveKey(taskID string) []byte {
	return append(verifierReservePrefix, []byte(taskID)...)
}

// SetVerifierReserve stores the 15% verifier reward reserve for a specific task
// during settlement. The reserve is claimed by the verifier node upon successful
// verification sampling.
func (k Keeper) SetVerifierReserve(ctx sdk.Context, taskID string, amount uint64) {
	ctx.KVStore(k.storeKey).Set(verifierReserveKey(taskID),
		[]byte(strconv.FormatUint(amount, 10)))
}

// GetVerifierReserve returns the reserved verifier reward for a task.
// Returns 0 if no reserve exists.
func (k Keeper) GetVerifierReserve(ctx sdk.Context, taskID string) uint64 {
	bz := ctx.KVStore(k.storeKey).Get(verifierReserveKey(taskID))
	if bz == nil {
		return 0
	}
	v, err := strconv.ParseUint(string(bz), 10, 64)
	if err != nil {
		return 0
	}
	return v
}

// DeleteVerifierReserve clears the verifier reserve for a task after it has been claimed.
func (k Keeper) DeleteVerifierReserve(ctx sdk.Context, taskID string) {
	ctx.KVStore(k.storeKey).Delete(verifierReserveKey(taskID))
}
