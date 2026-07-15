package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

const TypeMsgRegisterNode = "register_node"

var _ sdk.Msg = &MsgRegisterNode{}

func NewMsgRegisterNode(creator string, address string, model string, os string, role string) *MsgRegisterNode {
	return &MsgRegisterNode{
		Creator: creator,
		Address: address,
		Model:   model,
		Os:      os,
		Role:    role,
	}
}

func (msg *MsgRegisterNode) Route() string {
	return RouterKey
}

func (msg *MsgRegisterNode) Type() string {
	return TypeMsgRegisterNode
}

func (msg *MsgRegisterNode) GetSigners() []sdk.AccAddress {
	creator, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{creator}
}

func (msg *MsgRegisterNode) GetSignBytes() []byte {
	bz := ModuleCdc.MustMarshalJSON(msg)
	return sdk.MustSortJSON(bz)
}

func (msg *MsgRegisterNode) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address (%s)", err)
	}
	return nil
}
