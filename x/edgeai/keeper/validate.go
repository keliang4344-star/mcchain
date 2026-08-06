package keeper

import (
	"fmt"
	"strconv"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"mcchain/x/edgeai/types"
)

// detectCheatByConsensus 实现 B3.1 链上作弊自动检测：对同一任务的多节点 pending 结果做 ResultHash 一致性投票。
//
// 机制：
//   - 按 taskId 分组所有 pending 结果；
//   - 每组按 ResultHash 二次分组，统计各 hash 的提交人数；
//   - 多数派（占比最高）占比 > AntiCheatThresholdBps 时，少数派自动标记 cheat + slash；
//   - 已有争议的任务跳过（由仲裁者裁定）；
//   - 单结果任务跳过（无法做一致性判断，走原有乐观结算路径）。
//
// AntiCheatThresholdBps 默认 5000（50%），即多数派必须超过半数才触发自动判定。
// 例：3 节点提交，2 个相同 hash（67%）→ 1 个少数派自动 slash；
// 2 节点各不同 hash（各 50%）→ 不触发，留待争议窗口/仲裁者。
//
// FORK-4（上线前审计发现的致命项）：本函数会调用 SlashIfBad → RecordSlash，
// 向链上 slash 记录列表按顺序追加条目，属于会写入共识状态的操作。
// 原实现直接 range 两个 Go map（pendingByTask / hashGroups）来驱动这些写入，
// 而 Go 的 map 迭代顺序是每进程随机化的——不同验证者会以不同顺序写入 slash 记录，
// 算出的 AppHash 随之不同，最终导致链分叉/停块。
//
// 现在的实现全程不依赖 map 迭代序：
//   - 任务顺序由 KVStore 前缀迭代（字典序）给出，全网一致；
//   - 同一任务内的结果顺序由结果键（<taskID>/<submitter>）字典序给出，全网一致；
//   - hash 分组按「首次出现顺序」这一确定性序列遍历，多数派并列时同样由该序列裁决。
func (k Keeper) detectCheatByConsensus(ctx sdk.Context, taskIDs []string, byTask map[string][]*Result) {
	params := k.GetParams(ctx)
	threshold := params.AntiCheatThresholdBps
	if threshold == 0 {
		return // 阈值=0 表示禁用自动检测
	}

	for _, taskID := range taskIDs {
		// 只对 pending 结果做一致性投票（已结算/已拒绝的不再参与）。
		resList := make([]*Result, 0, len(byTask[taskID]))
		for _, r := range byTask[taskID] {
			if r.Status == types.ResultStatusPending {
				resList = append(resList, r)
			}
		}
		if len(resList) < 2 {
			continue // 单结果无法一致性检测
		}

		// 有争议的任务跳过自动检测，由仲裁者裁定
		dispute, _ := k.GetDispute(ctx, taskID)
		if dispute != nil {
			continue
		}

		// 按 ResultHash 分组；hashOrder 记录首次出现顺序，作为确定性遍历序列。
		hashGroups := make(map[string][]*Result)
		hashOrder := make([]string, 0, len(resList))
		for _, r := range resList {
			if _, ok := hashGroups[r.ResultHash]; !ok {
				hashOrder = append(hashOrder, r.ResultHash)
			}
			hashGroups[r.ResultHash] = append(hashGroups[r.ResultHash], r)
		}

		total := uint32(len(resList))

		// 找多数派（占比最高的 hash）。严格大于才替换，
		// 因此并列时取「首次出现」的那个 hash —— 确定性裁决，不依赖 map 序。
		var majorityHash string
		var majorityCount uint32
		for _, h := range hashOrder {
			c := uint32(len(hashGroups[h]))
			if c > majorityCount {
				majorityCount = c
				majorityHash = h
			}
		}

		// 多数派未超过阈值 → 无法判定，跳过
		if majorityCount*10000/total <= threshold {
			continue
		}

		// 标记少数派为 cheat：slash + rejected
		for _, h := range hashOrder {
			if h == majorityHash {
				continue
			}
			for _, r := range hashGroups[h] {
				reason := fmt.Sprintf(
					"anti-cheat consensus: hash %s deviates from majority %s (%d/%d submitters)",
					truncateHash(r.ResultHash), truncateHash(majorityHash), majorityCount, total,
				)
				if err := k.phonenodeKeeper.SlashIfBad(ctx, r.Submitter, reason, types.CheatSlashBps); err != nil {
					k.Logger(ctx).Error("edgeai: consensus auto-slash failed",
						"task_id", r.TaskId, "submitter", r.Submitter, "err", err.Error())
					continue
				}
				// 声誉更新：作弊 → -10（白皮书行 497）
				k.DecrementReputation(ctx, r.Submitter, types.ReputationCheatDecrease)
				r.Status = types.ResultStatusRejected
				if err := k.SetResult(ctx, r); err != nil {
					// ERR-1：写入失败不得静默吞掉（序列化异常属确定性故障，记录以便排障）。
					ctx.Logger().Error("edgeai: SetResult failed", "err", err.Error())
				}

				ctx.EventManager().EmitEvent(
					sdk.NewEvent("edgeai.CheatDetected",
						sdk.NewAttribute("task_id", r.TaskId),
						sdk.NewAttribute("submitter", r.Submitter),
						sdk.NewAttribute("result_hash", truncateHash(r.ResultHash)),
						sdk.NewAttribute("majority_hash", truncateHash(majorityHash)),
						sdk.NewAttribute("reason", "consensus_deviation"),
					),
				)
				telemetry.IncrCounter(1, "edgeai", "cheat_detected_count")
			}
		}
	}
}

