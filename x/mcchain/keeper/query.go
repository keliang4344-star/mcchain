package keeper

import (
	"mcchain/x/mcchain/types"
)

var _ types.QueryServer = Keeper{}
