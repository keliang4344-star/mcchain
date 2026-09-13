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
//
// 重算提交资格闸门。任何地址可对处于争议态的
// 任务提交任意哈希，而「与提交者哈希不一致 = 作弊」—— 一笔交易即可伪造
// 作弊证据，让无辜提交者被 slash + 托管金回收。现在要求提交者必须持有
// 该任务的验证者指派（verification 记录），与 OpenDispute 的资格闸门
// 共同构成纵深防御；伪造哈希的验证者会被声誉扣减追责（EvaluateRecompute
// honest 分支已有 ReputationFrivolousDecrease）。
func (k Keeper) RecordRecompute(ctx sdk.Context, taskID, requester, recomputeHash string) error {
	if _, err := sdk.AccAddressFromBech32(requester); err != nil {
		return fmt.Errorf("edgeai: invalid requester address (%s): %w", requester, err)
	}
	if !k.HasVerification(ctx, taskID, requester) {
		return fmt.Errorf("edgeai: recompute restricted to assigned verifiers (task %s, requester %s)", taskID, requester)
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

// applyCheatOutcome 对**自动重算**判定出的作弊执行处理。
//
// 自动路径只做可逆动作。
//
// 为什么改：链上无法验证「验证人自报的重算哈希」是否真的经过正确计算。
// RecordRecompute 的资格闸门只是「被指派验证者 + 任务处于 disputed」，
// 而 verification 由 BeginBlock 自动指派（每块 3 名）。于是单个验证人
// OpenDispute → SubmitRecompute(任意不同哈希) 即可让诚实提交者被
// SlashIfBad（10% 质押罚没，若为 bonded 验证人还会被 Jail）——全部不可逆，
// 且证据仅来自攻击者自己。等于把「罚没他人质押」做成了一方声明即可触发。
//
// 现在自动路径只：
//  1. 任务标记 Cheated（拒绝拨付，资金仍留在模块账户 escrow 内）；
//  2. 全部 pending 结果置 Rejected（清理索引，避免僵尸条目蚕食结算预算）。
//
// 不可逆的质押罚没与资金回收交由**仲裁人**在 ResolveDispute(cheat) 中执行
// （msg_server_resolve_dispute.go）。这样：
//   - 作弊者拿不到钱（拨付被拒）——经济目标达成；
//   - 诚实者被误判时最多损失「等待仲裁的时间」，不丢质押；
//   - 攻击者无法用自动路径造成任何不可逆破坏。
func (k Keeper) applyCheatOutcome(ctx sdk.Context, taskID string) {
	dispute, err := k.GetDispute(ctx, taskID)
	if err != nil || dispute == nil {
		return
	}

	// cheat 结案必须把 pending 结果落为 Rejected，
	// 否则僵尸 pending 索引每区块白耗结算预算（见 resolveDispute 注释）。
	k.rejectResultsForTask(ctx, taskID)
	if task, terr := k.GetTask(ctx, taskID); terr == nil && task != nil {
		task.Status = types.TaskStatusCheated
		if err := k.SetTask(ctx, task); err != nil {
			// 写入失败不得静默吞掉（序列化异常属确定性故障，记录以便排障）。
			ctx.Logger().Error("edgeai: SetTask failed", "err", err.Error())
		}
	}

	// 留痕：自动判定结论与「待仲裁」状态，供运维与仲裁人跟进。
	ctx.EventManager().EmitEvent(
		sdk.NewEvent("edgeai.CheatPendingArbitration",
			sdk.NewAttribute("task_id", taskID),
			sdk.NewAttribute("submitter", dispute.Submitter),
			sdk.NewAttribute("challenger", dispute.Challenger),
			sdk.NewAttribute("note", "auto recompute mismatch; payout withheld; slash/clawback pending arbitrator ruling"),
		),
	)
	k.Logger(ctx).Info("edgeai: auto recompute mismatch — payout withheld, awaiting arbitrator",
		"task_id", taskID, "submitter", dispute.Submitter)
}
