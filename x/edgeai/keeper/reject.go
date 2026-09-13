package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/edgeai/types"
)

// rejectResultsForTask 把任务下全部 pending 结果置为 Rejected。
//
// cheat 结案路径（仲裁裁定 / 超时自动结案 / 链上重算确认）
// 若都不修改结果状态，结果会永久滞留在 pending 索引里 —— 每个僵尸条目每区块
// 被 PendingResultBatch 反复扫到并占用 MaxTasksPerBlock 结算槽位，累积后
// 结算吞吐按比例衰减直至饿死，且状态只增不减。
//
// SetResult 是结果状态唯一写入点，非 pending 状态会同步删除 pending 索引
// （见 scan.go setPendingResultIndex），因此置 Rejected 即完成索引清理。
//
// 复杂度：与该任务的提交者数成正比（ResultsByTask 前缀迭代，确定性），
// 与全链历史结果总量无关，不会引入无界开销。
func (k Keeper) rejectResultsForTask(ctx sdk.Context, taskID string) {
	rejected := 0
	for _, r := range k.ResultsByTask(ctx, taskID) {
		if r.Status != types.ResultStatusPending {
			continue
		}
		r.Status = types.ResultStatusRejected
		if err := k.SetResult(ctx, r); err != nil {
			// 序列化异常属确定性故障，记录以便排障；不中断其余结果清理。
			ctx.Logger().Error("edgeai: reject result failed", "task_id", taskID, "err", err.Error())
			continue
		}
		rejected++
	}
	if rejected > 0 {
		ctx.EventManager().EmitEvent(sdk.NewEvent(
			"edgeai.ResultsRejected",
			sdk.NewAttribute("task_id", taskID),
			sdk.NewAttribute("count", sdk.NewInt(int64(rejected)).String()),
			sdk.NewAttribute("reason", "dispute_cheat_resolution"),
		))
	}
}
