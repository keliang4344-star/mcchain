package keeper

import (
	"math/rand"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"mcchain/x/edgeai/types"
)

// SelectNVerifierNodes 从合格验证者中选取 n 个（不重复）。
// 当前按洗牌后截取前 n 个，未来可与 reputation 联动优先选取高声誉节点。
func (k Keeper) SelectNVerifierNodes(ctx sdk.Context, n int) []string {
	addrs := k.phonenodeKeeper.GetVerifierNodes(ctx)
	if len(addrs) == 0 {
		return nil
	}
	// 洗牌避免同一批节点总是被选中
	rand.Shuffle(len(addrs), func(i, j int) { addrs[i], addrs[j] = addrs[j], addrs[i] })
	if len(addrs) < n {
		return addrs
	}
	return addrs[:n]
}

// verifyResultHashMatch 实现 REAL 的验证判定（取代原 sha256%101 伪随机打分）。
//
// 链上无法重跑 AI 执行沙箱，因此"验证"被定义为：比较验证者离线重跑后提交的
// 结果哈希（verifierHash）与提交者原始提交的结果哈希（submitterHash）。
//   - verifierHash == ""              → 验证者尚未提交结果，无法判定；
//   - verifierHash == submitterHash   → 一致 → 验证通过（pass / verified）；
//   - verifierHash != submitterHash   → 不一致 → 判定为作弊（cheat）。
func verifyResultHashMatch(submitterHash, verifierHash string) (verified, cheat bool) {
	if verifierHash == "" {
		return false, false
	}
	if verifierHash == submitterHash {
		return true, false
	}
	return false, true
}

// ScoreAndVerify 是 BeginBlock Phase 3 的验证者抽检指派。
//
// 由于链上无法重跑沙箱，本函数只负责"指派"抽检任务（写入 Verification 记录），
// 真正的验证裁定由验证者离线重跑后通过
//   SubmitVerification(taskID, verifierAddr, verifierResultHash)
// 提交结果哈希，经 verifyResultHashMatch 与提交者结果哈希比对得出
// verified / cheat 结论，并据此发放验证者奖励或创建作弊争议。
//
// 移除旧的"中位数阈值"伪评分逻辑：原实现用 sha256(taskID:verifier)%101 生成
// 确定性伪分数并与阈值比较，并非真实验证，现已被上述哈希比对取代。
func (k Keeper) ScoreAndVerify(ctx sdk.Context) {
	verifierCount := int(types.DefaultVerifierCount)

	verifiers := k.SelectNVerifierNodes(ctx, verifierCount)
	if len(verifiers) == 0 {
		return
	}

	usedTaskIDs := make(map[string]bool)
	for _, verifierAddr := range verifiers {
		task := k.sampleTaskExcluding(ctx, verifierAddr, usedTaskIDs)
		if task == nil {
			continue
		}
		usedTaskIDs[task.Id] = true

		if _, err := k.AssignVerification(ctx, task.Id, verifierAddr); err != nil {
			k.Logger(ctx).Error("edgeai: assign scoring verification failed",
				"task_id", task.Id, "verifier", verifierAddr, "err", err.Error())
			continue
		}

		ctx.EventManager().EmitEvent(
			sdk.NewEvent("edgeai.VerificationAssigned",
				sdk.NewAttribute("task_id", task.Id),
				sdk.NewAttribute("verifier", verifierAddr),
			),
		)
	}
}

// sampleTaskExcluding 选取一个已完成且未被 excluded 中的验证者检查过的任务。
func (k Keeper) sampleTaskExcluding(ctx sdk.Context, verifierAddr string, excluded map[string]bool) *Task {
	taskIDs := k.AllTaskIDs(ctx)
	candidates := make([]*Task, 0, len(taskIDs))
	for _, tid := range taskIDs {
		if excluded[tid] {
			continue
		}
		task, err := k.GetTask(ctx, tid)
		if err != nil || task == nil {
			continue
		}
		if task.Status != types.TaskStatusDone {
			continue
		}
		if k.HasVerification(ctx, tid, verifierAddr) {
			continue
		}
		candidates = append(candidates, task)
	}
	if len(candidates) == 0 {
		return nil
	}
	idx := rand.Intn(len(candidates))
	return candidates[idx]
}
