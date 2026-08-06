package keeper

import (
	"encoding/json"
	"fmt"

	"github.com/cosmos/cosmos-sdk/store/prefix"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/edgeai/types"
)

// Reputation 节点声誉记录（白皮书行 497）。
// 每个执行节点维护 ReputationScore（初始 100，范围 0-100）。
// 任务通过 → +1（上限 100）；任务拒绝/作弊 → -10（下限 0）。
// 声誉 < 30 的节点限制接单优先级。
//
// 当前以非 protobuf Go 结构体持久化（JSON 编码），待 proto 重新生成后可迁移。
type Reputation struct {
	NodeAddr                string `json:"node_addr"`
	Score                   uint32 `json:"score"`
	LastContributionBlock   int64  `json:"last_contribution_block"`
	ConsecutiveMissedBlocks int64  `json:"consecutive_missed_blocks"`
	// LastDecayBlock 记录最近一次衰减发生的高度（REP-1）。
	// 缺失该字段时，一旦跨过 ReputationDecayBlocks 阈值，节点会在此后的
	// 每一个区块都被 -1，满分 100 约 100 个区块（十分钟量级）即被清零，
	// 与白皮书「连续 N 区块无贡献 → 声誉 -1」的语义严重不符。
	// 有了该字段，衰减基准 = max(最近贡献高度, 最近衰减高度)，
	// 每个衰减周期至多扣 1 分。旧记录反序列化后为 0，自动回退到按贡献高度计。
	LastDecayBlock int64 `json:"last_decay_block"`
}

// reputation key prefix for KV store
var reputationKeyPrefix = []byte("reputation:")

func reputationKey(nodeAddr string) []byte {
	return append(reputationKeyPrefix, []byte(nodeAddr)...)
}

// SetReputation 持久化节点声誉记录（JSON 编码）。
func (k Keeper) SetReputation(ctx sdk.Context, r *Reputation) error {
	bz, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("edgeai: marshal reputation: %w", err)
	}
	ctx.KVStore(k.storeKey).Set(reputationKey(r.NodeAddr), bz)
	return nil
}

// GetReputation 读取节点声誉；不存在时返回初始声誉（100）。
func (k Keeper) GetReputation(ctx sdk.Context, nodeAddr string) (*Reputation, error) {
	bz := ctx.KVStore(k.storeKey).Get(reputationKey(nodeAddr))
	if bz == nil {
		return &Reputation{
			NodeAddr:                nodeAddr,
			Score:                   types.DefaultReputationScore,
			LastContributionBlock:   0,
			ConsecutiveMissedBlocks: 0,
		}, nil
	}
	var r Reputation
	if err := json.Unmarshal(bz, &r); err != nil {
		return nil, fmt.Errorf("edgeai: unmarshal reputation: %w", err)
	}
	return &r, nil
}

// AllReputations 返回全部声誉记录。
func (k Keeper) AllReputations(ctx sdk.Context) []*Reputation {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), reputationKeyPrefix)
	it := store.Iterator(nil, nil)
	defer it.Close()
	out := make([]*Reputation, 0)
	for ; it.Valid(); it.Next() {
		var r Reputation
		if err := json.Unmarshal(it.Value(), &r); err != nil {
			continue
		}
		out = append(out, &r)
	}
	return out
}

// IncrementReputation 增加节点声誉（上限 MaxReputationScore）。
func (k Keeper) IncrementReputation(ctx sdk.Context, nodeAddr string, delta uint32) {
	r, err := k.GetReputation(ctx, nodeAddr)
	if err != nil {
		return
	}
	newScore := r.Score + delta
	if newScore > types.MaxReputationScore {
		newScore = types.MaxReputationScore
	}
	r.Score = newScore
	r.LastContributionBlock = ctx.BlockHeight()
	r.ConsecutiveMissedBlocks = 0
	if err := k.SetReputation(ctx, r); err != nil {
		// ERR-1：写入失败不得静默吞掉（序列化异常属确定性故障，记录以便排障）。
		ctx.Logger().Error("edgeai: SetReputation failed", "err", err.Error())
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("edgeai.ReputationIncreased",
			sdk.NewAttribute("node_addr", nodeAddr),
			sdk.NewAttribute("new_score", fmt.Sprintf("%d", newScore)),
		),
	)
}

