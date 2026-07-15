package keeper

import (
	"fmt"

	"github.com/cometbft/cometbft/libs/log"
	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"

	"mcchain/x/edgeai/types"
)

type Keeper struct {
	cdc            codec.BinaryCodec
	storeKey       storetypes.StoreKey
	memKey         storetypes.StoreKey
	paramstore     paramtypes.Subspace
	phonenodeKeeper types.PhonenodeKeeper
	bankKeeper      types.BankKeeper
	payoutKeeper    types.PayoutKeeper
}

func NewKeeper(
	cdc codec.BinaryCodec,
	storeKey, memKey storetypes.StoreKey,
	ps paramtypes.Subspace,
	phonenodeKeeper types.PhonenodeKeeper,
	bankKeeper types.BankKeeper,
	payoutKeeper types.PayoutKeeper,
) *Keeper {
	if !ps.HasKeyTable() {
		ps = ps.WithKeyTable(types.ParamKeyTable())
	}
	return &Keeper{
		cdc:            cdc,
		storeKey:       storeKey,
		memKey:         memKey,
		paramstore:     ps,
		phonenodeKeeper: phonenodeKeeper,
		bankKeeper:      bankKeeper,
		payoutKeeper:    payoutKeeper,
	}
}

func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}