// truncateHash 截断 hash 用于日志/事件（前 12 字符）。
func truncateHash(h string) string {
	if len(h) <= 12 {
		return h
	}
	return h[:12] + "..."
}

// AllResults 返回全部已提交结果（protobuf 持久化，前缀迭代）。BeginBlock 用于扫描待结算结果。
func (k Keeper) AllResults(ctx sdk.Context) []*Result {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), resultKeyPrefix)
	it := store.Iterator(nil, nil)
	defer it.Close()
	out := make([]*Result, 0)
	for ; it.Valid(); it.Next() {
		var r Result
		if err := k.cdc.Unmarshal(it.Value(), &r); err != nil {
			// 关键审计路径：结果反序列化失败属状态损坏，fail-fast 而非静默吞掉数据损坏。
			panic(fmt.Sprintf("edgeai: corrupt result entry at key %q: %v", string(it.Key()), err))
		}
		out = append(out, &r)
	}
	return out
}

// DeterminePayout 计算一笔通过验证的结果应发奖励：取任务 reward，封顶 MaxTaskReward。
// 纯函数，便于单测（不触碰 bank / 外部 keeper）。
func DeterminePayout(task *Task, params types.Params) uint64 {
	reward := task.Reward
	cap, err := strconv.ParseUint(params.MaxTaskReward, 10, 64)
	if err != nil || cap == 0 {
		return reward
	}
	if reward > cap {
		return cap
	}
	return reward
}

