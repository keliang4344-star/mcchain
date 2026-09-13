package tokenomics

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/tokenomics/keeper"
	"mcchain/x/tokenomics/types"
)

// InitGenesis 初始化 tokenomics 模块状态（包级入口，调用 keeper 实现编排）。
// 编排顺序（genesis 顺序铁律：必须在 depin 之前）：
//  1. 一次性 MintCoins(TotalSupplyCap=1e15 umc) 到 tokenomics 模块账户，记录 minted_supply=cap；
//  2. 五池分配（设备激励 55% / 质押安全 15% / 团队 12% / 基金会 13% / 早期开发 5%），
//     强校验 bps 合计=10000、金额合计=cap；
//  3. 团队 3-of-5 多签 vesting（1 年 cliff + 3 年线性）；基金会 T0 解锁 + 2 年线性；
//  4. 从设备激励 55% 内切出 8250 万 MC 推荐预算，单独拨付 referral 生态模块账户；
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
