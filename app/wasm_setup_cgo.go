//go:build cgo

package app

import (
	"fmt"
	"path/filepath"

	wasmmodule "github.com/CosmWasm/wasmd/x/wasm"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	capabilitykeeper "github.com/cosmos/cosmos-sdk/x/capability/keeper"
	"github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

// setupWasmKeeper 创建 CosmWasm keeper（仅 CGO 构建，wasmvm 真链接）。
// 非 CGO 构建走 wasm_setup_nocgo.go 的空实现，模块完全跳过。
func setupWasmKeeper(app *App, homePath string, appOpts servertypes.AppOptions, keys map[string]*storetypes.KVStoreKey, scopedWasmKeeper capabilitykeeper.ScopedKeeper) error {
	wasmDir := filepath.Join(homePath, "wasm")
	wasmConfig, err := wasmmodule.ReadWasmConfig(appOpts)
	if err != nil {
		return fmt.Errorf("error reading wasm config: %w", err)
	}
	app.WasmKeeper = wasmkeeper.NewKeeper(
		app.appCodec,
		keys[wasmtypes.StoreKey],
		app.AccountKeeper,
		app.BankKeeper,
		app.StakingKeeper,
		keeper.NewQuerier(app.DistrKeeper),
		app.IBCKeeper.ChannelKeeper, // ICS4Wrapper (no ics29 fee middleware on this chain)
		app.IBCKeeper.ChannelKeeper,
		&app.IBCKeeper.PortKeeper,
		scopedWasmKeeper,
		app.TransferKeeper, // ICS20TransferPortSource
		app.MsgServiceRouter(),
		app.GRPCQueryRouter(),
		wasmDir,
		wasmConfig,
		"iterator,staking,stargate,cosmwasm_1_1,cosmwasm_1_2,cosmwasm_1_3,cosmwasm_1_4",
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)
	app.ScopedWasmKeeper = scopedWasmKeeper
	return nil
}

// wasmAppModuleBasic 返回 wasm 的基础模块（CGO 构建时有效）。
//
// wasm 若只注册进 ModuleManager（mm.Modules）而漏了
// ModuleBasics，两侧口径不一致，后果是 CGO 构建下：
//   - `init` 产出的 genesis 里**没有** app_state.wasm 段（DefaultGenesis 走 ModuleBasics），
//     但 mm.InitGenesis 仍会调用 wasm.InitGenesis 并反序列化空数据 → params 全是零值；
//   - `validate-genesis` 完全不校验 wasm 段（BasicManager 里没有它），
//     非法参数会被一路带到 InitChain。
//
// 现在 BasicManager 与 ModuleManager 由同一份条件编译开关决定，不会再漂移。
func wasmAppModuleBasic() module.AppModuleBasic {
	return wasmmodule.AppModuleBasic{}
}

// wasmAppModule 返回 wasm 模块实例（CGO 构建时有效）。
func wasmAppModule(app *App) module.AppModule {
	return wasmmodule.NewAppModule(
		app.appCodec,
		&app.WasmKeeper,
		app.StakingKeeper,
		app.AccountKeeper,
		app.BankKeeper,
		app.MsgServiceRouter(),
		app.GetSubspace(wasmtypes.ModuleName),
	)
}
