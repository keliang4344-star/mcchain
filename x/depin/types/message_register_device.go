package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

const TypeMsgRegisterDevice = "register_device"

var _ sdk.Msg = &MsgRegisterDevice{}

func NewMsgRegisterDevice(creator string, address string, model string, os string) *MsgRegisterDevice {
	return &MsgRegisterDevice{
		Creator: creator,
		Address: address,
		Model:   model,
		Os:      os,
	}
}

func (msg *MsgRegisterDevice) Route() string {
	return RouterKey
}

func (msg *MsgRegisterDevice) Type() string {
	return TypeMsgRegisterDevice
}

func (msg *MsgRegisterDevice) GetSigners() []sdk.AccAddress {
	creator, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{creator}
}

func (msg *MsgRegisterDevice) GetSignBytes() []byte {
	bz := ModuleCdc.MustMarshalJSON(msg)
	return sdk.MustSortJSON(bz)
}

func (msg *MsgRegisterDevice) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address (%s)", err)
	}
	return nil
}
