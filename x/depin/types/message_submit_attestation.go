package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// MsgSubmitAttestation 是预言机向 depin 模块提交设备 attestation 验证结果的消息。
//
// 注意：本文件仅保留 sdk.Msg 业务逻辑方法；消息结构体已由 protoc 生成的
// tx.pb.go 提供（与 proto 定义对齐）。

const TypeMsgSubmitAttestation = "submit_attestation"

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
