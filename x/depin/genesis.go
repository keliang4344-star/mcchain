package depin

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"mcchain/x/depin/keeper"
	"mcchain/x/depin/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func InitGenesis(ctx sdk.Context, k keeper.Keeper, genState types.GenesisState) {
	// this line is used by starport scaffolding # genesis/module/init
	k.SetParams(ctx, genState.Params)

	// Q7 / R1 铸币铁律：depin 绝不自铸，模块账户不持有 Minter 权限。
	// InitialPool（默认 types.DefaultInitialPool = 4.675e14 umc = 4.675 亿 MC）
	// 由 tokenomics 在 InitGenesis 从设备激励池切片 DepinInitialPoolSlice
	// (5.5e14 umc) 扣除 ReferralEcosystemBudget (8.25e13 umc) 后，经模块间
	// 转账注入 depin 模块账户（见 x/tokenomics/keeper/genesis.go）。
	// 顺序铁律：tokenomics.InitGenesis 必须排在 depin 之前（app.go SetOrderInitGenesis）。
}

// ExportGenesis returns the module's exported genesis
func ExportGenesis(ctx sdk.Context, k keeper.Keeper) *types.GenesisState {
	genesis := types.DefaultGenesis()
	genesis.Params = k.GetParams(ctx)

	// this line is used by starport scaffolding # genesis/module/export

	return genesis
}
