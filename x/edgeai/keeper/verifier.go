package keeper

import (
	"encoding/json"
	"fmt"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/edgeai/types"
)

// verification key prefix for KV store
var verificationKeyPrefix = []byte("verification:")

func verificationKey(taskID, verifier string) []byte {
	return append(verificationKeyPrefix, []byte(taskID+"/"+verifier)...)
}

// SetVerification persists a verification record (JSON encoded).
func (k Keeper) SetVerification(ctx sdk.Context, v *types.Verification) error {
	bz, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("edgeai: marshal verification: %w", err)
	}
	ctx.KVStore(k.storeKey).Set(verificationKey(v.TaskId, v.Verifier), bz)
	return nil
}

// GetVerification retrieves a verification record by (taskID, verifier).
func (k Keeper) GetVerification(ctx sdk.Context, taskID, verifier string) (*types.Verification, error) {
	bz := ctx.KVStore(k.storeKey).Get(verificationKey(taskID, verifier))
	if bz == nil {
		return nil, nil
	}
	var v types.Verification
	if err := json.Unmarshal(bz, &v); err != nil {
		return nil, fmt.Errorf("edgeai: unmarshal verification: %w", err)
	}
	return &v, nil
}

// HasVerification checks whether a verification record exists for the given
// (taskID, verifier) pair.
func (k Keeper) HasVerification(ctx sdk.Context, taskID, verifier string) bool {
	return ctx.KVStore(k.storeKey).Has(verificationKey(taskID, verifier))
}

// AllVerifications returns all stored verification records.
func (k Keeper) AllVerifications(ctx sdk.Context) []*types.Verification {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), verificationKeyPrefix)
	it := store.Iterator(nil, nil)
	defer it.Close()
	out := make([]*types.Verification, 0)
	for ; it.Valid(); it.Next() {
		var v types.Verification
		if err := json.Unmarshal(it.Value(), &v); err != nil {
			continue
		}
		out = append(out, &v)
	}
	return out
}

// SelectVerifierNode picks a verifier from the eligible set returned by the
// phononode keeper, weighted by on-chain reputation.  Returns empty string if
// no eligible verifier exists.
//
// WP-2（白皮书↔代码一致性缺口）：白皮书载明「验证者遴选按声誉加权」
// （Verifier selection is reputation-weighted）。旧实现是等概率均匀抽样，
// 声誉分只用于扣分与接单限流，从不影响被抽中的概率——满分节点与刚被扣到
// 濒临清零的节点机会完全相同，白皮书的这条承诺在链上并不成立。
//
// 现按声誉加权抽样：候选 i 的权重 = 其声誉分（0-100）。
//   - 声誉 100 的节点被选中的概率是声誉 25 的节点的 4 倍；
//   - 声誉被扣至 0（反复作弊）的节点权重为 0，事实上退出抽检队列，
//     不再获得对他人评分的权力；
//   - 若全部候选声誉均为 0（极端情形），退化为等概率抽样，
//     保证抽检机制本身不会因此停摆。
//
// 确定性（FORK-3 约束）：候选顺序来自 staking 的 power 索引（确定），
// 随机源来自区块数据派生的 newDeterministicRand，权重累加为整数运算、
// 无浮点，全网各节点计算结果逐位一致。
func (k Keeper) SelectVerifierNode(ctx sdk.Context) string {
	addrs := k.phonenodeKeeper.GetVerifierNodes(ctx)
	if len(addrs) == 0 {
		return ""
	}
	// FORK-3：确定性随机源（见 rand.go），禁止 math/rand 全局源。
	rng := newDeterministicRand(ctx, "edgeai/select-verifier")

	weights := make([]uint64, len(addrs))
	var total uint64
	for i, addr := range addrs {
		rep, err := k.GetReputation(ctx, addr)
		if err != nil || rep == nil {
			// 读取失败按 0 权重处理，不影响其余候选，也不中断抽检。
			continue
		}
		weights[i] = uint64(rep.Score)
		total += weights[i]
	}

	// 全员声誉为 0：退化为等概率抽样，抽检不停摆。
	if total == 0 {
		return addrs[rng.Intn(len(addrs))]
	}

	// 加权抽样：在 [0, total) 上取点，落入哪段区间就选哪个候选。
	// total 上界 = MaxValidators × MaxReputationScore，远小于 int63 范围。
	pick := uint64(rng.Int63n(int64(total)))
	for i, w := range weights {
		if pick < w {
			return addrs[i]
		}
		pick -= w
	}
	// 理论不可达（整数精确累加，pick < total 必然命中某段）。
	return addrs[len(addrs)-1]
}

