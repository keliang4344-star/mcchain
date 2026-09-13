package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"mcchain/x/phonenode/types"
)

func (k Keeper) Attestation(goCtx context.Context, req *types.QueryAttestationRequest) (*types.QueryAttestationResponse, error) {
	if req == nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "nil request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	att, _ := k.GetAttestation(ctx, req.Address)
	if att == nil {
		att = &types.Attestation{}
	}
	return &types.QueryAttestationResponse{Attestation: *att}, nil
}
