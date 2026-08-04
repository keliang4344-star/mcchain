package keeper

import (
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/mcchain/types"
)

// ---------------------------------------------------------------------------
// 渐进治理移交（Progressive Governance Handover）
//
// 目标：把链的治理权从创始团队（多签）按「可预期、可审计、不可突袭」的方式移交给
// 新的治理主体（社区 DAO / 新多签）。核心是两阶段 + 时间锁：
//
//	InitiateHandover  -> 登记新治理地址，并把生效高度锁定在 当前高度 + TimelockBlocks
//	CompleteHandover  -> 只有越过生效高度后才允许执行，执行一次即终态（Executed）
//
// 时间锁窗口内，任何观察者都能读到「谁将接管、何时接管」，从而有充足时间退出或反对。
//
// 配置不进 proto Params（无代码生成工具），改为 JSON 持久化于模块 KVStore，
// 与 x/phonenode 的 NodeAllowanceConfig 保持同一套模式。
// ---------------------------------------------------------------------------

// GovernanceHandoverConfig 渐进治理移交配置（JSON 持久化于模块 KVStore）。
type GovernanceHandoverConfig struct {
	// Enabled 是否开启移交流程；关闭时任何发起/执行都不生效。
	Enabled bool `json:"enabled"`
	// CurrentGovernor 当前治理主体地址（bech32）。链上消息 MsgInitiateHandover /
	// MsgCompleteHandover 的签名者必须等于该地址；为空表示尚未配置治理主体，
	// 此时不接受任何链上移交消息（只能通过创世/升级由 SetGovernanceHandoverConfig 设定）。
	CurrentGovernor string `json:"current_governor"`
	// TimelockBlocks 时间锁长度（区块数），发起到可执行之间的强制冷静期。
	TimelockBlocks uint64 `json:"timelock_blocks"`
	// RequiredSigners 执行移交所需的多签阈值（与团队多签配合使用）。
	RequiredSigners uint32 `json:"required_signers"`
	// NewGovernor 待接管的新治理地址（bech32）；为空表示当前无待决移交。
	NewGovernor string `json:"new_governor"`
	// ActivationHeight 时间锁到期高度，达到该高度后方可 CompleteHandover。
	ActivationHeight int64 `json:"activation_height"`
	// Executed 移交是否已完成（终态，不可回退）。
	Executed bool `json:"executed"`
}

// DefaultGovernanceHandoverConfig 默认配置：默认关闭，时间锁 43200 块（约 12 小时，
// 按 1s 出块），多签阈值 3（与团队 3-of-5 多签一致）。
var DefaultGovernanceHandoverConfig = GovernanceHandoverConfig{
	Enabled:          false,
	TimelockBlocks:   43200,
	RequiredSigners:  3,
	NewGovernor:      "",
	ActivationHeight: 0,
	Executed:         false,
}

// GetGovernanceHandoverConfig 读取移交配置；未设置或解析失败返回确定性默认值。
func (k Keeper) GetGovernanceHandoverConfig(ctx sdk.Context) GovernanceHandoverConfig {
	bz := ctx.KVStore(k.storeKey).Get(types.GovernanceHandoverConfigKey)
	if bz == nil {
		return DefaultGovernanceHandoverConfig
	}
	var cfg GovernanceHandoverConfig
	if err := json.Unmarshal(bz, &cfg); err != nil {
		return DefaultGovernanceHandoverConfig
	}
	if cfg.TimelockBlocks == 0 {
		cfg.TimelockBlocks = DefaultGovernanceHandoverConfig.TimelockBlocks
	}
	if cfg.RequiredSigners == 0 {
		cfg.RequiredSigners = DefaultGovernanceHandoverConfig.RequiredSigners
	}
	return cfg
}

