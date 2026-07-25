package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

const TypeMsgSubmitResult = "submit_result"

var _ sdk.Msg = &MsgSubmitResult{}

func NewMsgSubmitResult(creator, taskID, resultHash, attestationNonce string) *MsgSubmitResult {
	return &MsgSubmitResult{Creator: creator, TaskId: taskID, ResultHash: resultHash, AttestationNonce: attestationNonce}
}

func (msg *MsgSubmitResult) Route() string { return RouterKey }
func (msg *MsgSubmitResult) Type() string  { return TypeMsgSubmitResult }
func (msg *MsgSubmitResult) GetSigners() []sdk.AccAddress {
	creator, _ := sdk.AccAddressFromBech32(msg.Creator)
	return []sdk.AccAddress{creator}
}
func (msg *MsgSubmitResult) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(msg))
}
func (msg *MsgSubmitResult) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil { return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator (%s)", err) }
	if msg.TaskId == "" || msg.ResultHash == "" {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "task_id and result_hash required")
	}
	return nil
}
