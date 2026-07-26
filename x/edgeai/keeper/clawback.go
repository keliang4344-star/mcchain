package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"mcchain/x/edgeai/types"
)

// clawbackSubmitterReward reclaims the submitter's escrowed 80% reward when a
// dispute is resolved as cheat.
//
// 经济模型（需求方付费 / escrow）：任务创建时需求方将全额 reward 托管进
// edgeai 模块账户。结算前，本应拨付给提交者的 80% 仍留在模块账户（处于托管状态）。
// 当仲裁裁定作弊时，这部分"提交者奖励"被销毁（退出流通 / 回退到模块奖励池），
// 因此作弊提交者无法领取，同时由 phonenode 模块另行 slash。
//
// 80/15/5 分账比例保持不变（提交者 80% / 验证者预留 15% / 销毁 5%）；本函数只
// 在作弊裁定这一非正常路径上回收提交者那份，正常结算路径仍由 BeginBlock 发放。
//
// 边界：若任务在作弊裁定前已被结算（80% 已拨付给提交者），模块账户不再持有该
// 份额，BurnCoins 会失败，此时仅记录日志（提交者仍会被独立 slash）。
func (k Keeper) clawbackSubmitterReward(ctx sdk.Context, taskID string) {
	task, err := k.GetTask(ctx, taskID)
	if err != nil || task == nil || task.Reward == 0 {
		return
	}

	submitterAmount := task.Reward * uint64(types.EdgeAISubmitterRatioBps) / 10000
	if submitterAmount == 0 {
		return
	}

	burnCoin := sdk.NewCoins(sdk.NewInt64Coin(types.EdgeAIDenom, int64(submitterAmount)))
	if err := k.bankKeeper.BurnCoins(ctx, types.ModuleName, burnCoin); err != nil {
		k.Logger(ctx).Error("edgeai: clawback submitter reward failed",
			"task_id", taskID, "amount", submitterAmount, "err", err.Error())
		return
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("edgeai.ClawedBack",
			sdk.NewAttribute("task_id", taskID),
			sdk.NewAttribute("amount", fmt.Sprintf("%d", submitterAmount)),
			sdk.NewAttribute("reason", "dispute_cheat"),
		),
	)
}
