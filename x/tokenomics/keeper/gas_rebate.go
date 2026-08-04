package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"mcchain/x/tokenomics/types"
)

// GasRebateRatioBps 定义每笔交易 gas 费回流安全池的比例（基点）。
// 1000 bps = 10%：即每笔交易 gas 费的 10% 从 fee_collector 转入 staking_security。
// 剩余 90% 走标准 Cosmos 分发路径（社区池 + 验证者）。
const GasRebateRatioBps uint32 = 1000

// GasBurnRatioBps defines the fraction of each block's gas fees burned to the
// blackhole (deflation, finalized 2026-08). 700 bps = 7%.
const GasBurnRatioBps uint32 = 700

// RebateGasFeesToSecurity transfers a portion of accumulated gas fees from
// fee_collector to the staking_security pool module account.
//
// 经济逻辑（B3.1 安全池闭环）：
//
//	交易费（gas）的 10% 回流注入安全池，为安全池补充流动性；
//	安全池通过 DripStakingSecurity（滴灌）按周期反哺质押者，
//	形成"交易→安全池→质押者→安全"的正反馈闭环。
//
// 本方法假设调用者（通常为 BeginBlocker）已判断触发时机（如每 N 区块执行一次）。
// 转账失败仅记录事件，不阻塞出块——gas 回流是增值行为，不应成为链安全瓶颈。
func (k Keeper) RebateGasFeesToSecurity(ctx sdk.Context) error {
	feeCollectorAddr := authtypes.NewModuleAddress(authtypes.FeeCollectorName)
	balance := k.bankKeeper.GetBalance(ctx, feeCollectorAddr, types.DefaultDenom)
	if balance.IsZero() {
		return nil
	}

	rebateAmount := balance.Amount.Uint64() * uint64(GasRebateRatioBps) / 10000
	burnAmount := balance.Amount.Uint64() * uint64(GasBurnRatioBps) / 10000
	if rebateAmount == 0 && burnAmount == 0 {
		return nil
	}

	// 7% of gas fees sent to the black hole (deflation, finalized 2026-08).
	//
	// 修正（2026-08）：销毁额按 fee_collector 余额计算，扣款账户必须同为 fee_collector。
	// 旧实现从 tokenomics 模块账户扣款（余额恒为 0），必然失败且 return 阻断了后续
	// 10% 安全池回流；现改为从 fee_collector 直接转入黑洞地址，且失败只记日志不阻断
	// ——与本函数「gas 回流是增值行为，不应成为链安全瓶颈」的既定语义一致。
	if burnAmount > 0 {
		burnCoins := sdk.NewCoins(sdk.NewInt64Coin(types.DefaultDenom, int64(burnAmount)))
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(
			ctx, authtypes.FeeCollectorName, types.BlackHoleAddress(), burnCoins,
		); err != nil {
			k.Logger(ctx).Error("tokenomics: gas burn to black hole failed",
				"burn_amount", burnAmount, "err", err.Error())
		} else {
			ctx.EventManager().EmitEvent(
				sdk.NewEvent("tokenomics.GasBurned",
					sdk.NewAttribute("amount", fmt.Sprintf("%d", burnAmount)),
					sdk.NewAttribute("ratio_bps", fmt.Sprintf("%d", GasBurnRatioBps)),
					sdk.NewAttribute("black_hole", types.BlackHoleAddress().String()),
				),
			)
			k.Logger(ctx).Info("tokenomics: gas fees sent to black hole",
				"amount_umc", burnAmount, "ratio_bps", GasBurnRatioBps)
		}
	}

	// 10% of gas fees rebated to the staking-security pool (B3.1 安全池闭环).
	if rebateAmount > 0 {
		coins := sdk.NewCoins(sdk.NewInt64Coin(types.DefaultDenom, int64(rebateAmount)))
		if err := k.bankKeeper.SendCoinsFromModuleToModule(
			ctx, authtypes.FeeCollectorName, types.StakingSecurityPoolName, coins,
		); err != nil {
			k.Logger(ctx).Error("tokenomics: gas rebate to security pool failed",
				"rebate_amount", rebateAmount, "err", err.Error())
			return fmt.Errorf("gas rebate to security: %w", err)
		}

		ctx.EventManager().EmitEvent(
			sdk.NewEvent("tokenomics.GasRebated",
				sdk.NewAttribute("amount", fmt.Sprintf("%d", rebateAmount)),
				sdk.NewAttribute("ratio_bps", fmt.Sprintf("%d", GasRebateRatioBps)),
				sdk.NewAttribute("destination", types.StakingSecurityPoolName),
			),
		)
		k.Logger(ctx).Info("tokenomics: gas fees rebated to security pool",
			"amount_umc", rebateAmount, "ratio_bps", GasRebateRatioBps)
	}
	return nil
}