// BeginBlock 实现 B3.1 R4「贡献即挖矿」经济闭环的结算（需求方付费 / escrow 模型）：
//   - 扫描所有 pending 结果；
//   - 无未决争议且已超过 DisputePeriodBlocks → 乐观判定有效，从任务托管金拨付 submitter；
//   - 存在 open 争议且争议窗口已过 → 因暂无链上作弊证明机制，乐观判定诚实（honest），
//     结案并拨付（争议仲裁者机制留待后续引入，见 audit.md 已知不足）。
//   - 超时未结算的 open 任务（TaskExpireBlocks）→ 标记 expired，退还托管金给创建者。
//
// 拨付经 bankKeeper 从 edgeai 模块账户（creator 创建任务时托管的 reward）出币给 submitter，
// "谁出币"= edgeai 模块账户（来自需求方托管），不直接 mint，受 B1 总量 cap 约束。
// 模块账户余额异常（如托管金不足）等错误仅记录事件、不阻塞出块。
//
// 每区块结算上限由 MaxTasksPerBlock 控制，防止 BeginBlock 过重阻塞出块。
// SCALE-1：本函数不再做任何整库扫描。待结算结果取自 pending 索引的有界轮转批次，
// 过期回收取自 open 任务索引的有界轮转批次，声誉衰减同样按固定预算轮转。
// 每区块工作量恒定，与链上历史实体总量无关。
func (k Keeper) BeginBlock(ctx sdk.Context) {
	// 本区块的待办批次：至多 MaxTasksPerBlock 个任务，分组完整、顺序确定。
	taskIDs, byTask := k.PendingResultBatch(ctx, int(types.MaxTasksPerBlock))

	// Phase 0: 多节点结果一致性投票（AntiCheatThresholdBps 自动作弊检测）
	k.detectCheatByConsensus(ctx, taskIDs, byTask)

	// Phase 0.5: 节点声誉衰减（白皮书行 497）
	// 连续 ReputationDecayBlocks 区块无贡献 → 声誉 -1
	k.DecayReputation(ctx)

	params := k.GetParams(ctx)
	settledCount := uint64(0)
	settledTaskIDs := make(map[string]bool) // 追踪已结算的任务，避免同一任务重复计数

	// 展平为确定性有序结果列表：任务按索引字典序，任务内按提交者字典序。
	results := make([]*Result, 0, len(taskIDs))
	for _, tid := range taskIDs {
		results = append(results, byTask[tid]...)
	}

	for _, r := range results {
		if settledCount >= types.MaxTasksPerBlock {
			break
		}
		if r.Status != types.ResultStatusPending {
			continue
		}
		task, err := k.GetTask(ctx, r.TaskId)
		if err != nil || task == nil {
			continue
		}
		// 任务已结算（首个有效结果已发币）→ 跳过后续结果，避免同一任务重复拨付。
		if task.Status == types.TaskStatusDone {
			continue
		}

		// 争议/窗口结算判定（B3.1）：
		//   - 争议已裁定 cheat → 跳过拨付（提交者已在裁定时 slash）
		//   - 争议已裁定 honest / 无争议且结果窗口已过 → 拨付
		//   - 争议仍 open 且窗口未过 → 跳过（等待裁决/窗口）
		dispute, _ := k.GetDispute(ctx, r.TaskId)
		if dispute != nil {
			switch dispute.Status {
			case "resolved":
				if dispute.Resolution == "cheat" {
					continue
				}
				// honest resolved → 进入拨付
			case "open":
				if dispute.OpenedAtBlock > 0 && (ctx.BlockHeight()-dispute.OpenedAtBlock) >= params.DisputePeriodBlocks {
					k.resolveDispute(ctx, dispute, "honest")
				} else {
					continue
				}
			default:
				continue
			}
		} else {
			if !(r.SubmittedAtBlock > 0 && (ctx.BlockHeight()-r.SubmittedAtBlock) >= params.DisputePeriodBlocks) {
				continue
			}
		}

		amount := DeterminePayout(task, params)

		// ---- 85/15 reward split ----
		// 85% → submitter (executor node)
		// 15% → verifier reserve (claimed on verification sampling)
		// 原 5% 结算销毁已撤销（白皮书《优化定稿版》§24.6 否决清单），份额并入提交者：
		// 需求方托管的任务付费 100% 流向真实完成工作的节点与核验者，链上零截留。
		submitterAmount := amount * uint64(types.EdgeAISubmitterRatioBps) / 10000
		verifierReserveAmount := amount - submitterAmount // catch rounding

		addr, err := sdk.AccAddressFromBech32(r.Submitter)
		if err != nil {
			continue
		}
		// 拨付 submitter 80%
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(
			ctx, types.ModuleName, addr,
			sdk.NewCoins(sdk.NewCoin(types.EdgeAIDenom, sdkmath.NewIntFromUint64(submitterAmount))),
		); err != nil {
			k.Logger(ctx).Error("edgeai: payout failed", "task_id", r.TaskId, "submitter", r.Submitter, "err", err.Error())
			ctx.EventManager().EmitEvent(
				sdk.NewEvent("edgeai.PayoutFailed",
					sdk.NewAttribute("task_id", r.TaskId),
					sdk.NewAttribute("submitter", r.Submitter),
					sdk.NewAttribute("amount", strconv.FormatUint(submitterAmount, 10)),
					sdk.NewAttribute("error", err.Error()),
				),
			)
			continue
		}

		// 15% 存入验证者奖励预留池（验证者抽检后领取）
		if verifierReserveAmount > 0 {
			k.SetVerifierReserve(ctx, r.TaskId, verifierReserveAmount)
			ctx.EventManager().EmitEvent(
				sdk.NewEvent("edgeai.VerifierReserved",
					sdk.NewAttribute("task_id", r.TaskId),
					sdk.NewAttribute("amount", strconv.FormatUint(verifierReserveAmount, 10)),
					sdk.NewAttribute("ratio", "15%"),
				),
			)
		}

		r.Status = types.ResultStatusValid
		if err := k.SetResult(ctx, r); err != nil {
			// ERR-1：写入失败不得静默吞掉（序列化异常属确定性故障，记录以便排障）。
			ctx.Logger().Error("edgeai: SetResult failed", "err", err.Error())
		}
		task.Status = types.TaskStatusDone
		if err := k.SetTask(ctx, task); err != nil {
			// ERR-1：写入失败不得静默吞掉（序列化异常属确定性故障，记录以便排障）。
			ctx.Logger().Error("edgeai: SetTask failed", "err", err.Error())
		}

		// ====================================================================
		// V3 新增：推荐奖励 hook（白皮书行 528-540）
		// ====================================================================
		// 任务结算拨付成功后，若 submitter 存在有效推荐关系，则按 rewardRateBps
		// 从生态池向 inviter 记入推荐奖励（受日熔断上限约束，超限拒绝但不影响本次结算）。
		if k.referralKeeper != nil {
			if refErr := k.referralKeeper.TrackEdgeAIReward(ctx, r.Submitter, sdkmath.NewIntFromUint64(submitterAmount)); refErr != nil {
				k.Logger(ctx).Info("edgeai: referral reward not tracked",
					"task_id", r.TaskId, "submitter", r.Submitter, "reason", refErr.Error())
			}
		}

		// 发射结算事件
		ctx.EventManager().EmitEvent(
			sdk.NewEvent("edgeai.Settled",
				sdk.NewAttribute("task_id", r.TaskId),
				sdk.NewAttribute("submitter", r.Submitter),
				sdk.NewAttribute("submitter_amount", strconv.FormatUint(submitterAmount, 10)),
				sdk.NewAttribute("verifier_reserve", strconv.FormatUint(verifierReserveAmount, 10)),
				sdk.NewAttribute("result_status", types.ResultStatusValid),
			),
		)
		ctx.EventManager().EmitEvent(
			sdk.NewEvent("edgeai.RewardPaid",
				sdk.NewAttribute("task_id", r.TaskId),
				sdk.NewAttribute("submitter", r.Submitter),
				sdk.NewAttribute("amount", strconv.FormatUint(submitterAmount, 10)),
			),
		)
		// O1 业务指标：edgeai 贡献即挖矿拨付计数（经 app telemetry 在 /metrics 暴露）。
		telemetry.IncrCounter(1, "edgeai", "reward_paid_count")
		telemetry.IncrCounter(float32(amount), "edgeai", "reward_paid_amount")

		if !settledTaskIDs[r.TaskId] {
			settledTaskIDs[r.TaskId] = true
			settledCount++
			// 声誉更新：任务通过 → +1（白皮书行 497）
			k.IncrementReputation(ctx, r.Submitter, types.ReputationPassIncrease)
		}
	}

	// Phase 2: 任务过期处理
	// 遍历所有 open 状态的任务，超时未结算（TaskExpireBlocks）的标记为 expired，
	// 退还托管金给任务创建者。
	//
	// SCALE-1：只遍历 open 任务索引的有界轮转批次，不再扫描全量任务表。
	if settledCount < types.MaxTasksPerBlock {
		for _, tid := range k.OpenTaskBatch(ctx, types.MaxOpenTaskScanPerBlock) {
			if settledCount >= types.MaxTasksPerBlock {
				break
			}
			task, err := k.GetTask(ctx, tid)
			if err != nil || task == nil {
				continue
			}
			if task.Status != types.TaskStatusOpen {
				continue
			}
			// 任务过期判定：优先使用 CreatedAtBlock（protobuf 持久化后生效），
			// 若为 0（旧任务或 proto 未重新生成），回退到基于 CreatedAt 时间戳的近似计算。
			expired := false
			if task.CreatedAtBlock > 0 {
				expired = uint64(ctx.BlockHeight()-task.CreatedAtBlock) >= types.TaskExpireBlocks
			} else {
				// 回退：假设 ~6 秒/区块，将 TaskExpireBlocks 映射为秒
				expireSec := int64(types.TaskExpireBlocks) * 6
				expired = ctx.BlockTime().Unix()-task.CreatedAt > expireSec
			}
			if !expired {
				continue
			}

			task.Status = types.TaskStatusExpired
			if err := k.SetTask(ctx, task); err != nil {
				// ERR-1：写入失败不得静默吞掉（序列化异常属确定性故障，记录以便排障）。
				ctx.Logger().Error("edgeai: SetTask failed", "err", err.Error())
			}

				// 退还托管金给创建者
				if task.Reward > 0 {
					creatorAddr, addrErr := sdk.AccAddressFromBech32(task.Creator)
					if addrErr == nil {
						// OVF-1：task.Reward 虽经 CreateTask 上界校验，但 genesis/迁移
						// 路径可能绕过；int64() 回绕会让退款额翻负。全程用 uint64 → Int。
						rewardCoins := sdk.NewCoins(sdk.NewCoin(types.EdgeAIDenom, sdkmath.NewIntFromUint64(task.Reward)))
						if refundErr := k.bankKeeper.SendCoinsFromModuleToAccount(
							ctx, types.ModuleName, creatorAddr, rewardCoins,
						); refundErr != nil {
							k.Logger(ctx).Error("edgeai: task expired refund failed",
								"task_id", tid, "creator", task.Creator, "err", refundErr.Error())
						}
					}
				}

			ctx.EventManager().EmitEvent(
				sdk.NewEvent("edgeai.TaskExpired",
					sdk.NewAttribute("task_id", tid),
					sdk.NewAttribute("creator", task.Creator),
					sdk.NewAttribute("reward", strconv.FormatUint(task.Reward, 10)),
				),
			)
			settledCount++
		}
	}

	// Phase 3: Verifier 多验证者投票评分（白皮书行 496-497）
	// 替换原单体 auto-pass 逻辑：从合格节点中抽取 N 个验证者，
	// 每人独立对已完成任务打分 (0-100)，去掉最高最低分取中位数，
	// 中位数 ≥ ThresholdScore 则通过并奖励，否则拒绝并进入争议。
	k.ScoreAndVerify(ctx)
}

// resolveDispute 将争议标记结案（供 BeginBlock 乐观结算使用）。
func (k Keeper) resolveDispute(ctx sdk.Context, d *Dispute, resolution string) {
	d.Status = "resolved"
	d.Resolution = resolution
	if err := k.SetDispute(ctx, d); err != nil {
		// ERR-1：写入失败不得静默吞掉（序列化异常属确定性故障，记录以便排障）。
		ctx.Logger().Error("edgeai: SetDispute failed", "err", err.Error())
	}
	ctx.EventManager().EmitEvent(
		sdk.NewEvent("edgeai.DisputeResolved",
			sdk.NewAttribute("task_id", d.TaskId),
			sdk.NewAttribute("resolution", resolution),
		),
	)
}
