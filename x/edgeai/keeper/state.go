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
	taskKeyPrefix    = []byte("task:")
	resultKeyPrefix  = []byte("result:")
	disputeKeyPrefix = []byte("dispute:")
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
func (k Keeper) SetTask(ctx sdk.Context, t *Task) error {
	bz, err := k.cdc.Marshal(t)
	if err != nil {
		return fmt.Errorf("edgeai: marshal task: %w", err)
	}
	ctx.KVStore(k.storeKey).Set(taskKey(t.Id), bz)
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
func (k Keeper) SetResult(ctx sdk.Context, r *Result) error {
	bz, err := k.cdc.Marshal(r)
	if err != nil {
		return fmt.Errorf("edgeai: marshal result: %w", err)
	}
	ctx.KVStore(k.storeKey).Set(resultKeyFor(r.TaskId, r.Submitter), bz)
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

// GetResultByTask 返回某任务下的首个结果（争议裁定时定位提交者用）。
func (k Keeper) GetResultByTask(ctx sdk.Context, taskID string) (*Result, error) {
	results := k.AllResults(ctx)
	for _, r := range results {
		if r.TaskId == taskID {
			return r, nil
		}
	}
	return nil, nil
}