// SampleTask selects a random done-status task that has NOT yet been verified
// by the given verifier.  Returns nil if no eligible task is found.
//
// SCALE-1：候选集取自定长「近期完成任务环」（DoneTaskRingSize 条），
// 不再做 AllTaskIDs 全表扫描——抽检的价值在于覆盖新近完成的任务，
// 对远期历史采样并无收益，而全表扫描会随任务累积无上限地拖慢调用方。
func (k Keeper) SampleTask(ctx sdk.Context, verifierAddr string) *Task {
	return k.sampleTaskExcluding(ctx, k.RecentDoneTaskIDs(ctx), verifierAddr, nil)
}

// AssignVerification creates a new verification assignment record with
// status "assigned".
func (k Keeper) AssignVerification(ctx sdk.Context, taskID, verifierAddr string) (*types.Verification, error) {
	v := &types.Verification{
		TaskId:    taskID,
		Verifier:  verifierAddr,
		IsHonest:  false,
		Proof:     "",
		Rewarded:  false,
		CreatedAt: ctx.BlockTime().Unix(),
	}
	if err := k.SetVerification(ctx, v); err != nil {
		return nil, err
	}
	return v, nil
}

// SubmitVerification processes a verification submission from a verifier node.
//
// The verifier is expected to have re-run the task off-chain and submits the
// ResultHash (verifierResultHash) it observed. Verification is REAL: we compare
// the verifier's submitted hash against the original submitter's ResultHash via
// verifyResultHashMatch:
//   - match  → task verified: mark honest, pay the verifier from the 15% reserve.
//   - differ → task flagged as cheat: auto-create a Dispute for arbitrator ruling
//              (the arbitrator's cheat verdict later triggers clawbackSubmitterReward).
func (k Keeper) SubmitVerification(ctx sdk.Context, taskID, verifierAddr, verifierResultHash string) error {
	// Update the verification record
	v, err := k.GetVerification(ctx, taskID, verifierAddr)
	if err != nil || v == nil {
		return fmt.Errorf("edgeai: verification not found for task %s verifier %s", taskID, verifierAddr)
	}

	// 取提交者原始结果哈希
	result, _ := k.GetResultByTask(ctx, taskID)
	var submitterHash string
	if result != nil {
		submitterHash = result.ResultHash
	}

	_, cheat := verifyResultHashMatch(submitterHash, verifierResultHash)
	v.VerifierResultHash = verifierResultHash
	if err := k.SetVerification(ctx, v); err != nil {
		// ERR-1：写入失败不得静默吞掉（序列化异常属确定性故障，记录以便排障）。
		ctx.Logger().Error("edgeai: SetVerification failed", "err", err.Error())
	}

	if cheat {
		// 结果不一致 → 作弊嫌疑，自动创建争议，由仲裁者裁定（将触发 clawback）。
		existing, _ := k.GetDispute(ctx, taskID)
		if existing == nil {
			task, terr := k.GetTask(ctx, taskID)
			submitter := ""
			if result != nil {
				submitter = result.Submitter
			}
			if terr == nil && task != nil {
				d := &Dispute{
					TaskId:        taskID,
					Challenger:    verifierAddr,
					Submitter:     submitter,
					Reason:        fmt.Sprintf("verifier result hash %s != submitter hash %s", truncateHash(verifierResultHash), truncateHash(submitterHash)),
					Status:        "open",
					Resolution:    "none",
					OpenedAt:      ctx.BlockTime().Unix(),
					OpenedAtBlock: ctx.BlockHeight(),
				}
				if derr := k.SetDispute(ctx, d); derr == nil {
					task.Status = types.TaskStatusDisputed
					if err := k.SetTask(ctx, task); err != nil {
						// ERR-1：写入失败不得静默吞掉（序列化异常属确定性故障，记录以便排障）。
						ctx.Logger().Error("edgeai: SetTask failed", "err", err.Error())
					}
					// 声誉更新：作弊嫌疑 → -10（白皮书行 497）
					if submitter != "" {
						k.DecrementReputation(ctx, submitter, types.ReputationCheatDecrease)
					}
				}
			}
		}
		ctx.EventManager().EmitEvent(
			sdk.NewEvent("edgeai.VerifierCheatFlagged",
				sdk.NewAttribute("task_id", taskID),
				sdk.NewAttribute("verifier", verifierAddr),
				sdk.NewAttribute("verifier_hash", truncateHash(verifierResultHash)),
				sdk.NewAttribute("submitter_hash", truncateHash(submitterHash)),
			),
		)
		telemetry.IncrCounter(1, "edgeai", "verifier_dispute_count")
		return nil
	}

	// 一致 → 验证通过：标记诚实并领取 15% 验证者预留池奖励。
	v.IsHonest = true
	v.Proof = fmt.Sprintf("verify: submitter=%s verifier=%s match", truncateHash(submitterHash), truncateHash(verifierResultHash))
	if err := k.SetVerification(ctx, v); err != nil {
		// ERR-1：写入失败不得静默吞掉（序列化异常属确定性故障，记录以便排障）。
		ctx.Logger().Error("edgeai: SetVerification failed", "err", err.Error())
	}

	if v.Rewarded {
		return nil // already rewarded
	}

	// 从该任务的验证者奖励预留池领取 15%
	reserve := k.GetVerifierReserve(ctx, taskID)
	var reward uint64
	if reserve > 0 {
		reward = reserve
		k.DeleteVerifierReserve(ctx, taskID)
	} else {
		// 兜底：若预留池已空（如历史任务在 80/15/5 分账前结算），回退到固定奖励
		reward = types.VerifierRewardPerSample
	}

	addr, addrErr := sdk.AccAddressFromBech32(verifierAddr)
	if addrErr != nil {
		return fmt.Errorf("edgeai: invalid verifier address %s: %w", verifierAddr, addrErr)
	}
	// OVF-1：reserve 可能来自任务结算的 15% 预留池（由 task.Reward 推导），
	// int64() 回绕会让验证者奖励翻负。全程用 uint64 → Int。
	if sendErr := k.bankKeeper.SendCoinsFromModuleToAccount(
		ctx, types.ModuleName, addr,
		sdk.NewCoins(sdk.NewCoin(types.EdgeAIDenom, sdkmath.NewIntFromUint64(reward))),
	); sendErr != nil {
		k.Logger(ctx).Error("edgeai: verifier reward failed",
			"task_id", taskID, "verifier", verifierAddr, "err", sendErr.Error())
		return sendErr
	}

	v.Rewarded = true
	if err := k.SetVerification(ctx, v); err != nil {
		// ERR-1：写入失败不得静默吞掉（序列化异常属确定性故障，记录以便排障）。
		ctx.Logger().Error("edgeai: SetVerification failed", "err", err.Error())
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("edgeai.VerifierRewarded",
			sdk.NewAttribute("task_id", taskID),
			sdk.NewAttribute("verifier", verifierAddr),
			sdk.NewAttribute("reward", fmt.Sprintf("%d", reward)),
		),
	)
	telemetry.IncrCounter(1, "edgeai", "verifier_reward_count")
	telemetry.IncrCounter(float32(reward), "edgeai", "verifier_reward_amount")
	return nil
}

