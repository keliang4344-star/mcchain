package keeper

import (
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	tokenomicstypes "mcchain/x/tokenomics/types"
	"mcchain/x/phonenode/types"
)

// ---------------------------------------------------------------------------
// 节点资本津贴（建设溢价）
//
// 白皮书定义：节点资本津贴 = 设备激励池 30 MC / 节点 / 日，作为「跑设备的建设溢价」，
// 区别于 DePIN 任务奖励，旨在奖励真实运行移动节点的建设性贡献，抑制纯持币空转。
//
// 实现要点（2026-08 落地，真实可运行）：
//   - 资金来源于设备激励池（depin 模块账户），不新铸、不破 1B 上限；
//   - 以 UTC「日序号」(BlockTime/86400) 为粒度，每节点每日至多发放一次（幂等）；
//   - 由 BeginBlock 在「跨天」时统一遍历活跃节点分发一次，避免每区块重复发放；
//   - 配置（Enabled / PerDay）存于模块 KVStore，可按治理调整（经后续 Msg）。
// ---------------------------------------------------------------------------

const (
	// DefaultNodeCapitalAllowancePerDay 节点建设溢价日津贴默认值（umc）。
	// 1 MC = 1e6 umc，故 30 MC = 30_000_000 umc。
	DefaultNodeCapitalAllowancePerDay uint64 = 30_000_000
	// DefaultNodeCapitalAllowanceEnabled 是否启用节点资本津贴分发。
	DefaultNodeCapitalAllowanceEnabled = true
)

// NodeAllowanceConfig 节点资本津贴配置（JSON 持久化于模块 KVStore）。
type NodeAllowanceConfig struct {
	Enabled bool   `json:"enabled"`
	PerDay  uint64 `json:"per_day_umc"` // umc / 节点 / 日
}

// GetNodeAllowanceConfig 读取配置；未设置或解析失败返回确定性默认值。
func (k Keeper) GetNodeAllowanceConfig(ctx sdk.Context) NodeAllowanceConfig {
	bz := ctx.KVStore(k.storeKey).Get(types.NodeAllowanceConfigKey)
	if bz == nil {
		return NodeAllowanceConfig{Enabled: DefaultNodeCapitalAllowanceEnabled, PerDay: DefaultNodeCapitalAllowancePerDay}
	}
	var cfg NodeAllowanceConfig
	if err := json.Unmarshal(bz, &cfg); err != nil {
		return NodeAllowanceConfig{Enabled: DefaultNodeCapitalAllowanceEnabled, PerDay: DefaultNodeCapitalAllowancePerDay}
	}
	if cfg.PerDay == 0 {
		cfg.PerDay = DefaultNodeCapitalAllowancePerDay
	}
	return cfg
}

// SetNodeAllowanceConfig 持久化节点资本津贴配置。
func (k Keeper) SetNodeAllowanceConfig(ctx sdk.Context, cfg NodeAllowanceConfig) {
	bz, err := json.Marshal(cfg)
	if err != nil {
		ctx.Logger().Error("phonenode: marshal node allowance config", "err", err)
		return
	}
	ctx.KVStore(k.storeKey).Set(types.NodeAllowanceConfigKey, bz)
}

// dayIndex 由区块时间推算 UTC「日序号」。
func dayIndex(ctx sdk.Context) uint64 {
	return uint64(ctx.BlockTime().Unix() / 86400)
}

func (k Keeper) getLastAllowanceDay(ctx sdk.Context, addr string) (uint64, bool) {
	bz := ctx.KVStore(k.storeKey).Get(types.NodeAllowanceDayKey(addr))
	if bz == nil {
		return 0, false
	}
	var d uint64
	if err := json.Unmarshal(bz, &d); err != nil {
		return 0, false
	}
	return d, true
}

func (k Keeper) setLastAllowanceDay(ctx sdk.Context, addr string, d uint64) {
	bz, _ := json.Marshal(d)
	ctx.KVStore(k.storeKey).Set(types.NodeAllowanceDayKey(addr), bz)
}

func (k Keeper) getGlobalLastAllowanceDay(ctx sdk.Context) (uint64, bool) {
	bz := ctx.KVStore(k.storeKey).Get(types.GlobalLastAllowanceDayKey)
	if bz == nil {
		return 0, false
	}
	var d uint64
	if err := json.Unmarshal(bz, &d); err != nil {
		return 0, false
	}
	return d, true
}

func (k Keeper) setGlobalLastAllowanceDay(ctx sdk.Context, d uint64) {
	bz, _ := json.Marshal(d)
	ctx.KVStore(k.storeKey).Set(types.GlobalLastAllowanceDayKey, bz)
}

// PayNodeCapitalAllowance 向单个节点运营者拨付当日建设溢价。
// 幂等：同「日序号」已发则跳过。资金来自设备激励池（depin 模块账户）。
func (k Keeper) PayNodeCapitalAllowance(ctx sdk.Context, operator sdk.AccAddress, di uint64) error {
	cfg := k.GetNodeAllowanceConfig(ctx)
	if !cfg.Enabled || cfg.PerDay == 0 {
		return nil
	}
	if last, ok := k.getLastAllowanceDay(ctx, operator.String()); ok && last == di {
		return nil // 当日已发，幂等跳过
	}
	amt := sdk.NewCoins(sdk.NewCoin(tokenomicstypes.DefaultDenom, sdk.NewIntFromUint64(cfg.PerDay)))
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, tokenomicstypes.DepinModuleName, operator, amt); err != nil {
		return fmt.Errorf("phonenode: pay node capital allowance to %s: %w", operator.String(), err)
	}
	k.setLastAllowanceDay(ctx, operator.String(), di)
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"phonenode.NodeCapitalAllowance",
		sdk.NewAttribute("node", operator.String()),
		sdk.NewAttribute("amount", amt.String()),
		sdk.NewAttribute("day_index", fmt.Sprintf("%d", di)),
	))
	return nil
}

// DistributeNodeCapitalAllowances 每日（按 UTC 天）向所有活跃移动节点分发建设溢价。
// 从 BeginBlock 调用：仅在「日序号」跨天时执行一次；节点级幂等为第二道保险。
func (k Keeper) DistributeNodeCapitalAllowances(ctx sdk.Context) error {
	cfg := k.GetNodeAllowanceConfig(ctx)
	if !cfg.Enabled || cfg.PerDay == 0 {
		return nil
	}
	di := dayIndex(ctx)
	if lastRun, ok := k.getGlobalLastAllowanceDay(ctx); ok && lastRun == di {
		return nil
	}

	for _, node := range k.AllNodes(ctx) {
		if !node.Registered {
			continue
		}
		// 已 jail / inactive 的节点不发放（仅奖励真实在线建设贡献）。
		if node.VerifierStatus == "jailed" || node.VerifierStatus == "inactive" {
			continue
		}
		opAddr, err := sdk.AccAddressFromBech32(node.Address)
		if err != nil {
			continue
		}
		if err := k.PayNodeCapitalAllowance(ctx, opAddr, di); err != nil {
			ctx.Logger().Error("phonenode: distribute node capital allowance failed",
				"node", node.Address, "err", err)
			continue
		}
	}
	k.setGlobalLastAllowanceDay(ctx, di)
	return nil
}
