package keeper

import (
	"encoding/json"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	"mcchain/x/dex/types"
)

// ---------------------------------------------------------------------------
// 结算权限与熔断配置（JSON 持久化于模块 KVStore，不进 proto Params）
//
// 安全修复（2026-08）：此前 Submit/FinalizeSettlementBatch 对 msg.Creator 零鉴权，
// 任意地址可提交「收款人=自己、金额=DEX 全部余额」的批次并立即清算，掏空 DEX 模块账户。
// 现规定：仅 Authority 地址可提交/清算结算批次；Halted 为熔断开关（治理可置位）。
//
// 默认 Authority = 治理模块账户（gov），即只有经多签治理提案才能动用结算资金，
// 从根上消除「任意地址可 drained」的漏洞。主网前可将 Authority 配置为
// 指定的运营地址（通过 SetSettlementConfig，通常由创世/治理升级设定）。
// ---------------------------------------------------------------------------

// SettlementConfig 结算运营地址与熔断状态。
type SettlementConfig struct {
	// Authority 唯一被允许 Submit/Finalize 结算批次的 bech32 地址。
	Authority string `json:"authority"`
	// Halted 熔断开关：为 true 时拒绝一切结算提交与清算（链上止血手段，A4）。
	Halted bool `json:"halted"`
}

// DefaultSettlementConfig 安全默认：治理模块账户为授权方，未熔断。
func DefaultSettlementConfig() SettlementConfig {
	return SettlementConfig{
		Authority: authtypes.NewModuleAddress(govtypes.ModuleName).String(),
		Halted:    false,
	}
}

// GetSettlementConfig 读取结算配置；未设置或解析失败返回确定性安全默认。
func (k Keeper) GetSettlementConfig(ctx sdk.Context) SettlementConfig {
	bz := ctx.KVStore(k.storeKey).Get(types.SettlementConfigKey)
	if bz == nil {
		return DefaultSettlementConfig()
	}
	var cfg SettlementConfig
	if err := json.Unmarshal(bz, &cfg); err != nil {
		return DefaultSettlementConfig()
	}
	if cfg.Authority == "" {
		cfg.Authority = DefaultSettlementConfig().Authority
	}
	return cfg
}

// SetSettlementConfig 持久化结算配置（治理升级路径调用）。
func (k Keeper) SetSettlementConfig(ctx sdk.Context, cfg SettlementConfig) {
	bz, err := json.Marshal(cfg)
	if err != nil {
		ctx.Logger().Error("dex: marshal settlement config", "err", err)
		return
	}
	ctx.KVStore(k.storeKey).Set(types.SettlementConfigKey, bz)
}
