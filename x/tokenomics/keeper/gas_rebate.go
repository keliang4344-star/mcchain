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
	if rebateAmount == 0 {
		return nil
	}

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
	return nil
}
