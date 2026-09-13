package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// 渐进治理移交消息的 sdk.Msg 实现。
// 结构体本身由 proto 生成（tx.pb.go），此处只补业务方法。

const (
	TypeMsgInitiateHandover = "initiate_handover"
	TypeMsgCompleteHandover = "complete_handover"
)

var (
	_ sdk.Msg = &MsgInitiateHandover{}
	_ sdk.Msg = &MsgCompleteHandover{}
)

func NewMsgInitiateHandover(authority, newGovernor string) *MsgInitiateHandover {
	return &MsgInitiateHandover{Authority: authority, NewGovernor: newGovernor}
}

func (m *MsgInitiateHandover) Route() string { return RouterKey }
func (m *MsgInitiateHandover) Type() string  { return TypeMsgInitiateHandover }
func (m *MsgInitiateHandover) GetSigners() []sdk.AccAddress {
	addr, err := sdk.AccAddressFromBech32(m.Authority)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{addr}
}
func (m *MsgInitiateHandover) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}
func (m *MsgInitiateHandover) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid authority address (%s)", err)
	}
	if _, err := sdk.AccAddressFromBech32(m.NewGovernor); err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid new_governor address (%s)", err)
	}
	if m.Authority == m.NewGovernor {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "new_governor must differ from the current authority")
	}
	return nil
}

func NewMsgCompleteHandover(authority string) *MsgCompleteHandover {
	return &MsgCompleteHandover{Authority: authority}
}

func (m *MsgCompleteHandover) Route() string { return RouterKey }
func (m *MsgCompleteHandover) Type() string  { return TypeMsgCompleteHandover }
func (m *MsgCompleteHandover) GetSigners() []sdk.AccAddress {
	addr, err := sdk.AccAddressFromBech32(m.Authority)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{addr}
}
func (m *MsgCompleteHandover) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}
func (m *MsgCompleteHandover) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid authority address (%s)", err)
	}
	return nil
}
