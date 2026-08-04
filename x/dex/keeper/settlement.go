package keeper

import (
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"mcchain/x/dex/types"
)

// ---------------------------------------------------------------------------
// 离链高频微结算批处理（2026-08 落地）
//
// 高频微支付在链下聚合并构造 Merkle 树；链上仅执行「最终清算」：一个批次交易即可
// 把多笔微支付净额结算给各接收方，显著降低链上 gas 与拥堵（白皮书 §18）。
//
// 流程：链下聚合器调用 SubmitBatch 提交批次根 + 各条目 → FinalizeBatch 从结算源
// 模块账户（dex）一次性拨付给各接收方。条目资金不足时 best-effort 跳过该条目。
// ---------------------------------------------------------------------------

var SettlementBatchKeyPrefix = []byte("SettleBatch:")

// BatchEntry 单笔微结算条目。
type BatchEntry struct {
	Recipient string `json:"recipient"`
	Amount    uint64 `json:"amount"` // umc
}

// SettlementBatch 一个离链聚合批次的链上清算记录。
type SettlementBatch struct {
	BatchID     string       `json:"batch_id"`
	MerkleRoot  string       `json:"merkle_root"` // hex
	Entries     []BatchEntry `json:"entries"`
	Total       uint64       `json:"total"`
	SubmittedBy string       `json:"submitted_by"`
	Status      string       `json:"status"` // pending / settled
	BlockHeight int64        `json:"block_height"`
}

func settlementBatchKey(id string) []byte { return append(SettlementBatchKeyPrefix, []byte(id)...) }

// SubmitBatch 提交一个离链聚合批次（根 + 条目），进入 pending 待清算。
func (k Keeper) SubmitBatch(ctx sdk.Context, batchID, merkleRoot, submittedBy string, entries []BatchEntry) error {
	if batchID == "" || merkleRoot == "" {
		return fmt.Errorf("dex: batch_id and merkle_root are required")
	}
	if len(entries) == 0 {
		return fmt.Errorf("dex: batch must contain at least one entry")
	}
	var total uint64
	for _, e := range entries {
		if _, err := sdk.AccAddressFromBech32(e.Recipient); err != nil {
			return fmt.Errorf("dex: invalid recipient %s: %w", e.Recipient, err)
		}
		if e.Amount == 0 {
			return fmt.Errorf("dex: entry amount must be positive")
		}
		total += e.Amount
	}
	b := SettlementBatch{
		BatchID:     batchID,
		MerkleRoot:  merkleRoot,
		Entries:     entries,
		Total:       total,
		SubmittedBy: submittedBy,
		Status:      "pending",
		BlockHeight: ctx.BlockHeight(),
	}
	bz, _ := json.Marshal(b)
	ctx.KVStore(k.storeKey).Set(settlementBatchKey(batchID), bz)
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"dex.SettlementBatchSubmitted",
		sdk.NewAttribute("batch_id", batchID),
		sdk.NewAttribute("entries", fmt.Sprintf("%d", len(entries))),
		sdk.NewAttribute("total", fmt.Sprintf("%d", total)),
	))
	return nil
}

// GetBatch 读取批次清算记录；不存在返回 (nil, false)。
func (k Keeper) GetBatch(ctx sdk.Context, batchID string) (*SettlementBatch, bool) {
	bz := ctx.KVStore(k.storeKey).Get(settlementBatchKey(batchID))
	if bz == nil {
		return nil, false
	}
	var b SettlementBatch
	if err := json.Unmarshal(bz, &b); err != nil {
		return nil, false
	}
	return &b, true
}

// FinalizeBatch 将批次内各条目从结算源模块账户（dex）一次性净额拨付给接收方。
// 单条目资金不足时跳过该条目（best-effort），不影响其余条目。
func (k Keeper) FinalizeBatch(ctx sdk.Context, batchID string) error {
	b, ok := k.GetBatch(ctx, batchID)
	if !ok {
		return fmt.Errorf("dex: batch %s not found", batchID)
	}
	if b.Status == "settled" {
		return nil // 幂等
	}
	denom := "umc"
	for _, e := range b.Entries {
		amt := sdk.NewCoins(sdk.NewCoin(denom, sdk.NewIntFromUint64(e.Amount)))
		to, err := sdk.AccAddressFromBech32(e.Recipient)
		if err != nil {
			continue
		}
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, to, amt); err != nil {
			ctx.Logger().Error("dex: batch entry settlement failed",
				"batch", batchID, "recipient", e.Recipient, "err", err.Error())
			continue
		}
	}
	b.Status = "settled"
	bz, _ := json.Marshal(b)
	ctx.KVStore(k.storeKey).Set(settlementBatchKey(batchID), bz)
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"dex.SettlementBatchFinalized",
		sdk.NewAttribute("batch_id", batchID),
		sdk.NewAttribute("total", fmt.Sprintf("%d", b.Total)),
	))
	return nil
}
