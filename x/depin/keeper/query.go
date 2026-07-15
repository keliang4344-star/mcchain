package keeper

import (
	"mcchain/x/depin/types"
)

var _ types.QueryServer = Keeper{}