// SetGovernanceHandoverConfig 持久化移交配置。
func (k Keeper) SetGovernanceHandoverConfig(ctx sdk.Context, cfg GovernanceHandoverConfig) {
	bz, err := json.Marshal(cfg)
	if err != nil {
		ctx.Logger().Error("mcchain: marshal governance handover config", "err", err)
		return
	}
	ctx.KVStore(k.storeKey).Set(types.GovernanceHandoverConfigKey, bz)
}

// InitiateHandover 发起治理移交：登记新治理地址并启动时间锁。
// 生效高度 = 当前高度 + TimelockBlocks，在此之前 CompleteHandover 一律失败。
func (k Keeper) InitiateHandover(ctx sdk.Context, newGovAddr string) error {
	cfg := k.GetGovernanceHandoverConfig(ctx)
	if !cfg.Enabled {
		return fmt.Errorf("mcchain: governance handover is disabled")
	}
	if cfg.Executed {
		return fmt.Errorf("mcchain: governance handover already executed")
	}
	if _, err := sdk.AccAddressFromBech32(newGovAddr); err != nil {
		return fmt.Errorf("mcchain: invalid new governor address %q: %w", newGovAddr, err)
	}

	cfg.NewGovernor = newGovAddr
	cfg.ActivationHeight = ctx.BlockHeight() + int64(cfg.TimelockBlocks)
	k.SetGovernanceHandoverConfig(ctx, cfg)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"mcchain.GovernanceHandoverInitiated",
		sdk.NewAttribute("new_governor", cfg.NewGovernor),
		sdk.NewAttribute("activation_height", fmt.Sprintf("%d", cfg.ActivationHeight)),
	))
	return nil
}

// CompleteHandover 完成治理移交：仅在时间锁到期后允许，执行一次即终态。
// 未启用或已执行时为幂等空操作。
func (k Keeper) CompleteHandover(ctx sdk.Context) error {
	cfg := k.GetGovernanceHandoverConfig(ctx)
	if !cfg.Enabled || cfg.Executed {
		return nil
	}
	if cfg.NewGovernor == "" {
		return fmt.Errorf("mcchain: no pending governance handover")
	}
	if ctx.BlockHeight() < cfg.ActivationHeight {
		return fmt.Errorf("mcchain: governance handover timelock not elapsed: current height %d < activation height %d",
			ctx.BlockHeight(), cfg.ActivationHeight)
	}

	cfg.Executed = true
	// 移交生效：新治理主体正式接管，后续链上治理消息以其地址为准。
	cfg.CurrentGovernor = cfg.NewGovernor
	k.SetGovernanceHandoverConfig(ctx, cfg)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"mcchain.GovernanceHandoverCompleted",
		sdk.NewAttribute("new_governor", cfg.NewGovernor),
	))
	return nil
}

// AssertGovernanceAuthority 校验链上治理消息的签名者是否为当前治理主体。
// 未配置治理主体时一律拒绝，避免任意地址发起移交。
func (k Keeper) AssertGovernanceAuthority(ctx sdk.Context, authority string) error {
	if _, err := sdk.AccAddressFromBech32(authority); err != nil {
		return fmt.Errorf("mcchain: invalid authority address %q: %w", authority, err)
	}
	cfg := k.GetGovernanceHandoverConfig(ctx)
	if cfg.CurrentGovernor == "" {
		return fmt.Errorf("mcchain: current governor is not configured; on-chain handover messages are rejected")
	}
	if cfg.CurrentGovernor != authority {
		return fmt.Errorf("mcchain: unauthorized: expected governance authority %s, got %s", cfg.CurrentGovernor, authority)
	}
	return nil
}

// IsHandoverPending 是否存在「已发起但尚未执行」的移交。
func (k Keeper) IsHandoverPending(ctx sdk.Context) bool {
	cfg := k.GetGovernanceHandoverConfig(ctx)
	return cfg.Enabled && cfg.NewGovernor != "" && !cfg.Executed
}

// IsHandoverComplete 移交是否已完成。
func (k Keeper) IsHandoverComplete(ctx sdk.Context) bool {
	return k.GetGovernanceHandoverConfig(ctx).Executed
}
