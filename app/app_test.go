package app

import (
	"testing"

	dbm "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/libs/log"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	depinmoduletypes "mcchain/x/depin/types"
	tokenomicsmoduletypes "mcchain/x/tokenomics/types"
)

// TestTokenomicsMaccPerms 验证 app.go 模块账户权限划分（Q7 硬约束 + 五池模型）：
//   - tokenomics 持有 Minter（唯一发行入口）；
//   - depin 不再持有 Minter（仅 {Burner, Staking}），同时托管设备激励池资金；
//   - 质押安全 / 基金会 / 早期开发 为独立模块账户（五池模型）。
//
// 采用 package app（与 app.go 同包）以直接访问未导出的 maccPerms 映射。
func TestTokenomicsMaccPerms(t *testing.T) {
	// tokenomics 必须注册并持有 Minter。
	tkPerms, ok := maccPerms[tokenomicsmoduletypes.ModuleName]
	require.True(t, ok, "tokenomics must be registered in maccPerms")
	require.Contains(t, tkPerms, authtypes.Minter, "tokenomics must hold Minter")

	// depin 必须保留注册（托管设备激励池），但不得持有 Minter（仅 Burner/Staking）。
	depinPerms, ok := maccPerms[depinmoduletypes.ModuleName]
	require.True(t, ok, "depin must remain registered in maccPerms")
	require.NotContains(t, depinPerms, authtypes.Minter, "depin must NOT hold Minter after Q7")
	require.ElementsMatch(t, []string{authtypes.Burner, authtypes.Staking}, depinPerms)

	// 质押安全 / 基金会 / 早期开发 为独立模块账户（五池模型）。
	_, ok = maccPerms[tokenomicsmoduletypes.StakingSecurityPoolName]
	require.True(t, ok, "staking_security pool must be a module account")
	_, ok = maccPerms[tokenomicsmoduletypes.FoundationPoolName]
	require.True(t, ok, "foundation pool must be a module account")
	_, ok = maccPerms[tokenomicsmoduletypes.EarlyDevPoolName]
	require.True(t, ok, "early_dev pool must be a module account")
}

// TestTokenomicsInitGenesisOrder 验证 genesis 顺序铁律（R1 硬约束）：
// tokenomics.InitGenesis 必须排在 depin.InitGenesis 之前，确保生态切片拨付与
// cap 记账由 tokenomics 完成，depin 仅消费已到账资金。
//
// 通过构造整链 app 并检查 module.Manager.OrderInitGenesis 顺序做端到端验证。
func TestTokenomicsInitGenesisOrder(t *testing.T) {
	db := dbm.NewMemDB()
	encCfg := MakeEncodingConfig()
	app := New(
		log.NewNopLogger(),
		db,
		nil,            // traceStore
		false,          // loadLatest
		map[int64]bool{}, // skipUpgradeHeights
		"",             // homePath
		0,              // invCheckPeriod
		encCfg,
		simtestutil.EmptyAppOptions{},
	)
	require.NotNil(t, app, "app must construct without panic")

	order := app.mm.OrderInitGenesis
	tkIdx := indexOf(order, tokenomicsmoduletypes.ModuleName)
	depinIdx := indexOf(order, depinmoduletypes.ModuleName)
	require.GreaterOrEqual(t, tkIdx, 0, "tokenomics must appear in init genesis order")
	require.GreaterOrEqual(t, depinIdx, 0, "depin must appear in init genesis order")
	require.Less(t, tkIdx, depinIdx, "tokenomics must run InitGenesis before depin")
}

// indexOf 返回 s 中元素 s 的索引，未找到返回 -1。
func indexOf(slice []string, s string) int {
	for i, v := range slice {
		if v == s {
			return i
		}
	}
	return -1
}
