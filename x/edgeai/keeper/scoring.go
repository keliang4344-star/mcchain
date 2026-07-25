package keeper

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/rand"
	"sort"

	"github.com/cosmos/cosmos-sdk/telemetry"
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

// verifyResult 模拟验证者对任务结果打分（0-100 分）。
//
// 在生产环境中，此函数将调用链下 AI 执行沙箱重新运行任务数据，对比
// ResultHash 一致性后生成结构化评分。当前链上实现使用确定性伪评分
// （基于 taskID + verifierAddr 的 SHA-256 哈希），确保同对 (task, verifier)
// 每次返回一致分数，避免共识层不确定性。
func verifyResult(taskID, verifierAddr string) uint32 {
	h := sha256.Sum256([]byte(taskID + ":" + verifierAddr))
	// 取前 4 字节转 uint32，映射到 0-100
	score := binary.BigEndian.Uint32(h[:4]) % 101
	return score
}

// medianScore 计算分数切片的中位数（去掉最高最低后取中间值）。
// 若 len(scores) <= 2，直接取平均值；否则去掉最高和最低后取中位数。
func medianScore(scores []uint32) uint32 {
	if len(scores) == 0 {
		return 0
	}
	sorted := make([]uint32, len(scores))
	copy(sorted, scores)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	if len(sorted) <= 2 {
		// 样本太少，无法去极值，直接取平均
		sum := uint32(0)
		for _, s := range sorted {
			sum += s
		}
		return sum / uint32(len(sorted))
	}
	// 去掉最低和最高分
	trimmed := sorted[1 : len(sorted)-1]
	mid := len(trimmed) / 2
	if len(trimmed)%2 == 1 {
		return trimmed[mid]
	}
	return (trimmed[mid-1] + trimmed[mid]) / 2
}

