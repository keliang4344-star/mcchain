package phonenode_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	keepertest "mcchain/testutil/keeper"
	"mcchain/testutil/nullify"
	"mcchain/x/phonenode"
	"mcchain/x/phonenode/types"
)

func TestGenesis(t *testing.T) {
	genesisState := types.GenesisState{
		Params: types.DefaultParams(),

		// this line is used by starport scaffolding # genesis/test/state
	}

	k, ctx := keepertest.PhonenodeKeeper(t)
	phonenode.InitGenesis(ctx, *k, genesisState)
	got := phonenode.ExportGenesis(ctx, *k)
	require.NotNil(t, got)

	nullify.Fill(&genesisState)
	nullify.Fill(got)

	// this line is used by starport scaffolding # genesis/test/assert
}
