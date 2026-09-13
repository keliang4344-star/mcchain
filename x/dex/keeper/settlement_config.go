package keeper

import (
	"encoding/json"
	"errors"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	"mcchain/x/dex/types"
)

// ---------------------------------------------------------------------------
// 结算权限与熔断配置（JSON 持久化于模块 KVStore，不进 proto Params）
//
// Submit/FinalizeSettlementBatch 必须有 msg.Creator 鉴权，否则
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

// HasSettlementConfig 报告结算配置是否已在链上落盘。
//
// 与 GetSettlementConfig 的区别在于「键是否存在」与「读到的值」：
// Get 在键缺失时返回默认值（隐式路径，适合业务逻辑），
// Has 用于创世路径判断「是否需要写入初始熔断态」—— 若已有记录则不覆盖，
// 否则一次导出/导入会把已开启的结算通道重新熔断
func (k Keeper) HasSettlementConfig(ctx sdk.Context) bool {
	return ctx.KVStore(k.storeKey).Has(types.SettlementConfigKey)
}

// ---------------------------------------------------------------------------
// 结算通道的显式状态与开启路径
// ---------------------------------------------------------------------------
//
// 问题：SetSettlementConfig 在全仓范围内**没有任何调用者**（只有定义与注释），
// x/dex 也没有 MsgUpdateParams 之类的入口，创世同样不写这份配置。于是
// Authority 永远停在默认值——治理模块账户。而治理模块账户**无法签署交易**，
// 所以 SubmitSettlementBatch / FinalizeSettlementBatch 对任何人都是 unauthorized。
//
// 后果不是「不安全」，而是**功能在链上永久不可用**，而且状态是「看起来已启用
// （Halted=false）实际永远拒绝」——这是最难排查的一类故障。
//
// 处置（分两步，都不动 proto）：
//  1. 创世显式把 Halted 置为 true —— 让链上状态说真话：「结算未启用」，
//     而不是「已启用但谁都不许用」；
//  2. 提供经过校验的开启路径 EnableSettlement，供下一次软件升级的
//     UpgradeHandler 调用（见 app.RegisterUpgradeHandlers）。
//
// 开启时必须校验目标地址是**可签名的普通账户**：把一个模块账户（或任何无
// 私钥的地址）设为 Authority，等于把上面这个故障原样复制一遍。

// ErrSettlementAuthorityIsModuleAccount 目标地址是模块账户，无法签名交易。
var ErrSettlementAuthorityIsModuleAccount = errors.New("settlement authority must be a signable account, not a module account")

// ValidateSettlementAuthority 校验结算运营地址可用性。
//
// 拒绝三类地址：
//   - 非法 bech32；
//   - 模块账户地址（无私钥，永远无法签名 → 结算通道必然死锁）；
//   - 空地址（回落到默认值时同样不可用）。
func ValidateSettlementAuthority(addr string) error {
	if addr == "" {
		return errors.New("settlement authority must not be empty")
	}
	if _, err := sdk.AccAddressFromBech32(addr); err != nil {
		return fmt.Errorf("settlement authority is not a valid bech32 account address: %w", err)
	}
	// 模块账户的地址由模块名派生。逐个模块名比对是最直接的判定方式，
	// 不依赖 auth keeper（避免在纯校验函数里引入 keeper 依赖）。
	for _, m := range moduleAccountNames {
		if authtypes.NewModuleAddress(m).String() == addr {
			return fmt.Errorf("%w: %q is the %q module account", ErrSettlementAuthorityIsModuleAccount, addr, m)
		}
	}
	return nil
}

// moduleAccountNames 需要在结算鉴权里显式排除的模块账户名。
//
// 至少必须包含 gov —— 当前默认值就是它，也正是死锁的成因。
var moduleAccountNames = []string{
	govtypes.ModuleName,
	"fee_collector",
	"distribution",
	"bonded_tokens_pool",
	"not_bonded_tokens_pool",
	"mint",
	"staking_security",
	"ecosystem",
	"depin",
	"edgeai",
	"referral",
	"tokenomics",
}

// EnableSettlement 开启结算通道并指定运营地址。
//
// 这是**升级路径**专用（UpgradeHandler / 创世脚本），不是交易可达接口——
// 它不做治理鉴权，鉴权由调用它的升级提案承担（链级治理是唯一授权来源）。
func (k Keeper) EnableSettlement(ctx sdk.Context, operatorAddr string) error {
	if err := ValidateSettlementAuthority(operatorAddr); err != nil {
		return err
	}
	k.SetSettlementConfig(ctx, SettlementConfig{Authority: operatorAddr, Halted: false})
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeSettlementEnabled,
		sdk.NewAttribute(types.AttrSettlementAuthority, operatorAddr),
	))
	return nil
}

// HaltSettlement 熔断结算通道（保留 Authority，仅置 Halted）。
func (k Keeper) HaltSettlement(ctx sdk.Context, reason string) {
	cfg := k.GetSettlementConfig(ctx)
	cfg.Halted = true
	k.SetSettlementConfig(ctx, cfg)
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeSettlementHalted,
		sdk.NewAttribute(types.AttrReason, reason),
	))
}

// IsSettlementUsable 报告结算通道当前是否真的可用。
//
// 「可用」的定义必须同时满足三点，缺一即不可用：
//   - 未熔断；
//   - Authority 是合法 bech32；
//   - Authority 不是模块账户（模块账户无法签名）。
//
// 运维可用它做上线自检：BuildSettlementUnusableReason 会给出不可用的具体原因。
func (k Keeper) IsSettlementUsable(ctx sdk.Context) bool {
	return k.settlementUnusableReason(ctx) == ""
}

func (k Keeper) settlementUnusableReason(ctx sdk.Context) string {
	cfg := k.GetSettlementConfig(ctx)
	if cfg.Halted {
		return "halted by config"
	}
	if err := ValidateSettlementAuthority(cfg.Authority); err != nil {
		return err.Error()
	}
	return ""
}

// SettlementUnusableReason 暴露不可用原因（空串表示可用），供查询与上线自检。
func (k Keeper) SettlementUnusableReason(ctx sdk.Context) string {
	return k.settlementUnusableReason(ctx)
}