// SampleAndVerify is called during BeginBlock (Phase 3) to randomly sample
// one settled (done) task and assign it to a verifier for inspection.
//
// Workflow:
//  1. Select a random eligible verifier node (staked, attested, online).
//  2. Select a random done task not yet verified by this verifier.
//  3. Assign a verification record and immediately "submit" the result.
//     In this simplified on-chain path we auto-pass as honest (true AI
//     re-execution requires off-chain infrastructure).
//
// Sampling is capped at MaxVerificationsPerBlock per block.
func (k Keeper) SampleAndVerify(ctx sdk.Context) {
	// FORK-3：此处原先调用 rand.Seed(...) 播种 math/rand 全局源，
	// 该做法自 Go 1.20 起已废弃且在共识路径下不安全（全局源为进程共享，
	// 播种与取值之间会被其它 goroutine 打断，各节点序列不一致 → 分叉）。
	// 现已改为在各取值点由区块数据派生本地 *rand.Rand（见 rand.go），
	// 无需也不允许在此播种任何全局状态。
	verifierAddr := k.SelectVerifierNode(ctx)
	if verifierAddr == "" {
		return
	}

	task := k.SampleTask(ctx, verifierAddr)
	if task == nil {
		return
	}

	if _, err := k.AssignVerification(ctx, task.Id, verifierAddr); err != nil {
		k.Logger(ctx).Error("edgeai: assign verification failed",
			"task_id", task.Id, "verifier", verifierAddr, "err", err.Error())
		return
	}

	// 注意：链上无法重跑沙箱，因此只做"指派"，不自动判定。
	// 真正的验证裁定由验证者离线重跑后通过 SubmitVerification 提交结果哈希，
	// 经哈希比对得出 verified / cheat 结论（见 SubmitVerification）。
}