// DecrementReputation 减少节点声誉（下限 MinReputationScore）。
func (k Keeper) DecrementReputation(ctx sdk.Context, nodeAddr string, delta uint32) {
	r, err := k.GetReputation(ctx, nodeAddr)
	if err != nil {
		return
	}
	if delta >= r.Score {
		r.Score = types.MinReputationScore
	} else {
		r.Score -= delta
	}
	r.LastContributionBlock = ctx.BlockHeight()
	r.ConsecutiveMissedBlocks = 0
	if err := k.SetReputation(ctx, r); err != nil {
		// ERR-1：写入失败不得静默吞掉（序列化异常属确定性故障，记录以便排障）。
		ctx.Logger().Error("edgeai: SetReputation failed", "err", err.Error())
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("edgeai.ReputationDecreased",
			sdk.NewAttribute("node_addr", nodeAddr),
			sdk.NewAttribute("new_score", fmt.Sprintf("%d", r.Score)),
		),
	)
}

// IsLowPriority 判断节点是否因声誉过低而限制接单优先级。
func (k Keeper) IsLowPriority(ctx sdk.Context, nodeAddr string) bool {
	r, err := k.GetReputation(ctx, nodeAddr)
	if err != nil {
		return false
	}
	return r.Score < types.ReputationLowPriorityThreshold
}

// DecayReputation 在 BeginBlock 中调用，对连续无贡献的节点执行声誉衰减。
// 每满 ReputationDecayBlocks (1000) 个区块无贡献 → -1。
//
// SCALE-1：不再调用 AllReputations 把全量声誉记录读进内存，
// 改为按持久化游标做有界轮转，每区块至多检查 MaxReputationScanPerBlock 条。
// 游标属链上状态，各节点批次一致，不引入非确定性；扫到末尾自动回到开头。
//
// REP-1：衰减基准取 max(最近贡献高度, 最近衰减高度)，
// 保证一个衰减周期最多扣 1 分，而不是跨过阈值后每个区块都扣。
func (k Keeper) DecayReputation(ctx sdk.Context) {
	currentHeight := ctx.BlockHeight()
	entries := boundedScan(
		ctx.KVStore(k.storeKey),
		reputationKeyPrefix,
		reputationCursorKey,
		types.MaxReputationScanPerBlock,
	)

	for _, e := range entries {
		var r Reputation
		if err := json.Unmarshal(e.Value, &r); err != nil {
			continue
		}
		if r.LastContributionBlock <= 0 {
			// 新节点尚未贡献过，跳过衰减
			continue
		}
		if r.Score <= types.MinReputationScore {
			continue // 已见底，无需重复写入
		}

		base := r.LastContributionBlock
		if r.LastDecayBlock > base {
			base = r.LastDecayBlock
		}
		if currentHeight-base < types.ReputationDecayBlocks {
			continue
		}

		r.Score--
		r.LastDecayBlock = currentHeight
		r.ConsecutiveMissedBlocks = currentHeight - r.LastContributionBlock
		if err := k.SetReputation(ctx, &r); err != nil {
			// ERR-1：写入失败不得静默吞掉（序列化异常属确定性故障，记录以便排障）。
			ctx.Logger().Error("edgeai: SetReputation failed", "err", err.Error())
			continue
		}

		ctx.EventManager().EmitEvent(
			sdk.NewEvent("edgeai.ReputationDecayed",
				sdk.NewAttribute("node_addr", r.NodeAddr),
				sdk.NewAttribute("new_score", fmt.Sprintf("%d", r.Score)),
				sdk.NewAttribute("missed_blocks", fmt.Sprintf("%d", r.ConsecutiveMissedBlocks)),
			),
		)
		telemetry.IncrCounter(1, "edgeai", "reputation_decay_count")
	}
}