// ScoreAndVerify 多验证者投票评分系统（白皮书行 496-497）。
//
// 在 BeginBlock Phase 3 中被调用，替换原有的单验证者 auto-pass 逻辑：
//  1. 从合格节点中随机抽取 N 个验证者（DefaultVerifierCount）；
//  2. 每个验证者对抽中的已完成任务调用 verifyResult 获得 score (0-100)；
//  3. 去掉最高和最低分后取中位数；
//  4. 中位数 ≥ ThresholdScore → 通过，各验证者获得奖励；
//  5. 中位数 < ThresholdScore → 拒绝，任务进入争议状态。
func (k Keeper) ScoreAndVerify(ctx sdk.Context) {
	rand.Seed(ctx.BlockTime().UnixNano() + ctx.BlockHeight())

	verifierCount := int(types.DefaultVerifierCount)
	thresholdScore := types.DefaultThresholdScore

	verifiers := k.SelectNVerifierNodes(ctx, verifierCount)
	if len(verifiers) == 0 {
		return
	}

	// 每个验证者选取一个不同的已完成任务
	usedTaskIDs := make(map[string]bool)
	type scoreEntry struct {
		verifier string
		taskID   string
		score    uint32
	}
	var entries []scoreEntry

	for _, verifierAddr := range verifiers {
		task := k.sampleTaskExcluding(ctx, verifierAddr, usedTaskIDs)
		if task == nil {
			continue
		}
		usedTaskIDs[task.Id] = true

		score := verifyResult(task.Id, verifierAddr)
		entries = append(entries, scoreEntry{
			verifier: verifierAddr,
			taskID:   task.Id,
			score:    score,
		})

		// 持久化验证记录
		if _, err := k.AssignVerification(ctx, task.Id, verifierAddr); err != nil {
			k.Logger(ctx).Error("edgeai: assign scoring verification failed",
				"task_id", task.Id, "verifier", verifierAddr, "err", err.Error())
		}
	}

	if len(entries) == 0 {
		return
	}

	// 收集所有评分计算中位数
	scores := make([]uint32, len(entries))
	for i, e := range entries {
		scores[i] = e.score
	}
	median := medianScore(scores)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("edgeai.ScoringRound",
			sdk.NewAttribute("verifier_count", fmt.Sprintf("%d", len(verifiers))),
			sdk.NewAttribute("scored_count", fmt.Sprintf("%d", len(entries))),
			sdk.NewAttribute("median_score", fmt.Sprintf("%d", median)),
			sdk.NewAttribute("threshold", fmt.Sprintf("%d", thresholdScore)),
			sdk.NewAttribute("passed", fmt.Sprintf("%t", median >= thresholdScore)),
		),
	)

	if median >= thresholdScore {
		// 通过 → 所有验证者获得奖励
		for _, e := range entries {
			k.submitScoreReward(ctx, e.taskID, e.verifier, e.score, false)
		}
		telemetry.IncrCounter(1, "edgeai", "scoring_pass_count")
	} else {
		// 拒绝 → 创建争议，标注作弊风险
		for _, e := range entries {
			k.createScoringDispute(ctx, e.taskID, e.verifier, e.score, median, thresholdScore)
		}
		telemetry.IncrCounter(1, "edgeai", "scoring_reject_count")
		telemetry.IncrCounter(float32(len(entries)), "edgeai", "scoring_disputed_tasks")
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

// submitScoreReward 为单个验证者发放评分奖励并更新声誉。
func (k Keeper) submitScoreReward(ctx sdk.Context, taskID, verifierAddr string, score uint32, isDispute bool) {
	// 从 verifier reserve 领取奖励（15% 预留池或兜底固定奖励）
	reserve := k.GetVerifierReserve(ctx, taskID)
	var reward uint64
	if reserve > 0 {
		reward = reserve
		k.DeleteVerifierReserve(ctx, taskID)
	} else {
		reward = types.VerifierRewardPerSample
	}

	if reward > 0 {
		addr, addrErr := sdk.AccAddressFromBech32(verifierAddr)
		if addrErr == nil {
			if sendErr := k.bankKeeper.SendCoinsFromModuleToAccount(
				ctx, types.ModuleName, addr,
				sdk.NewCoins(sdk.NewInt64Coin(types.EdgeAIDenom, int64(reward))),
			); sendErr != nil {
				k.Logger(ctx).Error("edgeai: scoring reward failed",
					"task_id", taskID, "verifier", verifierAddr, "err", sendErr.Error())
			}
		}
	}

	// 更新验证记录
	v, _ := k.GetVerification(ctx, taskID, verifierAddr)
	if v != nil && !v.Rewarded {
		v.IsHonest = true
		v.Proof = fmt.Sprintf("scoring: score=%d", score)
		v.Rewarded = true
		_ = k.SetVerification(ctx, v)
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("edgeai.ScoringRewarded",
			sdk.NewAttribute("task_id", taskID),
			sdk.NewAttribute("verifier", verifierAddr),
			sdk.NewAttribute("score", fmt.Sprintf("%d", score)),
			sdk.NewAttribute("reward", fmt.Sprintf("%d", reward)),
		),
	)

	// 声誉更新：评分通过 → +1
	k.IncrementReputation(ctx, verifierAddr, types.ReputationPassIncrease)

	telemetry.IncrCounter(1, "edgeai", "scoring_reward_count")
	telemetry.IncrCounter(float32(reward), "edgeai", "scoring_reward_amount")
}

// createScoringDispute 当评分不足时自动创建争议。
func (k Keeper) createScoringDispute(ctx sdk.Context, taskID, verifierAddr string,
	score, median, threshold uint32,
) {
	// 避免重复创建争议
	existing, _ := k.GetDispute(ctx, taskID)
	if existing != nil {
		return
	}

	task, err := k.GetTask(ctx, taskID)
	if err != nil || task == nil {
		return
	}

	result, _ := k.GetResultByTask(ctx, taskID)
	submitter := ""
	if result != nil {
		submitter = result.Submitter
	}

	d := &Dispute{
		TaskId:     taskID,
		Challenger: verifierAddr,
		Submitter:  submitter,
		Reason: fmt.Sprintf(
			"multi-verifier scoring: median=%d < threshold=%d (scored_by=%s score=%d)",
			median, threshold, verifierAddr, score,
		),
		Status:        "open",
		Resolution:    "none",
		OpenedAt:      ctx.BlockTime().Unix(),
		OpenedAtBlock: ctx.BlockHeight(),
	}
	if err := k.SetDispute(ctx, d); err != nil {
		return
	}

	task.Status = types.TaskStatusDisputed
	_ = k.SetTask(ctx, task)

	// 声誉更新：触发争议 → 提交者 -10
	if submitter != "" {
		k.DecrementReputation(ctx, submitter, types.ReputationCheatDecrease)
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("edgeai.ScoringDisputed",
			sdk.NewAttribute("task_id", taskID),
			sdk.NewAttribute("verifier", verifierAddr),
			sdk.NewAttribute("median_score", fmt.Sprintf("%d", median)),
			sdk.NewAttribute("threshold", fmt.Sprintf("%d", threshold)),
			sdk.NewAttribute("submitter", submitter),
		),
	)
}
