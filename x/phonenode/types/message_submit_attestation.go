package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

const TypeMsgSubmitAttestation = "submit_attestation"

var _ sdk.Msg = &MsgSubmitAttestation{}

func NewMsgSubmitAttestation(creator string, rootHash string, nonce string, deviceIDHash string) *MsgSubmitAttestation {
	return &MsgSubmitAttestation{
		Creator:      creator,
		RootHash:     rootHash,
		Nonce:        nonce,
		DeviceIdHash: deviceIDHash,
	}
}

func (msg *MsgSubmitAttestation) Route() string {
	return RouterKey
}

func (msg *MsgSubmitAttestation) Type() string {
	return TypeMsgSubmitAttestation
}

func (msg *MsgSubmitAttestation) GetSigners() []sdk.AccAddress {
	creator, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{creator}
}

func (msg *MsgSubmitAttestation) GetSignBytes() []byte {
	bz := ModuleCdc.MustMarshalJSON(msg)
	return sdk.MustSortJSON(bz)
}

func (msg *MsgSubmitAttestation) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address (%s)", err)
	}
	return nil
}
