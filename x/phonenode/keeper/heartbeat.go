package keeper

import (
	"encoding/binary"
	"fmt"

	"github.com/cosmos/cosmos-sdk/store/prefix"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/phonenode/types"
)

// dueEntry 一条「已到期」的离线检测候选：心跳高度 + 节点地址。
type dueEntry struct {
	height int64
	addr   string
}

// DetectOffline 在 BeginBlock 调用：对持有有效 attestation 但超过 OfflineGraceBlocks
// 个区块未提交 state proof（在线心跳）的节点执行离线 slash。
//
// 仅对已 attest 节点检测；未 attest 节点不触发（其领取已被 depin 闸口拒绝）。
// 离线判据基于区块高度（LastProofBlock），与链上时间无关，避免弱网秒级抖动误 slash。
//
// 设计约束：不得每块调用 AllNodes 把全量节点读进内存 —— 移动节点规模上量后，单次
// BeginBlock 的耗时将超过出块间隔导致链停摆。故改为「按心跳高度排序」的有界索引。
//
// # 出队语义
//
// 「不全量读」只是第一步，容量仍被两处泄漏吃掉：
//
//  1. SlashIfBad 经 SetNode 回写状态时 LastProofBlock 并未变化，而 SetNode 只在
//     高度变化时才移除旧索引位——于是被 slash 的离线节点永久滞留索引；
//  2. 扫描中对「attestation 已失效 / 节点主记录已不存在」的条目只做 continue
//     而出队，这些条目下一区块会被原样再扫一遍，反复读状态却什么都做不了。
//
// 两者叠加后，索引头部（即最早心跳区段）持续堆积僵尸条目，把每块预算吃干净，
// 真正新离线的节点排队越来越久，检测延迟无上界增长；而节点资本津贴在此期间
// 照发，等于给「掉线不亏」开了口子。
//
// 现在语义是严格 FIFO 到期队列，并保证「处理过必出队」：
//
//   - 索引按 LastProofBlock 升序，头部即最早心跳 = 最早到期者；
//   - 每条被扫到的条目必定以 slash 或「判定为无需再检测」而离开索引，
//     因此持久化轮转游标不再是必需的，扫描严格前进、不会空转；
//   - 批次未取完即视为积压：置位积压标记，下一区块自动放大预算追赶（自愈），
//     并发出 phonenode.OfflineBacklog 事件，避免积压静默累积。
//
// 容量模型：在线节点每次心跳都会把索引条目重写到更大的高度，条目不断向索引
// 尾部迁移；索引头部只可能积累「已经停止心跳、真正离线」的节点。配合出队语义：
//
//	稳态每区块处理量 ≈ 该区块真正离线的节点数 ≈ 0
//
// 即容量与「已注册设备总数」解耦——不存在「N 台设备上限」。只有大面积同时
// 离线（如区域性断网）才会短时积压，由放大预算与事件告警覆盖。
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
	budget := types.MaxOfflineScanPerBlock
	if root.Get(types.OfflineBacklogFlagKey) != nil {
		// 上一区块出现积压 → 本区块放大预算追赶（突发大面积离线后的自愈路径）。
		budget = types.MaxOfflineScanPerBlockBacklog
	}

	// 先只读取候选并关闭迭代器，再执行 slash：
	// 「边迭代边写」在 cachekv 上属未定义行为。
	due, backlog, oldestLag := k.collectDueEntries(ctx, cutoff, budget)

	// 积压标记：批次未取完即置位，供下一区块放大预算；取完则清除。
	if backlog {
		root.Set(types.OfflineBacklogFlagKey, []byte{1})
	} else {
		root.Delete(types.OfflineBacklogFlagKey)
	}

	slashed := 0
	for _, e := range due {
		if k.resolveDueEntry(ctx, e, curHeight, params) {
			slashed++
		}
	}

	if backlog {
		// 积压可观测：链上事件 + telemetry。运营侧据此判断是否需要提高预算
		// 上限或排查区域性断网；oldest_lag_blocks 是下一条待处理条目已经逾期
		// 多久（> grace 说明该节点本应早已被判离线）。
		ctx.EventManager().EmitEvent(sdk.NewEvent(
			"phonenode.OfflineBacklog",
			sdk.NewAttribute("scanned", fmt.Sprintf("%d", len(due))),
			sdk.NewAttribute("budget", fmt.Sprintf("%d", budget)),
			sdk.NewAttribute("grace_blocks", fmt.Sprintf("%d", params.OfflineGraceBlocks)),
			sdk.NewAttribute("oldest_lag_blocks", fmt.Sprintf("%d", oldestLag)),
		))
		telemetry.IncrCounter(1, "phonenode", "offline_backlog")
	}
	if slashed > 0 {
		telemetry.IncrCounter(float32(slashed), "phonenode", "offline_slash")
	}
}

