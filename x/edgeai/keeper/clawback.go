package keeper

import (
	"fmt"

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
// 边界：若任务在作弊裁定前已被结算（85% 已拨付给提交者），模块账户不再持有该
// 份额，转账会失败，此时仅记录日志（提交者仍会被独立 slash）。
func (k Keeper) clawbackSubmitterReward(ctx sdk.Context, taskID string) {
	task, err := k.GetTask(ctx, taskID)
	if err != nil || task == nil || task.Reward == 0 {
		return
	}

	submitterAmount := task.Reward * uint64(types.EdgeAISubmitterRatioBps) / 10000
	if submitterAmount == 0 {
		return
	}

	clawCoin := sdk.NewCoins(sdk.NewInt64Coin(types.EdgeAIDenom, int64(submitterAmount)))
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
