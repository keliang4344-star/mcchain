package keeper

import (
	"mcchain/x/phonenode/types"
)

var _ types.QueryServer = Keeper{}
