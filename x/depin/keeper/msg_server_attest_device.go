package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"mcchain/x/depin/types"
)

func (k msgServer) AttestDevice(goCtx context.Context, msg *types.MsgAttestDevice) (*types.MsgAttestDeviceResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if _, err := sdk.AccAddressFromBech32(msg.Address); err != nil {
		return nil, sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid device address (%s)", err)
	}

	st, err := k.Keeper.GetDevice(ctx, msg.Address)
	if err != nil {
		return nil, err
	}

	// T2 可插拔预言机：attestation 校验交由 DefaultOracle（默认 SoftOracle，
	// 行为与历史一致；生产可 SetOracle(NewTeeOracle(pk)) 切换真实验签）。
	if err := types.DefaultOracle.VerifyDeviceAttestation(ctx, msg.Address, msg.Challenge, msg.Signature); err != nil {
		return nil, err
	}

	st.Attested = true
	if err := k.Keeper.SetDevice(ctx, st); err != nil {
		return nil, err
	}

	return &types.MsgAttestDeviceResponse{}, nil
}
