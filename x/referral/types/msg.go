package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// MaxInviteCodeLength 邀请码最大长度（字符）。
const MaxInviteCodeLength = 32

// validInviteCodeChar 邀请码允许的字符集：字母、数字、'-'、'_'。
// 限制字符集是为了让邀请码可安全地出现在链下 URL、二维码与日志里，
// 也避免不可见字符构造出「看起来一样」的两个码。
func validInviteCodeChar(c rune) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	case c == '-' || c == '_':
		return true
	}
	return false
}

// Message type assertions — struct definitions live in tx.pb.go (proto-generated).
var (
	_ sdk.Msg = &MsgCreateReferral{}
	_ sdk.Msg = &MsgClaimReferralReward{}
)

func (m *MsgCreateReferral) Route() string { return RouterKey }
func (m *MsgCreateReferral) Type() string  { return "create_referral" }

// GetSigners 返回**被推荐人**。
//
// 朴素实现以推荐人（inviter）为唯一签名方，等于任何人都能把一个自己不认识的
// 已注册设备单方面挂到自己名下。推荐关系一旦写入即永久有效、不可更改，
// 于是「扫描新注册地址 → 抢先绑定」就能无成本吞掉整棵下线树，
// 真正的推荐人反而绑不上。
//
// 绑定的授权来源只能是被推荐人本人：由被推荐人携带邀请码上链声明
// 「我由 X 推荐」，与白皮书「邀请码绑定」的语义一致。
func (m *MsgCreateReferral) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Invitee)
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
	if m.Inviter == m.Invitee {
		return ErrSelfReferral
	}
	// 邀请码必须校验：空串、超长串、控制字符都不能写进永久推荐记录。
	if m.InviteCode == "" {
		return sdkerrors.Wrap(ErrInvalidInviteCode, "invite code must not be empty")
	}
	if len([]rune(m.InviteCode)) > MaxInviteCodeLength {
		return sdkerrors.Wrap(ErrInvalidInviteCode,
			fmt.Sprintf("invite code exceeds %d characters", MaxInviteCodeLength))
	}
	for _, c := range m.InviteCode {
		if !validInviteCodeChar(c) {
			return sdkerrors.Wrap(ErrInvalidInviteCode,
				"invite code may only contain letters, digits, '-' and '_'")
		}
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
