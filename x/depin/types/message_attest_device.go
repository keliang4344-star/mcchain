package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

const TypeMsgAttestDevice = "attest_device"

var _ sdk.Msg = &MsgAttestDevice{}

func NewMsgAttestDevice(creator string, address string, challenge string, signature string) *MsgAttestDevice {
	return &MsgAttestDevice{
		Creator:   creator,
		Address:   address,
		Challenge: challenge,
		Signature: signature,
	}
}

func (msg *MsgAttestDevice) Route() string {
	return RouterKey
}

func (msg *MsgAttestDevice) Type() string {
	return TypeMsgAttestDevice
}

func (msg *MsgAttestDevice) GetSigners() []sdk.AccAddress {
	creator, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{creator}
}

func (msg *MsgAttestDevice) GetSignBytes() []byte {
	bz := ModuleCdc.MustMarshalJSON(msg)
	return sdk.MustSortJSON(bz)
}

func (msg *MsgAttestDevice) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address (%s)", err)
	}
	return nil
}
