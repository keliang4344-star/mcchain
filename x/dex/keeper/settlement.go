package keeper

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"mcchain/internal/safemath"
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
	MerkleRoot  string       `json:"merkle_root"` // hex（链下聚合器构造，供收款方链下自验）
	Entries     []BatchEntry `json:"entries"`
	EntriesHash string       `json:"entries_hash"` // 链上条目完整性承诺（提交即定，清算时复核）
	Total       uint64       `json:"total"`
	SubmittedBy string       `json:"submitted_by"`
	Status      string       `json:"status"` // pending / settled
	BlockHeight int64        `json:"block_height"`
}

// computeEntriesHash 对条目做规范化承诺：sha256(recipient:amount;...) 的 hex。
// 用于链上保证「提交的条目 == 清算的条目」，防止批次在 pending 期间被置换。
func computeEntriesHash(entries []BatchEntry) string {
	h := sha256.New()
	for _, e := range entries {
		h.Write([]byte(e.Recipient))
		h.Write([]byte(":"))
		h.Write([]byte(fmt.Sprintf("%d", e.Amount)))
		h.Write([]byte(";"))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func settlementBatchKey(id string) []byte { return append(SettlementBatchKeyPrefix, []byte(id)...) }

// SubmitBatch 提交一个离链聚合批次（根 + 条目），进入 pending 待清算。
//
// batch_id 一经提交即不可复用：无论目标批次处于 pending 还是 settled，重复提交
// 一律拒绝。此前缺少该校验时，运营方（或其被盗密钥）可用同一 batch_id 覆写一个
// 已清算批次并再次 Finalize，导致同一笔款项被重复拨付。
func (k Keeper) SubmitBatch(ctx sdk.Context, batchID, merkleRoot, submittedBy string, entries []BatchEntry) error {
	if batchID == "" || merkleRoot == "" {
		return fmt.Errorf("dex: batch_id and merkle_root are required")
	}
	if len(entries) == 0 {
		return fmt.Errorf("dex: batch must contain at least one entry")
	}
	if len(entries) > types.MaxSettlementEntries {
		return fmt.Errorf("dex: batch may contain at most %d entries, got %d",
			types.MaxSettlementEntries, len(entries))
	}
	if existing, found := k.GetBatch(ctx, batchID); found {
		return fmt.Errorf("dex: batch %s already exists (status=%s); batch_id must be unique",
			batchID, existing.Status)
	}
	var total uint64
	seen := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		if _, err := sdk.AccAddressFromBech32(e.Recipient); err != nil {
			return fmt.Errorf("dex: invalid recipient %s: %w", e.Recipient, err)
		}
		if e.Amount == 0 {
			return fmt.Errorf("dex: entry amount must be positive")
		}
		// 同一批次内接收方去重：重复条目会让 entries_hash 承诺与实际净额脱节，
		// 也是把单笔支付放大成多笔的最简手法。
		if _, dup := seen[e.Recipient]; dup {
			return fmt.Errorf("dex: duplicate recipient %s in batch %s", e.Recipient, batchID)
		}
		seen[e.Recipient] = struct{}{}
		// total 是 uint64：不做检查时，构造溢出可让链上记录的批次总额远小于实际拨付额。
		sum, ok := safemath.AddUint64(total, e.Amount)
		if !ok {
			return fmt.Errorf("dex: batch %s total overflows uint64", batchID)
		}
		total = sum
	}
	b := SettlementBatch{
		BatchID:     batchID,
		MerkleRoot:  merkleRoot,
		Entries:     entries,
		EntriesHash: computeEntriesHash(entries),
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

// UnencumberedBalance 返回 dex 模块账户中「未被 AMM 池储备占用」的某币种余额。
//
// dex 模块账户同时托管两类资金：① 各流动性池的储备（属于 LP，绝不可挪用）；
// ② 微结算可动用的运营资金。二者混在同一账户，若清算时不做区分，一个批次即可
// 把 LP 储备拨走——链上池状态仍记着储备，实际代币却已不在，池子瞬间资不抵债。
// 因此清算前必须以「总余额 − 全部池储备」为可用额度做偿付能力校验。
func (k Keeper) UnencumberedBalance(ctx sdk.Context, denom string) sdk.Int {
	moduleAddr := authtypes.NewModuleAddress(types.ModuleName)
	available := k.bankKeeper.GetBalance(ctx, moduleAddr, denom).Amount

	for _, p := range k.GetAllPools(ctx) {
		if p.DenomA == denom {
			if v, ok := sdk.NewIntFromString(p.ReserveA); ok && v.IsPositive() {
				available = available.Sub(v)
			}
		}
		if p.DenomB == denom {
			if v, ok := sdk.NewIntFromString(p.ReserveB); ok && v.IsPositive() {
				available = available.Sub(v)
			}
		}
	}
	if available.IsNegative() {
		return sdk.ZeroInt()
	}
	return available
}

// FinalizeBatch 将批次内各条目从结算源模块账户（dex）一次性净额拨付给接收方。
//
// 原子语义：任一条目拨付失败即整批返回错误，本次交易的全部状态变更回滚。
// 旧实现对失败条目「best-effort 跳过」后仍把批次标记为 settled——收款方的钱
// 永久丢失且批次不可重提（batch_id 唯一），属于静默资损，已改为原子清算。
func (k Keeper) FinalizeBatch(ctx sdk.Context, batchID string) error {
	b, ok := k.GetBatch(ctx, batchID)
	if !ok {
		return fmt.Errorf("dex: batch %s not found", batchID)
	}
	if b.Status == "settled" {
		return nil // 幂等
	}
	// 条目完整性复核：清算时实际条目必须仍与提交时承诺的 hash 一致，
	// 防止批次在 pending 期间被置换（防御性校验，A1）。
	if got := computeEntriesHash(b.Entries); got != b.EntriesHash {
		return fmt.Errorf("dex: batch %s entries hash mismatch: stored %s, recomputed %s", batchID, b.EntriesHash, got)
	}

	const denom = "umc"

	// 偿付能力前置校验：批次总额不得动用任何 AMM 池储备。
	total := sdk.NewIntFromUint64(b.Total)
	if avail := k.UnencumberedBalance(ctx, denom); avail.LT(total) {
		return fmt.Errorf(
			"dex: batch %s requires %s %s but only %s is unencumbered (pool reserves are not spendable)",
			batchID, total.String(), denom, avail.String())
	}

	for _, e := range b.Entries {
		to, err := sdk.AccAddressFromBech32(e.Recipient)
		if err != nil {
			return fmt.Errorf("dex: batch %s has invalid recipient %s: %w", batchID, e.Recipient, err)
		}
		amt := sdk.NewCoins(sdk.NewCoin(denom, sdk.NewIntFromUint64(e.Amount)))
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, to, amt); err != nil {
			return fmt.Errorf("dex: batch %s entry to %s failed: %w", batchID, e.Recipient, err)
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
