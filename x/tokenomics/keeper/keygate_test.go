package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	keepertest "mcchain/testutil/keeper"
	"mcchain/x/tokenomics/types"
)

// 回归：主网创世密钥闸门不得依赖 chain-id 命名。
//
// 回归守则：闸门不得依赖 `strings.Contains(chainID, "mainnet")`。这个判定
// 两头都不安全 —— chain-id 含 mainnet 但还没配真密钥时起链直接 panic（链起不来）；
// chain-id 换个不含 mainnet 的名字时闸门静默失效，1.75 亿 MC 落到任何读过源码
// 的人都能还原的占位私钥上，而日志里连一句告警都没有。
//
// 现在改由显式环境变量 MC_REQUIRE_REAL_GENESIS_KEYS=1 声明「本次是主网创世」，
// 与 chain-id 怎么命名无关；chain-id 嗅探保留为第二道保险。
func TestInitGenesisRequiresRealKeysWhenEnvGateSet(t *testing.T) {
	if types.FoundationOverridesConfigured() {
		t.Skip("本机构已配置真实基金会公钥，占位密钥闸门不会触发")
	}

	// 未声明主网 ⇒ 允许占位（devnet / CI 不受影响）。
	k, ctx, _, _ := keepertest.TokenomicsKeeper(t)
	require.NoError(t, k.InitGenesis(ctx, *types.DefaultGenesis()),
		"未设置 MC_REQUIRE_REAL_GENESIS_KEYS 时 devnet 必须能正常起链")

	// 显式声明主网 ⇒ 占位密钥必须硬失败。
	t.Setenv(types.RequireRealGenesisKeysEnv, "1")
	k2, ctx2, _, _ := keepertest.TokenomicsKeeper(t)
	err := k2.InitGenesis(ctx2, *types.DefaultGenesis())
	require.Error(t, err, "主网创世使用源码可推导的占位密钥必须硬失败，而不是打一行日志继续跑")
	require.Contains(t, err.Error(), "PLACEHOLDER")
}

// TestIsMainnetChainID 第二道保险的判定口径（只作兜底，不是唯一判据）。
func TestIsMainnetChainID(t *testing.T) {
	require.True(t, types.IsMainnetChainID("mcchain-mainnet-1"))
	require.True(t, types.IsMainnetChainID("MC-MainNet-2"))
	require.False(t, types.IsMainnetChainID("mcchain-testnet-1"))
	// 反例：主网若改名不含 mainnet，chain-id 兜底会失效 —— 这正是必须与
	// MC_REQUIRE_REAL_GENESIS_KEYS 并存的理由。
	require.False(t, types.IsMainnetChainID("mc-1"))
}
