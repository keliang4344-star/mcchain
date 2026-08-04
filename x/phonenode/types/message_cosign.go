package types

import (
	"encoding/hex"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// Path C 手机-云端共签消息的 sdk.Msg 实现。
// 结构体由 proto 生成（tx.pb.go），此处补业务方法。

const (
	TypeMsgRegisterCloudSigner = "register_cloud_signer"
	TypeMsgSubmitCosign        = "submit_cosign"
)

var (
	_ sdk.Msg = &MsgRegisterCloudSigner{}
	_ sdk.Msg = &MsgSubmitCosign{}
)

func NewMsgRegisterCloudSigner(creator, cloudPubKey string) *MsgRegisterCloudSigner {
	return &MsgRegisterCloudSigner{Creator: creator, CloudPubKey: cloudPubKey}
}

func (msg *MsgRegisterCloudSigner) Route() string { return RouterKey }
func (msg *MsgRegisterCloudSigner) Type() string  { return TypeMsgRegisterCloudSigner }
func (msg *MsgRegisterCloudSigner) GetSigners() []sdk.AccAddress {
	creator, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{creator}
}
func (msg *MsgRegisterCloudSigner) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(msg))
}
func (msg *MsgRegisterCloudSigner) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Creator); err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address (%s)", err)
	}
	pub, err := hex.DecodeString(msg.CloudPubKey)
	if err != nil || len(pub) != 33 {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "cloud_pub_key must be a 33-byte hex encoded secp256k1 public key")
	}
	return nil
}

func NewMsgSubmitCosign(creator, payloadHash, cloudSignature string) *MsgSubmitCosign {
	return &MsgSubmitCosign{Creator: creator, PayloadHash: payloadHash, CloudSignature: cloudSignature}
}

func (msg *MsgSubmitCosign) Route() string { return RouterKey }
func (msg *MsgSubmitCosign) Type() string  { return TypeMsgSubmitCosign }
func (msg *MsgSubmitCosign) GetSigners() []sdk.AccAddress {
	creator, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{creator}
}
func (msg *MsgSubmitCosign) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(msg))
}
func (msg *MsgSubmitCosign) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Creator); err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address (%s)", err)
	}
	payload, err := hex.DecodeString(msg.PayloadHash)
	if err != nil || len(payload) != 32 {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "payload_hash must be a 32-byte hex string")
	}
	sig, err := hex.DecodeString(msg.CloudSignature)
	if err != nil || len(sig) != 64 {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "cloud_signature must be a 64-byte hex string")
	}
	return nil
}
