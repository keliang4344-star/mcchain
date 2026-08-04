//go:build cgo

package app

import (
	"fmt"
	"path/filepath"

	wasmmodule "github.com/CosmWasm/wasmd/x/wasm"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	capabilitykeeper "github.com/cosmos/cosmos-sdk/x/capability/keeper"
	"github.com/cosmos/cosmos-sdk/types/module"
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
