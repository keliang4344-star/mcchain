package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"mcchain/x/phonenode/types"
)

func (k Keeper) Slashes(goCtx context.Context, req *types.QuerySlashesRequest) (*types.QuerySlashesResponse, error) {
	if req == nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "nil request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	recs := k.GetSlashes(ctx, req.Address)
	if recs == nil {
		recs = []types.SlashRecord{}
	}
	return &types.QuerySlashesResponse{Records: recs}, nil
}
