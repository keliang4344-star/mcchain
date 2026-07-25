package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

const TypeMsgSubmitStateProof = "submit_state_proof"

var _ sdk.Msg = &MsgSubmitStateProof{}

func NewMsgSubmitStateProof(creator string, root string, leaf string, index string, proof string) *MsgSubmitStateProof {
	return &MsgSubmitStateProof{
		Creator: creator,
		Root:    root,
		Leaf:    leaf,
		Index:   index,
		Proof:   proof,
	}
}

func (msg *MsgSubmitStateProof) Route() string {
	return RouterKey
}

func (msg *MsgSubmitStateProof) Type() string {
	return TypeMsgSubmitStateProof
}

func (msg *MsgSubmitStateProof) GetSigners() []sdk.AccAddress {
	creator, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{creator}
}

func (msg *MsgSubmitStateProof) GetSignBytes() []byte {
	bz := ModuleCdc.MustMarshalJSON(msg)
	return sdk.MustSortJSON(bz)
}

func (msg *MsgSubmitStateProof) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address (%s)", err)
	}
	return nil
}
