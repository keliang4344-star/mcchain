package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"mcchain/x/depin/types"
)

// PayoutReward 从 DePIN 模块账户（生态池，B1 由 tokenomics 在 InitGenesis 一次性拨付）
// 向 addr 拨付 amount 个 reward denom（umc）。
//
// 经济约束：本函数不铸造、不突破 B1 总量 cap（1B MC），仅从池内划拨；
// 池余额不足时返回错误，由调用方决策（通常是拒绝拨付、保留结果）。
//
// 这是 B3.1 R4 的拨付入口：x/edgeai 在任务结果通过争议期后调用本函数，
// 实现「移动端执行 AI 任务 → 贡献即挖矿」的经济闭环。edgeai 不直接持有 Minter。
func (k Keeper) PayoutReward(ctx sdk.Context, addr sdk.AccAddress, amount uint64) error {
	if amount == 0 {
		return nil
	}
	denom := k.GetParams(ctx).RewardDenom
	amt := sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(int64(amount))))
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, addr, amt); err != nil {
		return fmt.Errorf("depin: payout reward from pool: %w", err)
	}
	return nil
}
