package keeper

import (
	"context"
	"strconv"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"mcchain/x/depin/types"
)

// SubmitContribution records a verified contribution from a device and, when the
// computed reward is positive, pays it out from the DePIN reward pool (minted at
// genesis). Payout requires the device address to first be registered as a
// phonenode (association key: SubmitContribution.Creator == phonenode node
// Address), enforced below as the "发币闸口".
//
// V3 白皮书对照补齐（行 366-382）：
//   - 七层防刷量防线（defense.go）在 attestation 检查通过后、入账前执行
//   - 共振分发算法（resonance.go）在基础奖励计算后调整最终奖励
//   - 线性摊薄释放（release.go）在发币前检查日释放额度
func (k msgServer) SubmitContribution(goCtx context.Context, msg *types.MsgSubmitContribution) (*types.MsgSubmitContributionResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// score 在消息层为 string，需解析为 int
	score, err := strconv.Atoi(msg.Score)
	if err != nil {
		return nil, sdkerrors.Wrapf(types.ErrInvalidScore, "score %q is not an integer", msg.Score)
	}

	// 贡献设备身份 = 提交者（Creator）。必须已注册且通过 attestation（防女巫）。
	deviceAddr := msg.Creator
	if _, err := sdk.AccAddressFromBech32(deviceAddr); err != nil {
		return nil, sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address (%s)", err)
	}

	// task_id 直接进入 KVStore 键（Contribution:<taskID>），
	// 状态层再设一道长度闸门（ValidateBasic 可被绕过，这里才是权威口径）。
	if msg.TaskId == "" || len(msg.TaskId) > types.MaxContributionTaskIDLength {
		return nil, sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest,
			"task_id length out of range (expect 1..%d bytes, got %d)",
			types.MaxContributionTaskIDLength, len(msg.TaskId))
	}

	st, err := k.Keeper.GetDevice(ctx, deviceAddr)
	if err != nil {
		return nil, err
	}
	// 认证必须「仍在有效期内」，不能凭一次旧认证长期兑付。
	if !IsDeviceAttestationValid(st, ctx.BlockTime().Unix()) {
		return nil, types.ErrDeviceNotAttested
	}

	// ========================================================================
	// V3 新增：七层防刷量防线（defense.go，白皮书行 378-382）
	// ========================================================================
	// 在入账前执行全部七层防线，任一失败则拒绝本次贡献。
	// 计数器策略：仅身份/认证类失败（Layer 1/2）重置
	// 连续提交计数器（设备换硬件/认证过期，历史与当前设备无关）；
	// 活跃度异常（Layer 5）拒绝时惩罚性减半（可恢复限流）；
	// 其余失败（频率/质量/分散度/经济上限）不重置 —— 否则攻击者可故意触发
	// 低层失败来清零计数，绕过 MaxConsecutiveSubmissions。
	defenseResult := k.Keeper.RunDefensePipeline(ctx, deviceAddr, msg.TaskType, score)
	if !defenseResult.Passed {
		switch defenseResult.FailedLayer {
		case 1, 2:
			k.Keeper.ResetConsecutiveCounter(ctx, deviceAddr)
		case 5:
			k.Keeper.HalveConsecutiveCounter(ctx, deviceAddr)
		}
		ctx.EventManager().EmitEvent(
			sdk.NewEvent("depin.DefenseBlocked",
				sdk.NewAttribute("device", deviceAddr),
				sdk.NewAttribute("task_id", msg.TaskId),
				sdk.NewAttribute("task_type", msg.TaskType),
				sdk.NewAttribute("failed_layer", strconv.Itoa(defenseResult.FailedLayer)),
				sdk.NewAttribute("reason", defenseResult.RejectReason),
			),
		)
		return nil, sdkerrors.Wrapf(types.ErrInvalidScore, "defense layer %d: %s", defenseResult.FailedLayer, defenseResult.RejectReason)
	}

	// 落盘 + 计奖（不在此处发币）
	reward, err := k.Keeper.SubmitAndReward(ctx, msg.TaskId, deviceAddr, msg.TaskType, score)
	if err != nil {
		// 此处多为确定性错误（如重复 taskId），
		// 不再重置计数器 —— 原重置逻辑与「故意触发失败清零计数」的绕过路径相同。
		return nil, err
	}

	// ========================================================================
	// V3 新增：共振分发算法调整奖励（resonance.go，白皮书行 366-376）
	// ========================================================================
	// 倍数改用 sdk.Dec 定点表示，与链上实际发放口径完全一致，
	// 且不引入任何浮点运算——事件属性也必须在各节点上逐位相同。
	resonanceMultiplier := sdk.OneDec()
	if reward > 0 {
		adjustedReward := k.Keeper.ComputeResonanceRewardWithContext(ctx, reward, deviceAddr, score)
		if adjustedReward != reward {
			resonanceMultiplier = ResonanceMultiplierOf(reward, adjustedReward)
			reward = adjustedReward
		}
		// 更新贡献记录的奖励字段（共振调整后）
		if c, ok := k.Keeper.GetContribution(ctx, msg.TaskId); ok {
			c.Reward = reward
			if setErr := k.Keeper.SetContribution(ctx, c); setErr != nil {
				k.Keeper.Logger(ctx).Error("depin: update contribution reward after resonance",
					"task_id", msg.TaskId, "err", setErr.Error())
			}
		}
	}

	// 仅当奖励 > 0 时，从 DePIN 模块账户（方案 A 池）向设备拨付。
	if reward > 0 {
		// 发币闸口前置关联校验——设备地址必须先在 phonenode 注册为节点，
		// 否则拒绝发币（不铸、不拨）。关联键 = SubmitContribution.Creator == 节点 Address。
		if !k.Keeper.phonenodeKeeper.HasNode(ctx, deviceAddr) {
			return nil, types.ErrPhonenodeNotRegistered
		}

		// 反女巫闸口：设备地址必须持有有效 attestation，否则拒绝拨付（与 depin 自身
		// Attested 叠加）。未 attest 节点即使注册也不发币，强制硬件 attestation。
		if !k.Keeper.phonenodeKeeper.IsAttested(ctx, deviceAddr) {
			return nil, types.ErrDeviceNotAttested
		}

		// 强化：生产链要求「预言机已背书且在有效期内」（IsVerifiedAttested），
		// 而不是仅凭设备自签的 IsAttested。自签材料任何人可造，只作为参与门槛；
		// 真金白银的 DePIN 拨付必须有独立外部背书，且背书过期后需重新走真机校验。
		// 非生产链保持原有宽松口径（本地挖矿与 CI 不受影响）。
		if isProductionChainID(ctx.ChainID()) && !k.Keeper.phonenodeKeeper.IsVerifiedAttested(ctx, deviceAddr) {
			return nil, types.ErrDeviceNotAttested.Wrap(
				"production chain requires a valid oracle-endorsed attestation for payout")
		}

		toAddr, err := sdk.AccAddressFromBech32(deviceAddr)
		if err != nil {
			return nil, sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid device address (%s)", err)
		}

		// reward denom is a module param (default "umc"), not a const.
		denom := k.Keeper.GetParams(ctx).RewardDenom

		// ---- 设备任务赏金 100% 归节点（白皮书《优化定稿版》§24.6 否决清单）----
		// 早期版本曾对每笔赏金抽取 5% 销毁；该设计已被明确否决并撤销：
		// 通缩只应来自「协议使用费」（gas、DEX 手续费）与「作恶罚没」，
		// 绝不侵蚀参与者用真实算力/带宽/在线时长换来的劳动应得。
		// 节点完成任务应得多少，就足额拿到多少，链上不做任何截留。
		payoutAmount := uint64(reward)

		// ====================================================================
		// V3 新增：线性摊薄释放检查（release.go，白皮书行 372）
		// ====================================================================
		// 拨付之前检查日释放额度，确保每日发放不超线性摊薄上限。
		allowed, dailyCap, remaining, releaseErr := k.Keeper.CheckDailyReleaseCap(ctx, payoutAmount)
		if releaseErr != nil {
			k.Keeper.Logger(ctx).Error("depin: daily release cap check error", "err", releaseErr.Error())
		} else if !allowed {
			k.Keeper.Logger(ctx).Info("depin: daily release cap exceeded",
				"device", deviceAddr,
				"payout_amount", payoutAmount,
				"daily_cap", dailyCap,
				"remaining", remaining,
			)
			ctx.EventManager().EmitEvent(
				sdk.NewEvent("depin.ReleaseCapped",
					sdk.NewAttribute("device", deviceAddr),
					sdk.NewAttribute("task_id", msg.TaskId),
					sdk.NewAttribute("payout_amount", strconv.FormatUint(payoutAmount, 10)),
					sdk.NewAttribute("daily_cap", strconv.FormatUint(dailyCap, 10)),
					sdk.NewAttribute("remaining", strconv.FormatUint(remaining, 10)),
				),
			)
			return nil, sdkerrors.Wrapf(types.ErrInvalidScore, "daily release cap exceeded: need %d, cap %d, remaining %d", payoutAmount, dailyCap, remaining)
		}

		// payoutAmount 虽 ≤ MaxRewardPerTask，但统一用 uint64 → Int，无 int64 面。
		amt := sdk.NewCoins(sdk.NewCoin(denom, sdkmath.NewIntFromUint64(payoutAmount)))
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, toAddr, amt); err != nil {
			return nil, sdkerrors.Wrapf(err, "failed to pay reward from depin pool")
		}

		// ====================================================================
		// V3 新增：拨付成功后记录防线状态和释放额度
		// ====================================================================
		// 记录设备当日累计奖励（defense.go Layer7 日收益上限）
		k.Keeper.RecordDailyReward(ctx, deviceAddr, payoutAmount)
		// 记录当日全局释放额度（release.go 线性摊薄）
		k.Keeper.RecordDailyRelease(ctx, payoutAmount)

		// ====================================================================
		// V3 新增：推荐奖励 hook（白皮书行 528-540）
		// ====================================================================
		// 任务结算拨付成功后，若贡献者存在有效推荐关系，则按 rewardRateBps
		// 从生态池向 inviter 记入推荐奖励（受日熔断上限约束，超限拒绝但不影响本次拨付）。
		if k.Keeper.referralKeeper != nil {
			if refErr := k.Keeper.referralKeeper.TrackDepinReward(ctx, deviceAddr, sdkmath.NewIntFromUint64(payoutAmount)); refErr != nil {
				k.Keeper.Logger(ctx).Info("depin: referral reward not tracked",
					"device", deviceAddr, "task_id", msg.TaskId, "reason", refErr.Error())
			}
		}

		// 发币事件 —— 移动端 SDK 据此监听「贡献即挖矿」到账通知。
		ctx.EventManager().EmitEvent(
			sdk.NewEvent("depin.RewardPaid",
				sdk.NewAttribute("task_id", msg.TaskId),
				sdk.NewAttribute("device", deviceAddr),
				sdk.NewAttribute("task_type", msg.TaskType),
				sdk.NewAttribute("score", msg.Score),
				sdk.NewAttribute("reward", strconv.FormatUint(uint64(reward), 10)),
				sdk.NewAttribute("payout", strconv.FormatUint(payoutAmount, 10)),
				sdk.NewAttribute("denom", denom),
				sdk.NewAttribute("resonance_multiplier", resonanceMultiplier.String()),
			),
		)

		// O1 业务指标：depin 奖励拨付计数与累计金额（经 app telemetry 在 /metrics 暴露）。
		telemetry.IncrCounter(1, "depin", "reward_paid_count")
		telemetry.IncrCounter(float32(payoutAmount), "depin", "reward_paid_amount")
	}

	return &types.MsgSubmitContributionResponse{}, nil
}