// collectDueEntries 从心跳索引头部取出最多 budget 条已到期候选。
//
// 索引按 LastProofBlock 升序，头部即最早心跳（最早到期）的节点，顺序取出天然
// 构成 FIFO 到期队列，不需要轮转游标。backlog 表示批次未取完（仍有到期条目
// 待处理）；oldestLag 是下一条待处理条目距到期边界已过多少个区块，用于判断
// 积压严重程度。
//
// 本函数只读，不得修改 KVStore：必须在迭代器关闭之后才可以执行 slash 等写操作。
func (k Keeper) collectDueEntries(ctx sdk.Context, cutoff int64, budget int) (
	due []dueEntry, backlog bool, oldestLag int64,
) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.HeartbeatIndexKeyPrefix)
	it := store.Iterator(nil, types.HeartbeatIndexBound(cutoff))
	defer it.Close()

	due = make([]dueEntry, 0, budget)
	// 预算按扫描条数计，异常键同样计入 —— 否则一群
	// 畸形键就能让循环扫穿上界，把 BeginBlock 拖过出块时间窗（停块且不可自愈）。
	scanned := 0
	for ; it.Valid(); it.Next() {
		if scanned >= budget {
			break
		}
		scanned++
		key := it.Key()
		if len(key) <= 8 {
			continue // 异常键（缺高度段），跳过但计入预算
		}
		due = append(due, dueEntry{
			height: int64(binary.BigEndian.Uint64(key[:8])),
			addr:   string(key[8:]),
		})
	}

	// 循环因预算耗尽而退出且迭代器仍有效 ⇒ 还有到期条目没处理完。
	if it.Valid() && len(it.Key()) > 8 {
		backlog = true
		oldestLag = cutoff - int64(binary.BigEndian.Uint64(it.Key()[:8]))
	}
	return due, backlog, oldestLag
}

// resolveDueEntry 处理一条到期候选，返回是否真的执行了 slash。
//
// 严格 FIFO 前进性：无论走哪条分支，被扫到的条目都必须离开索引，否则下一区块
// 会在同一位置重复扫到同一批僵尸条目，预算永远无法推进。
func (k Keeper) resolveDueEntry(
	ctx sdk.Context, e dueEntry, curHeight int64, params types.Params,
) bool {
	// 1. attestation 缺失或已失效（含被 slash 吊销）→ 已无需检测离线，出队。
	if att, ok := k.GetAttestation(ctx, e.addr); !ok || att.Status != types.AttestationStatusValid {
		k.removeHeartbeatIndex(ctx, e.addr, e.height)
		return false
	}

	// 2. 节点主记录缺失 → 幽灵条目，出队。
	st, err := k.GetNode(ctx, e.addr)
	if err != nil {
		k.removeHeartbeatIndex(ctx, e.addr, e.height)
		return false
	}

	// 3. 索引位与主记录不一致（正常路径不可达：二者由 SetNode 同事务更新，
	//    此处仅防御升级遗留的历史数据）。先摘除旧位置，避免条目原地反复被扫。
	if st.LastProofBlock != e.height {
		k.removeHeartbeatIndex(ctx, e.addr, e.height)
	}

	// 4. 以主记录的真实心跳高度复核宽限，防止索引残影造成误 slash。
	if (curHeight - st.LastProofBlock) <= params.OfflineGraceBlocks {
		k.setHeartbeatIndex(ctx, e.addr, st.LastProofBlock)
		return false
	}

	// 5. 确实超时 → slash。SlashIfBad 内部会把节点移出检测队列（出队语义）。
	if err := k.SlashIfBad(ctx, e.addr, "offline", params.OfflineSlashBps); err != nil {
		ctx.Logger().Error("phonenode: offline slash failed", "node", e.addr, "err", err.Error())
		return false
	}
	return true
}
