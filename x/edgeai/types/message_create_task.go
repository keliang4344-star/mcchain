package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

const TypeMsgCreateTask = "create_task"

var _ sdk.Msg = &MsgCreateTask{}

func NewMsgCreateTask(creator, description string, reward uint64) *MsgCreateTask {
	return &MsgCreateTask{Creator: creator, Description: description, Reward: reward}
}

func (msg *MsgCreateTask) Route() string            { return RouterKey }
func (msg *MsgCreateTask) Type() string             { return TypeMsgCreateTask }
func (msg *MsgCreateTask) GetSigners() []sdk.AccAddress {
	creator, _ := sdk.AccAddressFromBech32(msg.Creator)
	return []sdk.AccAddress{creator}
}
func (msg *MsgCreateTask) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(msg))
}
func (msg *MsgCreateTask) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil { return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator (%s)", err) }
	if msg.Description == "" { return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "description required") }
	return nil
}
