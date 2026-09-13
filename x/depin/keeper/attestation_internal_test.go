package keeper

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestOracleInWhitelist 单测辅助函数。
func TestOracleInWhitelist(t *testing.T) {
	list := []string{"a", "b", "c"}
	require.True(t, oracleInWhitelist("b", list))
	require.False(t, oracleInWhitelist("z", list))
	require.False(t, oracleInWhitelist("a", nil))
}

// TestIsProductionChainID 单测辅助函数。
func TestIsProductionChainID(t *testing.T) {
	require.True(t, isProductionChainID("mcchain-mainnet-1"))
	require.True(t, isProductionChainID("")) // 空 chain-id 视为生产（fail-closed 更安全的默认）
	require.False(t, isProductionChainID("mcchain-testnet-1"))
	require.False(t, isProductionChainID("mcchain-dev-1"))
	require.False(t, isProductionChainID("local"))
	require.False(t, isProductionChainID("sim"))
}
