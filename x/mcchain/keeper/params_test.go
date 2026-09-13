package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	testkeeper "mcchain/testutil/keeper"
	"mcchain/x/mcchain/types"
)

func TestGetParams(t *testing.T) {
	k, ctx := testkeeper.McchainKeeper(t)
	params := types.DefaultParams()

	k.SetParams(ctx, params)

	require.EqualValues(t, params, k.GetParams(ctx))
}
