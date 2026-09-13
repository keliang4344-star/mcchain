package types_test

import (
	"strings"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"mcchain/x/referral/types"
)

func randAddr() string {
	return sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
}

// TestCreateReferralSignerIsInvitee 是签名者必须为被邀请人的核心回归。
//
// 绑定授权只能来自被推荐人本人。若签名方是推荐人，任何人都能扫描新注册地址
// 并抢先把它挂到自己名下——推荐关系永久不可改，被抢即无解。
func TestCreateReferralSignerIsInvitee(t *testing.T) {
	inviter, invitee := randAddr(), randAddr()
	msg := &types.MsgCreateReferral{Inviter: inviter, Invitee: invitee, InviteCode: "MC-2026"}

	signers := msg.GetSigners()
	require.Len(t, signers, 1)
	require.Equal(t, invitee, signers[0].String(), "签名方必须是被推荐人")
	require.NotEqual(t, inviter, signers[0].String())
}

func TestCreateReferralValidateBasic(t *testing.T) {
	inviter, invitee := randAddr(), randAddr()
	ok := func(code string) *types.MsgCreateReferral {
		return &types.MsgCreateReferral{Inviter: inviter, Invitee: invitee, InviteCode: code}
	}

	require.NoError(t, ok("MC-2026").ValidateBasic())
	require.NoError(t, ok("abcXYZ_09-").ValidateBasic())

	// 邀请码必须校验：空串 / 超长 / 控制字符都不能写进永久推荐记录。
	require.Error(t, ok("").ValidateBasic(), "空邀请码必须拒绝")
	require.Error(t, ok(strings.Repeat("a", types.MaxInviteCodeLength+1)).ValidateBasic(), "超长邀请码必须拒绝")
	require.Error(t, ok("bad code").ValidateBasic(), "空格不在允许字符集")
	require.Error(t, ok("邀请码").ValidateBasic(), "非 ASCII 不在允许字符集")
	require.Error(t, ok("drop\x00table").ValidateBasic(), "控制字符必须拒绝")

	// 自推荐在 ValidateBasic 阶段即拦截，不必进入状态机。
	self := &types.MsgCreateReferral{Inviter: inviter, Invitee: inviter, InviteCode: "MC"}
	require.Error(t, self.ValidateBasic())

	bad := &types.MsgCreateReferral{Inviter: "not-bech32", Invitee: invitee, InviteCode: "MC"}
	require.Error(t, bad.ValidateBasic())
}
