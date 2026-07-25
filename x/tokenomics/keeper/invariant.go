package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/tokenomics/types"
)

// MintedSupplyInvariant 校验累计已发行量不超过总量上限（R1：总量固化）。
func MintedSupplyInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		minted := k.GetMintedSupply(ctx)
		cap := sdk.NewIntFromUint64(types.TotalSupplyCap)
		if minted.GT(cap) {
			return sdk.FormatInvariant(
				types.ModuleName,
				"minted-supply",
				fmt.Sprintf("minted supply %s exceeds total supply cap %s", minted.String(), cap.String()),
			), true
		}
		return "", false
	}
}

// PoolSumInvariant 校验「各池 allocated_amount 之和 == minted_supply」（会计口径，Q9 共享知识 #2）。
// 运行期社区/生态会从各自池对外拨付，实时 bank 余额之和会 < minted_supply；
// 故不变量以链上记录的分配记账和 == 已发行为准（恒真）。
func PoolSumInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		minted := k.GetMintedSupply(ctx)
		sum := sdk.ZeroInt()
		for _, a := range k.GetAllocations(ctx) {
			sum = sum.Add(sdk.NewIntFromUint64(a.AllocatedAmount))
		}
		if !sum.Equal(minted) {
			return sdk.FormatInvariant(
				types.ModuleName,
				"pool-sum",
				fmt.Sprintf("sum of allocations %s != minted supply %s", sum.String(), minted.String()),
			), true
		}
		return "", false
	}
}
