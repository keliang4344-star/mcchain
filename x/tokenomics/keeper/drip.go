package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"mcchain/x/tokenomics/types"
)

// DripIntervalBlocks 定义安全池滴灌间隔（区块数）。
// 每 100 块（约 10 分钟）执行一次滴灌。
const DripIntervalBlocks int64 = 100

// DripRatioBps 定义每次滴灌从安全池释放的比例（基点）。
// 500 bps = 5%：每次滴灌释放当前安全池余额的 5%。
// 随余额递减自然收敛，避免安全池被一次性抽干。
const DripRatioBps uint32 = 500

// DripStakingSecurity transfers a portion of the staking_security pool balance
// into the fee_collector module account, where the distribution module's
// AllocateTokens (in BeginBlock) automatically distributes it to validators
// and delegators proportional to their staked amount.
//
// 经济逻辑（安全池滴灌闭环）：
//
//	安全池中的代币周期性释放到 fee_collector，由 distribution 模块按质押比例
//	自动分配给验证者与委托者；
//	形成"质押→安全→反哺质押"的正反馈；
//	滴灌比例固定为 5%（DripRatioBps），随池余额递减自然收敛；
//	配合 gas 回流（RebateGasFeesToSecurity），实现安全池的动态平衡。
//
// 本方法假设调用者（BeginBlocker）已判断触发时机（每 DripIntervalBlocks 执行一次）。
func (k Keeper) DripStakingSecurity(ctx sdk.Context) error {
	poolAddr := types.StakingSecurityPoolAddress()
	balance := k.bankKeeper.GetBalance(ctx, poolAddr, types.DefaultDenom)
	if balance.IsZero() {
		return nil
	}

	dripAmount := balance.Amount.Uint64() * uint64(DripRatioBps) / 10000
	if dripAmount == 0 {
		return nil
	}

	// 安全池 → fee_collector：由标准 distribution 模块的 AllocateTokens
	// 按质押比例自动分配给验证者与委托者，无需额外引入 keeper 依赖。
	coins := sdk.NewCoins(sdk.NewInt64Coin(types.DefaultDenom, int64(dripAmount)))

	if err := k.bankKeeper.SendCoinsFromModuleToModule(
		ctx, types.StakingSecurityPoolName, authtypes.FeeCollectorName, coins,
	); err != nil {
		k.Logger(ctx).Error("tokenomics: security pool drip failed",
			"drip_amount", dripAmount, "err", err.Error())
		return fmt.Errorf("security pool drip: %w", err)
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("tokenomics.SecurityDripped",
			sdk.NewAttribute("amount", fmt.Sprintf("%d", dripAmount)),
			sdk.NewAttribute("ratio_bps", fmt.Sprintf("%d", DripRatioBps)),
			sdk.NewAttribute("source", types.StakingSecurityPoolName),
			sdk.NewAttribute("destination", "fee_collector"),
		),
	)
	k.Logger(ctx).Info("tokenomics: security pool dripped to fee_collector for distribution",
		"amount_umc", dripAmount, "ratio_bps", DripRatioBps)
	return nil
}
