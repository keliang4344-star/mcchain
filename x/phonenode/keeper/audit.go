package keeper

import (
	"encoding/binary"
	"encoding/json"
	"sort"

	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/phonenode/types"
)

// IndexConsistencyReport 心跳索引与节点主记录的一致性巡检结果。
//
// 设计原则：**只报告，不触发 halt**。这些检查里有若干项会被「外部因素」影响
// （例如有人直接给某地址转账、或历史残留数据），把它们注册进 crisis 模块会
// 让一次无关的误报把整条链停掉。因此本巡检是运维工具：定期跑、看报告、人工判断。
type IndexConsistencyReport struct {
	// IndexEntries 心跳索引中的条目总数。
	IndexEntries int `json:"index_entries"`
	// NodeRecords 节点主记录总数。
	NodeRecords int `json:"node_records"`
	// OrphanEntries 索引有、主记录无的地址（幽灵条目）。
	// SlashIfBad 不出队会制造这类条目：它们每块被扫到却什么都不做，白吃预算。
	OrphanEntries []string `json:"orphan_entries"`
	// StaleEntries 索引高度与主记录 LastProofBlock 不符的地址（残影）。
	// 会让离线检测读到过期位置，导致判定空转或误判。
	StaleEntries []string `json:"stale_entries"`
	// MissingEntries 主记录有心跳高度、索引中却没有的地址。
	// 这类节点永远不会被离线检测扫到 —— 即"漏检"，是出队逻辑被过度清理的信号。
	MissingEntries []string `json:"missing_entries"`
	// ZombieEntries attestation 已失效（被吊销）却仍留在索引中的地址。
	// 直接违反「处理过必出队」，是检测容量被吃的头号原因。
	ZombieEntries []string `json:"zombie_entries"`
}

// Consistent 报告是否完全一致（无任何一类不一致）。
func (r IndexConsistencyReport) Consistent() bool {
	return len(r.OrphanEntries) == 0 &&
		len(r.StaleEntries) == 0 &&
		len(r.MissingEntries) == 0 &&
		len(r.ZombieEntries) == 0
}

// ProblemCount 返回不一致条目总数，便于告警阈值判断。
func (r IndexConsistencyReport) ProblemCount() int {
	return len(r.OrphanEntries) + len(r.StaleEntries) +
		len(r.MissingEntries) + len(r.ZombieEntries)
}

// AuditHeartbeatIndex 全量核对心跳索引与节点主记录的一致性。
//
// ⚠ 这是**巡检工具，不在共识路径**：复杂度 O(设备数)，绝不可在 BeginBlock
// 或任何交易处理路径中调用（那正是要避免的全量遍历）。
// 适用场景：运维定期巡检、升级前后对账、故障排查、以及回归测试断言。
//
// 检查四类不一致，以核对容量性质是否成立：
//
//	OrphanEntries  索引条目的地址没有主记录          → 幽灵条目
//	StaleEntries   索引高度 != 主记录 LastProofBlock → 残影，判定会空转
//	MissingEntries 有主记录且有心跳高度但索引里没有  → 漏检（该判离线却扫不到）
//	ZombieEntries  attestation 已吊销但仍在索引中    → 违反「处理过必出队」
//
// 注意 ZombieEntries 的判据是「attestation **存在且非 valid**」：
// 刚注册尚未 attest 的节点也会出现在索引中（注册即起算宽限），这是正常的，
// 不能算作僵尸。
func (k Keeper) AuditHeartbeatIndex(ctx sdk.Context) IndexConsistencyReport {
	root := ctx.KVStore(k.storeKey)
	rep := IndexConsistencyReport{}

	// 第一遍：收集索引中全部条目（地址 → 索引记录的高度）。
	indexed := make(map[string]int64)
	indexedAddrs := make([]string, 0)
	{
		store := prefix.NewStore(root, types.HeartbeatIndexKeyPrefix)
		it := store.Iterator(nil, nil)
		for ; it.Valid(); it.Next() {
			key := it.Key()
			if len(key) <= 8 {
				continue // 缺高度段的异常键
			}
			addr := string(key[8:])
			indexed[addr] = int64(binary.BigEndian.Uint64(key[:8]))
			indexedAddrs = append(indexedAddrs, addr)
			rep.IndexEntries++
		}
		it.Close()
	}

	// 第二遍：遍历主记录，与索引比对；命中即从 indexed 中移除，
	// 遍历结束后仍留在 indexed 里的就是「索引有、主记录无」的孤儿。
	{
		store := prefix.NewStore(root, NodeKeyPrefix)
		it := store.Iterator(nil, nil)
		for ; it.Valid(); it.Next() {
			addr := string(it.Key())
			rep.NodeRecords++

			var st NodeState
			if err := json.Unmarshal(it.Value(), &st); err != nil {
				continue
			}
			h, ok := indexed[addr]
			if !ok {
				// 主记录声称有心跳高度、且 attestation 仍有效（即仍需要被检测），
				// 索引里却没有 → 真漏检。
				//
				// 判据不能只看 st.LastProofBlock > 0：已被 slash 出队的节点，
				// 其 LastProofBlock 依旧停在被罚时的旧高度，但它已经"处理过、
				// 不再需要检测"，索引里没有它是**正确**的。用 attestation 是否
				// 有效作为唯一依据，才能同时说清「该有」与「不该有」。
				if st.LastProofBlock > 0 && k.hasValidAttestation(ctx, addr) {
					rep.MissingEntries = append(rep.MissingEntries, addr)
				}
				continue
			}
			if h != st.LastProofBlock {
				rep.StaleEntries = append(rep.StaleEntries, addr)
			}
			delete(indexed, addr)
		}
		it.Close()
	}

	for addr := range indexed {
		rep.OrphanEntries = append(rep.OrphanEntries, addr)
	}

	// 第三遍：索引中的条目若对应「已吊销」的 attestation，说明它本该被出队却
	// 还留在队列里 —— 这正是出队彻底性的核心目标，单独统计以便回归观测。
	for _, addr := range indexedAddrs {
		att, ok := k.GetAttestation(ctx, addr)
		if !ok {
			// 注册即入队（LastProofBlock = 注册高度），此时尚未 attest 属正常，
			// 它是在等待首次认证，不是僵尸条目。
			continue
		}
		if att.Status != types.AttestationStatusValid {
			rep.ZombieEntries = append(rep.ZombieEntries, addr)
		}
	}

	// map 遍历顺序随机，排序保证报告可复现（便于比对两次巡检结果）。
	for _, s := range [][]string{rep.OrphanEntries, rep.StaleEntries,
		rep.MissingEntries, rep.ZombieEntries} {
		sort.Strings(s)
	}
	return rep
}

// hasValidAttestation 判断某地址是否持有仍有效的 attestation。
//
// 这是判断「该节点是否仍需要被离线检测」的唯一依据：只有认证有效的节点才应
// 占据索引条目；一旦认证被吊销（含被 slash），条目就应当出队。
func (k Keeper) hasValidAttestation(ctx sdk.Context, addr string) bool {
	att, ok := k.GetAttestation(ctx, addr)
	return ok && att.Status == types.AttestationStatusValid
}
