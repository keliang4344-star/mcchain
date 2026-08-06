package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"mcchain/x/edgeai/types"
)

// SelectNVerifierNodes 从合格验证者中选取 n 个（不重复）。
// 当前按洗牌后截取前 n 个，未来可与 reputation 联动优先选取高声誉节点。
//
// FORK-3：洗牌必须使用由区块数据派生的确定性随机源（见 rand.go），
// 绝不可使用 math/rand 全局源，否则各节点选出的验证者不同 → AppHash 分叉。
// 输入 addrs 来自 store 前缀迭代，本身顺序确定，洗牌后依然全网一致。
func (k Keeper) SelectNVerifierNodes(ctx sdk.Context, n int) []string {
	addrs := k.phonenodeKeeper.GetVerifierNodes(ctx)
	if len(addrs) == 0 {
		return nil
	}
	// 洗牌避免同一批节点总是被选中
	rng := newDeterministicRand(ctx, "edgeai/select-n-verifiers")
	rng.Shuffle(len(addrs), func(i, j int) { addrs[i], addrs[j] = addrs[j], addrs[i] })
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

	// SCALE-1：抽检候选取自定长「近期完成任务环」（DoneTaskRingSize 条），
	// 一次读取供本轮全部验证者复用。原实现对每个验证者都做一次 AllTaskIDs
	// 全表扫描（每区块 N×全量任务），在任务量增长后必然拖垮出块。
	// 抽检的意义在于覆盖新近完成的任务，对远期历史采样并无收益，故定长环即足够。
	recentDone := k.RecentDoneTaskIDs(ctx)
	if len(recentDone) == 0 {
		return
	}

	usedTaskIDs := make(map[string]bool)
	for _, verifierAddr := range verifiers {
		task := k.sampleTaskExcluding(ctx, recentDone, verifierAddr, usedTaskIDs)
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

// sampleTaskExcluding 从给定候选集中选取一个已完成、且未被该验证者检查过、
// 也不在 excluded 中的任务。候选集由调用方一次性提供（近期完成任务环），
// 其顺序来自 KVStore 迭代，全网确定性一致。
func (k Keeper) sampleTaskExcluding(ctx sdk.Context, taskIDs []string, verifierAddr string, excluded map[string]bool) *Task {
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
	// FORK-3：确定性随机源；domain 带上 verifierAddr，
	// 使同一区块内为不同验证者抽到的任务互相独立而又全网可复现。
	rng := newDeterministicRand(ctx, "edgeai/sample-task-excluding:"+verifierAddr)
	idx := rng.Intn(len(candidates))
	return candidates[idx]
}
