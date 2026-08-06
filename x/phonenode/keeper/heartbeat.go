package keeper

import (
	"bytes"

	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"mcchain/x/phonenode/types"
)

// DetectOffline 在 BeginBlock 调用：对持有有效 attestation 但超过 OfflineGraceBlocks
// 个区块未提交 state proof（在线心跳）的节点执行离线 slash。
//
// 仅对已 attest 节点检测；未 attest 节点不触发（其领取已被 depin 闸口拒绝）。
// 离线判据基于区块高度（LastProofBlock），与链上时间无关，避免弱网秒级抖动误 slash。
//
// SCALE-1（上线前审计发现的致命项）：原实现每个区块调用 AllNodes 把全量节点
// 读进内存。在移动节点规模上量后，单次 BeginBlock 的耗时会直接超过出块间隔，
// 链停摆。现改为走「按心跳高度排序」的索引：
//   - 索引键 = be8(LastProofBlock) + addr，字典序即高度序；
//   - 只扫描 [游标, 高度 < 当前高度-宽限期) 这一段，段内条目全是真正的超时候选；
//   - 每区块至多消费 MaxOfflineScanPerBlock 条，游标持久化于链上状态，
//     全网各节点批次一致，不引入非确定性；
//   - 游标越过本轮上界或扫到段尾即复位，形成公平轮转。
func (k Keeper) DetectOffline(ctx sdk.Context) {
	params := k.GetParams(ctx)
	if params.OfflineGraceBlocks <= 0 {
		return
	}
	curHeight := ctx.BlockHeight()
	// 心跳高度严格小于 cutoff 才算离线（与 (cur-last) > grace 等价）。
	cutoff := curHeight - params.OfflineGraceBlocks
	if cutoff <= 0 {
		return
	}

	root := ctx.KVStore(k.storeKey)
	ps := prefix.NewStore(root, types.HeartbeatIndexKeyPrefix)
	end := types.HeartbeatIndexBound(cutoff)

	start := root.Get(types.OfflineScanCursorKey)
	if start != nil && bytes.Compare(start, end) >= 0 {
		start = nil // 游标已越过本轮上界 → 复位，从头重新轮转
	}

	// 先取出本批候选并关闭迭代器，再执行 slash：
	// 避免「边迭代边写」在 cachekv 上产生未定义行为。
	it := ps.Iterator(start, end)
	addrs := make([]string, 0, types.MaxOfflineScanPerBlock)
	scanned := 0
	for ; it.Valid() && scanned < types.MaxOfflineScanPerBlock; it.Next() {
		scanned++
		key := it.Key()
		if len(key) <= 8 {
			continue // 异常键，跳过
		}
		addrs = append(addrs, string(key[8:]))
	}
	var next []byte
	if it.Valid() {
		next = append([]byte(nil), it.Key()...)
	}
	it.Close()

	if next != nil {
		root.Set(types.OfflineScanCursorKey, next)
	} else {
		root.Delete(types.OfflineScanCursorKey)
	}

	for _, addr := range addrs {
		att, ok := k.GetAttestation(ctx, addr)
		if !ok || att.Status != types.AttestationStatusValid {
			continue
		}
		st, err := k.GetNode(ctx, addr)
		if err != nil {
			continue
		}
		// 以主记录为准复核一次，防止索引残影导致误 slash。
		// 入网/重 attest 已将 LastProofBlock 置为当前高度，新生节点享有完整宽限。
		if (curHeight - st.LastProofBlock) <= params.OfflineGraceBlocks {
			continue
		}
		if err := k.SlashIfBad(ctx, addr, "offline", params.OfflineSlashBps); err != nil {
			ctx.Logger().Error("phonenode: offline slash failed", "node", addr, "err", err.Error())
		}
	}
}
