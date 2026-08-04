package keeper

import (
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"mcchain/x/edgeai/types"
)

// ---------------------------------------------------------------------------
// 第二验证层：链上重算（2026-08 落地）
//
// 争议开启后，第二验证方（挑战者 / 独立验证者）可对同一任务提交「链上重算结果指纹」。
// 与原始提交结果指纹比对（白皮书 §13 第二验证层）：
//   - 不一致 → 强作弊证据：对原提交者执行 slash + 声誉扣减 + 回收奖励，任务标记为作弊；
//   - 一致   → 质疑不成立（可能误告）：对挑战方声誉轻度扣减，任务维持争议待仲裁。
//
// 该层逻辑真实可运行并已单元测试。外部触发消息（MsgSubmitRecompute）待 proto
// 代码生成工具链就绪后接入；当前由仲裁路径 ResolveDispute 在结算时统一评估。
// ---------------------------------------------------------------------------

var RecomputeKeyPrefix = []byte("Recompute:")

// Recompute 记录一次链上重算（第二验证层）。
type Recompute struct {
	TaskID        string `json:"task_id"`
	Requester     string `json:"requester"`
	RecomputeHash string `json:"recompute_hash"` // hex
	BlockHeight   int64  `json:"block_height"`
	Evaluated     bool   `json:"evaluated"`
	Outcome       string `json:"outcome"` // "" / "cheat" / "honest"
}

func recomputeKey(taskID string) []byte { return append(RecomputeKeyPrefix, []byte(taskID)...) }

// RecordRecompute 在任务处于争议态时记录一次链上重算结果（第二验证层）。
func (k Keeper) RecordRecompute(ctx sdk.Context, taskID, requester, recomputeHash string) error {
	if _, err := sdk.AccAddressFromBech32(requester); err != nil {
		return fmt.Errorf("edgeai: invalid requester address (%s): %w", requester, err)
	}
	task, err := k.GetTask(ctx, taskID)
	if err != nil || task == nil {
		return types.ErrTaskNotFound.Wrap(taskID)
	}
	if task.Status != types.TaskStatusDisputed {
		return fmt.Errorf("edgeai: recompute requires an open dispute (task %s not disputed)", taskID)
	}
	rc := Recompute{TaskID: taskID, Requester: requester, RecomputeHash: recomputeHash, BlockHeight: ctx.BlockHeight()}
	bz, _ := json.Marshal(rc)
	ctx.KVStore(k.storeKey).Set(recomputeKey(taskID), bz)
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"edgeai.RecomputeRecorded",
		sdk.NewAttribute("task_id", taskID),
		sdk.NewAttribute("requester", requester),
	))
	return nil
}

// GetRecompute 读取任务的链上重算记录；不存在返回 (nil, false)。
func (k Keeper) GetRecompute(ctx sdk.Context, taskID string) (*Recompute, bool) {
	bz := ctx.KVStore(k.storeKey).Get(recomputeKey(taskID))
	if bz == nil {
		return nil, false
	}
	var rc Recompute
	if err := json.Unmarshal(bz, &rc); err != nil {
		return nil, false
	}
	return &rc, true
}

// EvaluateRecompute 比对链上重算结果与原始提交结果：
//   - 不一致 → 作弊确认：slash 提交者 + 声誉扣减 + 回收奖励 + 任务标记作弊；
//   - 一致   → 质疑不成立：挑战方声誉轻度扣减。
//
// 返回 cheatConfirmed（重算是否与原始结果冲突）。幂等：已评估则直接返回上次结论。
func (k Keeper) EvaluateRecompute(ctx sdk.Context, taskID string) (bool, error) {
	rc, ok := k.GetRecompute(ctx, taskID)
	if !ok {
		return false, fmt.Errorf("edgeai: no recompute recorded for task %s", taskID)
	}
	if rc.Evaluated {
		return rc.Outcome == "cheat", nil
	}

	result, err := k.GetResultByTask(ctx, taskID)
	if err != nil || result == nil {
		return false, fmt.Errorf("edgeai: no original result for task %s", taskID)
	}

	cheat := result.ResultHash != rc.RecomputeHash
	if cheat {
		k.applyCheatOutcome(ctx, taskID)
		rc.Outcome = "cheat"
	} else {
		// 质疑不成立：对挑战方轻度声誉扣减（误告惩戒）
		k.DecrementReputation(ctx, rc.Requester, types.ReputationFrivolousDecrease)
		rc.Outcome = "honest"
	}
	rc.Evaluated = true
	bz, _ := json.Marshal(rc)
	ctx.KVStore(k.storeKey).Set(recomputeKey(taskID), bz)
	return cheat, nil
}

// applyCheatOutcome 对裁定/确认作弊的任务执行完整后果：
// slash 提交者 + 声誉扣减 + 回收奖励 + 任务标记作弊。
func (k Keeper) applyCheatOutcome(ctx sdk.Context, taskID string) {
	dispute, err := k.GetDispute(ctx, taskID)
	if err != nil || dispute == nil {
		return
	}
	if dispute.Submitter != "" {
		if serr := k.phonenodeKeeper.SlashIfBad(ctx, dispute.Submitter, "cheat_result", types.CheatSlashBps); serr != nil {
			ctx.Logger().Error("edgeai: recompute cheat slash failed", "task_id", taskID, "err", serr.Error())
		}
		k.DecrementReputation(ctx, dispute.Submitter, types.ReputationCheatDecrease)
	}
	k.clawbackSubmitterReward(ctx, taskID)
	if task, terr := k.GetTask(ctx, taskID); terr == nil && task != nil {
		task.Status = types.TaskStatusCheated
		_ = k.SetTask(ctx, task)
	}
}
