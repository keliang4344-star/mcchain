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

// 验证人状态词表（全模块唯一口径）。
//
// ValidateBasic 只放行 "active"/"suspended"，而 slash / 节点资本津贴分发等
// 业务逻辑判定的却是 "jailed"/"inactive"——"suspended" 是一个谁也不认的死值，
// 词表分裂。现统一为下列三态，消息校验与 keeper 共用同一组常量。
const (
	VerifierStatusActive   = "active"   // 在役，可参与验证、可领节点资本津贴
	VerifierStatusInactive = "inactive" // 自愿停役
	VerifierStatusJailed   = "jailed"   // 协议罚没结果，仅治理可解除
)

// IsValidVerifierStatus 判定状态取值是否属于合法词表。
func IsValidVerifierStatus(status string) bool {
	switch status {
	case VerifierStatusActive, VerifierStatusInactive, VerifierStatusJailed:
		return true
	default:
		return false
	}
}

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
	if _, err := sdk.AccAddressFromBech32(msg.NodeId); err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid node_id address (%s)", err)
	}
	if !IsValidVerifierStatus(msg.Status) {
		return sdkerrors.Wrap(ErrInvalidVerifierStatus, "status must be 'active', 'inactive' or 'jailed'")
	}
	return nil
}
