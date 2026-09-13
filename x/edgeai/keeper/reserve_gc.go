package keeper

import (
	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/edgeai/types"
)

// ---------------------------------------------------------------------------
// 验证者预留池过期退还
//
// 背景：结算时 15% 托管金记入 verifier_reserve，领取唯一入口 SubmitVerification
// 是 keeper 方法（无 Msg 入口，proto 工具链待就绪），导致预留永久沉淀为死资金。
//
// 方案：预留条目带结算高度（SetVerifierReserveAt），超过 VerifierReserveExpiryBlocks
// 未领取的，退还给任务创建者（托管金原始归属方）并删除条目。验证链路上线后，
// 只要在窗口内完成验证，机制不变；过期仅发生在验证者长期未提交的异常场景。
//
// 复杂度：每区块至多扫 VerifierReserveGCPerBlock 条（持久化游标轮转，与
// edgeai/scan.go boundedScan 同款设计），成本恒定、全网确定性一致。
// ---------------------------------------------------------------------------

const (
	// VerifierReserveExpiryBlocks 预留池领取窗口：7 天 × 21600 块/日（4s 出块）。
	VerifierReserveExpiryBlocks uint64 = 151200

	// VerifierReserveGCPerBlock 每区块过期检查预算。
	VerifierReserveGCPerBlock = 64
)

var verifierReserveCursorKey = []byte("cursor:verifier_reserve")

// refundExpiredVerifierReserves 扫描预留池，退还过期条目给任务创建者。
func (k Keeper) refundExpiredVerifierReserves(ctx sdk.Context) {
	root := ctx.KVStore(k.storeKey)
	ps := prefix.NewStore(root, verifierReservePrefix)

	now := uint64(ctx.BlockHeight())
	type refundOp struct {
		taskID  string
		creator string
		amount  uint64
	}
	var ops []refundOp
	var nextStart []byte

	it := ps.Iterator(root.Get(verifierReserveCursorKey), nil)
	scanned := 0
	for ; it.Valid(); it.Next() {
		// 预算按「扫描条数」计，不按「退款条数」计。
		// 删除/退款计数版本在稳态（预留均未过期）下 continue 不消耗预算，循环会扫穿
		// 整个前缀；BeginBlock 跑在 InfiniteGasMeter 下不受 max_gas 约束，结果是
		// 出块超时 → 全网停块且无法自愈。与 depin/defense.go gcSuffixedDateKeys、
		// referral/cap.go collectStaleKeys 统一为同一口径：scanned 先判预算再自增。
		if scanned >= VerifierReserveGCPerBlock {
			nextStart = append([]byte(nil), it.Key()...)
			break
		}
		scanned++

		taskID := string(it.Key())
		amount, settleHeight, ok := parseVerifierReserveValue(it.Value())
		if !ok || amount == 0 {
			// 损坏/空条目：直接删除，避免永久滞留。
			ops = append(ops, refundOp{taskID: taskID})
			continue
		}
		// 旧格式条目（settleHeight=0）无高度信息，不参与过期；已计入扫描预算。
		if settleHeight == 0 {
			continue
		}
		if now-settleHeight < VerifierReserveExpiryBlocks {
			continue // 未过期：已计入扫描预算
		}
		creator := ""
		if task, err := k.GetTask(ctx, taskID); err == nil && task != nil {
			creator = task.Creator
		}
		ops = append(ops, refundOp{taskID: taskID, creator: creator, amount: amount})
	}
	it.Close()

	if nextStart != nil {
		root.Set(verifierReserveCursorKey, nextStart)
	} else {
		root.Delete(verifierReserveCursorKey)
	}

	// 迭代结束后统一执行删除与转账。
	for _, op := range ops {
		root.Delete(verifierReserveKey(op.taskID))
		if op.amount == 0 || op.creator == "" {
			continue
		}
		creatorAddr, err := sdk.AccAddressFromBech32(op.creator)
		if err != nil {
			continue
		}
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(
			ctx, types.ModuleName, creatorAddr,
			sdk.NewCoins(sdk.NewCoin(types.EdgeAIDenom, sdkmath.NewIntFromUint64(op.amount))),
		); err != nil {
			k.Logger(ctx).Error("edgeai: verifier reserve refund failed",
				"task_id", op.taskID, "creator", op.creator, "err", err.Error())
			continue
		}
		ctx.EventManager().EmitEvent(sdk.NewEvent(
			"edgeai.VerifierReserveRefunded",
			sdk.NewAttribute("task_id", op.taskID),
			sdk.NewAttribute("creator", op.creator),
			sdk.NewAttribute("amount", sdkmath.NewIntFromUint64(op.amount).String()),
			sdk.NewAttribute("reason", "claim_window_expired"),
		))
	}
}

// getVerifierReserveEntryRaw 已由 state.go 的 parseVerifierReserveValue 取代。
