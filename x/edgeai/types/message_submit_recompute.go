package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// MsgSubmitRecompute 第二验证层：对争议任务提交链上重算结果指纹。
// 结构体由 proto 生成（tx.pb.go），此处补 sdk.Msg 业务方法。

const TypeMsgSubmitRecompute = "submit_recompute"

var _ sdk.Msg = &MsgSubmitRecompute{}

func NewMsgSubmitRecompute(creator, taskID, recomputeHash string) *MsgSubmitRecompute {
	return &MsgSubmitRecompute{Creator: creator, TaskId: taskID, RecomputeHash: recomputeHash}
}

func (msg *MsgSubmitRecompute) Route() string { return RouterKey }
func (msg *MsgSubmitRecompute) Type() string  { return TypeMsgSubmitRecompute }
func (msg *MsgSubmitRecompute) GetSigners() []sdk.AccAddress {
	creator, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{creator}
}
func (msg *MsgSubmitRecompute) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(msg))
}
func (msg *MsgSubmitRecompute) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Creator); err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator (%s)", err)
	}
	if msg.TaskId == "" {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "task_id is required")
	}
	if msg.RecomputeHash == "" {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "recompute_hash is required")
	}
	return nil
}
