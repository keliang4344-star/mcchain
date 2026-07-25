package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

const TypeMsgOpenDispute = "open_dispute"

var _ sdk.Msg = &MsgOpenDispute{}

func NewMsgOpenDispute(creator, taskID, reason string) *MsgOpenDispute {
	return &MsgOpenDispute{Creator: creator, TaskId: taskID, Reason: reason}
}

func (msg *MsgOpenDispute) Route() string { return RouterKey }
func (msg *MsgOpenDispute) Type() string  { return TypeMsgOpenDispute }
func (msg *MsgOpenDispute) GetSigners() []sdk.AccAddress {
	creator, _ := sdk.AccAddressFromBech32(msg.Creator)
	return []sdk.AccAddress{creator}
}
func (msg *MsgOpenDispute) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(msg))
}
func (msg *MsgOpenDispute) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil { return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator (%s)", err) }
	if msg.TaskId == "" { return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "task_id required") }
	return nil
}
