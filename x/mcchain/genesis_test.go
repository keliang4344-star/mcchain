package mcchain_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	keepertest "mcchain/testutil/keeper"
	"mcchain/testutil/nullify"
	"mcchain/x/mcchain"
	"mcchain/x/mcchain/types"
)

func TestGenesis(t *testing.T) {
	genesisState := types.GenesisState{
		Params: types.DefaultParams(),

		// this line is used by starport scaffolding # genesis/test/state
	}

	k, ctx := keepertest.McchainKeeper(t)
	mcchain.InitGenesis(ctx, *k, genesisState)
	got := mcchain.ExportGenesis(ctx, *k)
	require.NotNil(t, got)

	nullify.Fill(&genesisState)
	nullify.Fill(got)

	// this line is used by starport scaffolding # genesis/test/assert
}
