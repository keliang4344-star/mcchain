package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// MsgSubmitAttestation 是预言机向 depin 模块提交设备 attestation 验证结果的消息。
//
// 注意：本文件为手动编码实现 sdk.Msg（与 proto 定义对齐）。
// 当 protoc 重新生成时，需删除本文件，改用 tx.pb.go 中自动生成的类型。

const TypeMsgSubmitAttestation = "submit_attestation"

// MsgSubmitAttestation 手动编码的 proto 消息类型，必须包含 protobuf tag 供 gRPC 解码器使用。
type MsgSubmitAttestation struct {
	DeviceId         string `protobuf:"bytes,1,opt,name=device_id,json=deviceId,proto3" json:"device_id,omitempty"`
	AttestationProof string `protobuf:"bytes,2,opt,name=attestation_proof,json=attestationProof,proto3" json:"attestation_proof,omitempty"`
	Signature        string `protobuf:"bytes,3,opt,name=signature,proto3" json:"signature,omitempty"`
	OracleAddress    string `protobuf:"bytes,4,opt,name=oracle_address,json=oracleAddress,proto3" json:"oracle_address,omitempty"`
}

func (m *MsgSubmitAttestation) Reset()         { *m = MsgSubmitAttestation{} }
func (m *MsgSubmitAttestation) String() string { return "MsgSubmitAttestation" }
func (*MsgSubmitAttestation) ProtoMessage()    {}

var _ sdk.Msg = &MsgSubmitAttestation{}

func NewMsgSubmitAttestation(deviceID, proof, signature, oracleAddr string) *MsgSubmitAttestation {
	return &MsgSubmitAttestation{
		DeviceId:         deviceID,
		AttestationProof: proof,
		Signature:        signature,
		OracleAddress:    oracleAddr,
	}
}

func (msg *MsgSubmitAttestation) Route() string {
	return RouterKey
}

func (msg *MsgSubmitAttestation) Type() string {
	return TypeMsgSubmitAttestation
}

func (msg *MsgSubmitAttestation) GetSigners() []sdk.AccAddress {
	oracle, err := sdk.AccAddressFromBech32(msg.OracleAddress)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{oracle}
}

func (msg *MsgSubmitAttestation) GetSignBytes() []byte {
	bz := ModuleCdc.MustMarshalJSON(msg)
	return sdk.MustSortJSON(bz)
}

func (msg *MsgSubmitAttestation) ValidateBasic() error {
	if msg.DeviceId == "" {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "device_id is required")
	}
	if msg.AttestationProof == "" {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "attestation_proof is required")
	}
	if msg.Signature == "" {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "signature is required")
	}
	if msg.OracleAddress == "" {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "oracle_address is required")
	}
	_, err := sdk.AccAddressFromBech32(msg.OracleAddress)
	if err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid oracle address (%s)", err)
	}
	return nil
}

// MsgSubmitAttestationResponse 验证结果消息的响应。
type MsgSubmitAttestationResponse struct {
	Passed bool   `protobuf:"varint,1,opt,name=passed,proto3" json:"passed,omitempty"`
	Reason string `protobuf:"bytes,2,opt,name=reason,proto3" json:"reason,omitempty"`
}

func (m *MsgSubmitAttestationResponse) Reset()         { *m = MsgSubmitAttestationResponse{} }
func (m *MsgSubmitAttestationResponse) String() string { return "MsgSubmitAttestationResponse" }
func (*MsgSubmitAttestationResponse) ProtoMessage()    {}
