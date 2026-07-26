package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// MsgUpdateVerifierStatus 更新节点验证者状态的消息。
//
// 注意：本文件仅保留 sdk.Msg 业务逻辑方法；消息结构体已由 protoc 生成的
// tx.pb.go 提供（与 proto 定义对齐）。

const TypeMsgUpdateVerifierStatus = "update_verifier_status"

var _ sdk.Msg = &MsgUpdateVerifierStatus{}

func NewMsgUpdateVerifierStatus(creator, nodeID, status string) *MsgUpdateVerifierStatus {
	return &MsgUpdateVerifierStatus{
		Creator: creator,
		NodeId:  nodeID,
		Status:  status,
	}
}

func (msg *MsgUpdateVerifierStatus) Route() string {
	return RouterKey
}

func (msg *MsgUpdateVerifierStatus) Type() string {
	return TypeMsgUpdateVerifierStatus
}

func (msg *MsgUpdateVerifierStatus) GetSigners() []sdk.AccAddress {
	creator, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{creator}
}

func (msg *MsgUpdateVerifierStatus) GetSignBytes() []byte {
	bz := ModuleCdc.MustMarshalJSON(msg)
	return sdk.MustSortJSON(bz)
}

func (msg *MsgUpdateVerifierStatus) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address (%s)", err)
	}
	if msg.NodeId == "" {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "node_id is required")
	}
	if msg.Status != "active" && msg.Status != "suspended" {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "status must be 'active' or 'suspended'")
	}
	return nil
}
