package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

const TypeMsgResolveDispute = "resolve_dispute"

var _ sdk.Msg = &MsgResolveDispute{}

func NewMsgResolveDispute(creator, taskID, resolution string) *MsgResolveDispute {
	return &MsgResolveDispute{Creator: creator, TaskId: taskID, Resolution: resolution}
}

func (msg *MsgResolveDispute) Route() string { return RouterKey }
func (msg *MsgResolveDispute) Type() string  { return TypeMsgResolveDispute }
func (msg *MsgResolveDispute) GetSigners() []sdk.AccAddress {
	creator, _ := sdk.AccAddressFromBech32(msg.Creator)
	return []sdk.AccAddress{creator}
}
func (msg *MsgResolveDispute) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(msg))
}
func (msg *MsgResolveDispute) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator (%s)", err)
	}
	if msg.TaskId == "" {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "task_id required")
	}
	if msg.Resolution != "honest" && msg.Resolution != "cheat" {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "resolution must be 'honest' or 'cheat'")
	}
	return nil
}
