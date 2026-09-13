//go:build !cgo

package app

import (
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	capabilitykeeper "github.com/cosmos/cosmos-sdk/x/capability/keeper"
)

// setupWasmKeeper 非 CGO 构建的空实现：wasmvm 无法链接，wasm 模块完全跳过
// （ModuleManager 中对应的 order 条目会被 Manager 的存在性检查安全跳过）。
func setupWasmKeeper(_ *App, _ string, _ servertypes.AppOptions, _ map[string]*storetypes.KVStoreKey, _ capabilitykeeper.ScopedKeeper) error {
	return nil
}

// wasmAppModule 非 CGO 构建返回 nil（不注册 wasm 模块）。
func wasmAppModule(_ *App) module.AppModule {
	return nil
}

// wasmAppModuleBasic 非 CGO 构建返回 nil：wasm 既不在 ModuleManager，也不在
// ModuleBasics —— 两侧口径一致。
func wasmAppModuleBasic() module.AppModuleBasic {
	return nil
}
