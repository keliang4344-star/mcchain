package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"mcchain/x/depin/types"
)

func (k msgServer) RegisterDevice(goCtx context.Context, msg *types.MsgRegisterDevice) (*types.MsgRegisterDeviceResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// 设备地址必须合法（mc 前缀 bech32）
	if _, err := sdk.AccAddressFromBech32(msg.Address); err != nil {
		return nil, sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid device address (%s)", err)
	}

	// 注册设备（仅入网；attestation 通过后才可提交贡献）
	if _, err := k.Keeper.RegisterDevice(ctx, msg.Address, msg.Model, msg.Os); err != nil {
		return nil, err
	}

	return &types.MsgRegisterDeviceResponse{}, nil
}
