package keeper

import (
	"fmt"

	"github.com/cometbft/cometbft/libs/log"
	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"

	"mcchain/x/referral/types"
)

// Keeper is the referral module's state keeper.
// It manages referral records, pending rewards, and the ecosystem payout pool.
type Keeper struct {
	cdc             codec.BinaryCodec
	storeKey        storetypes.StoreKey
	paramstore      paramtypes.Subspace
	bankKeeper      types.BankKeeper
	phonenodeKeeper types.PhonenodeKeeper
}

// NewKeeper creates a new referral module Keeper instance.
func NewKeeper(
	cdc codec.BinaryCodec,
	storeKey storetypes.StoreKey,
	ps paramtypes.Subspace,
	bankKeeper types.BankKeeper,
	phonenodeKeeper types.PhonenodeKeeper,
) *Keeper {
	if !ps.HasKeyTable() {
		ps = ps.WithKeyTable(types.ParamKeyTable())
	}

	return &Keeper{
		cdc:             cdc,
		storeKey:        storeKey,
		paramstore:      ps,
		bankKeeper:      bankKeeper,
		phonenodeKeeper: phonenodeKeeper,
	}
}

func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

func (k Keeper) GetParams(ctx sdk.Context) (params types.Params) {
	k.paramstore.GetParamSet(ctx, &params)
	return params
}

func (k Keeper) SetParams(ctx sdk.Context, params types.Params) {
	k.paramstore.SetParamSet(ctx, &params)
}
