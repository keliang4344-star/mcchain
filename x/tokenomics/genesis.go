package tokenomics

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/tokenomics/keeper"
	"mcchain/x/tokenomics/types"
)

// InitGenesis 初始化 tokenomics 模块状态（包级入口，调用 keeper 实现编排）。
// 编排顺序（R1/R2，genesis 顺序铁律：必须在 depin 之前）：
//  1. 一次性 MintCoins(cap) 到 tokenomics 模块账户，记录 minted_supply=cap；
//  2. 创建团队多签 vesting 账户并拨付 1.5e14 umc；
//  3. SendCoins 到 community(3.5e14) / ecosystem(5e14) 模块账户；
//  4. 生态池切片 1e14 umc 转给 depin 模块账户（InitialPool，Q7）；
//  5. 写 Allocations / ReleaseSchedule 到 KVStore。
func InitGenesis(ctx sdk.Context, k keeper.Keeper, genState types.GenesisState) {
	if err := k.InitGenesis(ctx, genState); err != nil {
		panic(err)
	}
}

// ExportGenesis 导出 tokenomics 模块状态。
func ExportGenesis(ctx sdk.Context, k keeper.Keeper) *types.GenesisState {
	return k.ExportGenesis(ctx)
}
