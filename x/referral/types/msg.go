package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// Message type assertions — struct definitions live in tx.pb.go (proto-generated).
var (
	_ sdk.Msg = &MsgCreateReferral{}
	_ sdk.Msg = &MsgClaimReferralReward{}
)

func (m *MsgCreateReferral) Route() string { return RouterKey }
func (m *MsgCreateReferral) Type() string  { return "create_referral" }
func (m *MsgCreateReferral) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Inviter)
	return []sdk.AccAddress{addr}
}
func (m *MsgCreateReferral) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}
func (m *MsgCreateReferral) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Inviter); err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid inviter: %s", err)
	}
	if _, err := sdk.AccAddressFromBech32(m.Invitee); err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid invitee: %s", err)
	}
	return nil
}

func (m *MsgClaimReferralReward) Route() string { return RouterKey }
func (m *MsgClaimReferralReward) Type() string  { return "claim_referral_reward" }
func (m *MsgClaimReferralReward) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Claimer)
	return []sdk.AccAddress{addr}
}
func (m *MsgClaimReferralReward) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}
func (m *MsgClaimReferralReward) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Claimer); err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid claimer: %s", err)
	}
	return nil
}
