package keeper

import (
	"fmt"

	sdkmath "cosmossdk.io/math"
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
	// OVF-1：amount 是导出 API 的 uint64 入参，int64() 在超 MaxInt64 时会回绕为负，
	// 铸造负币 panic。用 NewIntFromUint64 保持任意精度。
	amt := sdk.NewCoins(sdk.NewCoin(denom, sdkmath.NewIntFromUint64(amount)))
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, addr, amt); err != nil {
		return fmt.Errorf("depin: payout reward from pool: %w", err)
	}
	return nil
}
