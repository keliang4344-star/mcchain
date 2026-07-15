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

	// Q7：depin 不再自铸。InitialPool(1e14 umc) 由 tokenomics 在 InitGenesis
	// 经生态池模块账户拨付到 depin 模块账户（见 x/tokenomics）。tokenomics 的
	// InitGenesis 必须排在 depin 之前（app.go SetOrderInitGenesis）。
}

// ExportGenesis returns the module's exported genesis
func ExportGenesis(ctx sdk.Context, k keeper.Keeper) *types.GenesisState {
	genesis := types.DefaultGenesis()
	genesis.Params = k.GetParams(ctx)

	// this line is used by starport scaffolding # genesis/module/export

	return genesis
}
