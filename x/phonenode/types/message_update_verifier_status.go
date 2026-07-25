package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// MsgUpdateVerifierStatus 更新节点验证者状态的消息。
//
// 注意：本文件为手动编码实现 sdk.Msg（与 proto 定义对齐）。
// 当 protoc 重新生成时，需删除本文件，改用 tx.pb.go 中自动生成的类型。

const TypeMsgUpdateVerifierStatus = "update_verifier_status"

// MsgUpdateVerifierStatus 手动编码的 proto 消息类型。
type MsgUpdateVerifierStatus struct {
	Creator string `protobuf:"bytes,1,opt,name=creator,proto3" json:"creator,omitempty"`
	NodeId  string `protobuf:"bytes,2,opt,name=node_id,json=nodeId,proto3" json:"node_id,omitempty"`
	Status  string `protobuf:"bytes,3,opt,name=status,proto3" json:"status,omitempty"`
}

func (m *MsgUpdateVerifierStatus) Reset()         { *m = MsgUpdateVerifierStatus{} }
func (m *MsgUpdateVerifierStatus) String() string { return "MsgUpdateVerifierStatus" }
func (*MsgUpdateVerifierStatus) ProtoMessage()    {}

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

// MsgUpdateVerifierStatusResponse 更新验证者状态消息的响应。
type MsgUpdateVerifierStatusResponse struct {
}

func (m *MsgUpdateVerifierStatusResponse) Reset()         { *m = MsgUpdateVerifierStatusResponse{} }
func (m *MsgUpdateVerifierStatusResponse) String() string { return "MsgUpdateVerifierStatusResponse" }
func (*MsgUpdateVerifierStatusResponse) ProtoMessage()    {}
