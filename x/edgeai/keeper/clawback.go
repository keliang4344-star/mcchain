package keeper

import (
	"fmt"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"mcchain/x/edgeai/types"
	tokenomicstypes "mcchain/x/tokenomics/types"
)

// clawbackSubmitterReward reclaims the submitter's escrowed 85% reward when a
// dispute is resolved as cheat.
//
// 经济模型（需求方付费 / escrow）：任务创建时需求方将全额 reward 托管进
// edgeai 模块账户。结算前，本应拨付给提交者的 85% 仍留在模块账户（处于托管状态）。
// 当仲裁裁定作弊时，作弊提交者无法领取这份托管款。
//
// 去向（2026-08 定稿）：回收款转入质押安全池，而非销毁。
// 按既定原则「作恶者的损失，变成诚实者的收益」——回收款经安全池滴灌补贴
// 诚实节点与验证人。此处不销毁：本函数回收的是需求方托管的任务付费，
// 与验证人 bonded 本金的罚没（40% 销毁 / 60% 回流）是两条独立路径。
//
// 85/15 分账比例保持不变（提交者 85% / 验证者预留 15%）；本函数只在作弊裁定
// 这一非正常路径上回收提交者那份，正常结算路径仍由 BeginBlock 发放。
//
// 边界：所有任务的托管金混在同一个模块账户里，
// 「转账会失败」的原假设不成立 —— 只要有任何其他任务未结算的托管金，
// 对已结算（Done）任务的 clawback 就会成功，把别的任务的托管金划走。
// 因此这里必须显式拒绝已结算任务的回收（提交者仍会被独立 slash）。
func (k Keeper) clawbackSubmitterReward(ctx sdk.Context, taskID string) {
	task, err := k.GetTask(ctx, taskID)
	if err != nil || task == nil || task.Reward == 0 {
		return
	}
	if task.Status == types.TaskStatusDone {
		// 已结算：85% 已拨付给提交者，模块账户里是其他任务的托管金，不可挪用。
		ctx.EventManager().EmitEvent(
			sdk.NewEvent("edgeai.ClawbackSkipped",
				sdk.NewAttribute("task_id", taskID),
				sdk.NewAttribute("reason", "task already settled; escrow belongs to other tasks"),
			),
		)
		return
	}

	// amount * 8500 在 uint64 上对超大 reward 回绕
	// （genesis/迁移路径可绕过经济上限）。85/15 拆分统一走 sdk.Int。
	submitterAmountInt := sdkmath.NewIntFromUint64(task.Reward).
		MulRaw(int64(types.EdgeAISubmitterRatioBps)).QuoRaw(10000)
	if !submitterAmountInt.IsPositive() {
		return
	}
	submitterAmount := submitterAmountInt.Uint64()

	// submitterAmount 由 task.Reward 推导（经 8500/10000），虽然
	// CreateTask 校验过 Reward ≤ MaxInt64，但 genesis/迁移路径可能绕过；
	// int64() 回绕会把回收额翻负。全程用 uint64 → Int。
	clawCoin := sdk.NewCoins(sdk.NewCoin(types.EdgeAIDenom, sdkmath.NewIntFromUint64(submitterAmount)))
	if err := k.bankKeeper.SendCoinsFromModuleToModule(
		ctx, types.ModuleName, tokenomicstypes.StakingSecurityPoolName, clawCoin,
	); err != nil {
		k.Logger(ctx).Error("edgeai: clawback submitter reward to security pool failed",
			"task_id", taskID, "amount", submitterAmount, "err", err.Error())
		return
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("edgeai.ClawedBack",
			sdk.NewAttribute("task_id", taskID),
			sdk.NewAttribute("amount", fmt.Sprintf("%d", submitterAmount)),
			sdk.NewAttribute("reason", "dispute_cheat"),
			sdk.NewAttribute("destination", tokenomicstypes.StakingSecurityPoolName),
		),
	)
}

// refundVerifierSlice 把 cheat 结案任务里「验证者预留」份额（15%）退还任务创建者。
//
//	正常结算路径把 reward 的 15% 存入 verifier_reserve（验证者抽检后领取）；
//	而 cheat 结案时任务从未结算，reserve 键不存在，这 15% 就一直留在 edgeai
//	模块账户 —— 既不给验证者（他们没提供有效验证），也不退创建者（任务作废），
//	成为无主沉淀。更严重的是 BeginBlock 的争议超时路径只标记 Cheated、
//	既不回收也不退款，创建者全额托管金永久卡死在模块账户。
//
//	现在 cheat 结案统一走「85% 回收至安全池 + 15% 退还创建者」，两条路径
//	（仲裁裁定 / 超时结案）口径一致，模块账户不留无主余额。
func (k Keeper) refundVerifierSlice(ctx sdk.Context, task *types.Task) {
	if task == nil || task.Reward == 0 || task.Creator == "" {
		return
	}
	// 与结算路径同口径的 sdk.Int 运算，避免 uint64 回绕。
	amtInt := sdkmath.NewIntFromUint64(task.Reward).
		MulRaw(int64(10000 - types.EdgeAISubmitterRatioBps)).QuoRaw(10000)
	if !amtInt.IsPositive() {
		return
	}
	creator, err := sdk.AccAddressFromBech32(task.Creator)
	if err != nil {
		k.Logger(ctx).Error("edgeai: refund verifier slice skipped — invalid creator",
			"task_id", task.Id, "creator", task.Creator, "err", err.Error())
		return
	}
	coin := sdk.NewCoins(sdk.NewCoin(types.EdgeAIDenom, amtInt))
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(
		ctx, types.ModuleName, creator, coin,
	); err != nil {
		k.Logger(ctx).Error("edgeai: refund verifier slice failed",
			"task_id", task.Id, "creator", task.Creator, "amount", amtInt.String(), "err", err.Error())
		return
	}
	ctx.EventManager().EmitEvent(
		sdk.NewEvent("edgeai.VerifierSliceRefunded",
			sdk.NewAttribute("task_id", task.Id),
			sdk.NewAttribute("creator", task.Creator),
			sdk.NewAttribute("amount", amtInt.String()),
			sdk.NewAttribute("reason", "task_cheated"),
		),
	)
}
