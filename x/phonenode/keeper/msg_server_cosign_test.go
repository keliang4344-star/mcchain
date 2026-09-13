package keeper_test

import (
	"encoding/hex"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	keepertest "mcchain/testutil/keeper"
	"mcchain/x/phonenode/keeper"
	"mcchain/x/phonenode/types"
)

// TestMsgServerCosignFlow 通过链上消息完成 Path C：绑定云端共签方 → 提交共签 → 链上验签存证。
func TestMsgServerCosignFlow(t *testing.T) {
	k, ctx := keepertest.PhonenodeKeeper(t)
	ms := keeper.NewMsgServerImpl(*k)
	goCtx := sdk.WrapSDKContext(ctx)

	nodePriv := secp256k1.GenPrivKey()
	node := sdk.AccAddress(nodePriv.PubKey().Address()).String()

	cloudPriv := secp256k1.GenPrivKey()
	cloudPub := cloudPriv.PubKey().(*secp256k1.PubKey)

	_, err := ms.RegisterCloudSigner(goCtx, types.NewMsgRegisterCloudSigner(node, hex.EncodeToString(cloudPub.Key)))
	require.NoError(t, err)

	cs, ok := k.GetCloudSigner(ctx, node)
	require.True(t, ok)
	require.True(t, cs.Registered)

	payload := make([]byte, 32)
	for i := range payload {
		payload[i] = byte(i + 1)
	}
	sig, err := cloudPriv.Sign(payload)
	require.NoError(t, err)

	_, err = ms.SubmitCosign(goCtx, types.NewMsgSubmitCosign(node, hex.EncodeToString(payload), hex.EncodeToString(sig)))
	require.NoError(t, err)

	att, ok := k.GetCosign(ctx, node)
	require.True(t, ok)
	require.Equal(t, hex.EncodeToString(payload), att.PayloadHash)
}

// TestMsgServerCosignRejectsBadSignature 签名不匹配或未绑定共签方时必须失败。
func TestMsgServerCosignRejectsBadSignature(t *testing.T) {
	k, ctx := keepertest.PhonenodeKeeper(t)
	ms := keeper.NewMsgServerImpl(*k)
	goCtx := sdk.WrapSDKContext(ctx)

	node := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
	cloudPriv := secp256k1.GenPrivKey()
	cloudPub := cloudPriv.PubKey().(*secp256k1.PubKey)

	payload := make([]byte, 32)
	sig, err := cloudPriv.Sign(payload)
	require.NoError(t, err)

	// 未绑定共签方 → 失败
	_, err = ms.SubmitCosign(goCtx, types.NewMsgSubmitCosign(node, hex.EncodeToString(payload), hex.EncodeToString(sig)))
	require.ErrorIs(t, err, types.ErrNoCloudSigner)

	_, err = ms.RegisterCloudSigner(goCtx, types.NewMsgRegisterCloudSigner(node, hex.EncodeToString(cloudPub.Key)))
	require.NoError(t, err)

	// 用另一份载荷的签名 → 验签失败
	other := make([]byte, 32)
	other[0] = 0xAB
	badSig, err := cloudPriv.Sign(other)
	require.NoError(t, err)
	_, err = ms.SubmitCosign(goCtx, types.NewMsgSubmitCosign(node, hex.EncodeToString(payload), hex.EncodeToString(badSig)))
	require.Error(t, err)

	_, ok := k.GetCosign(ctx, node)
	require.False(t, ok, "验签失败不应留下共签证明")
}

// TestCosignMsgValidateBasic 消息层面基础校验。
func TestCosignMsgValidateBasic(t *testing.T) {
	node := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
	pub := hex.EncodeToString(secp256k1.GenPrivKey().PubKey().(*secp256k1.PubKey).Key)

	require.NoError(t, types.NewMsgRegisterCloudSigner(node, pub).ValidateBasic())
	require.Error(t, types.NewMsgRegisterCloudSigner("bad", pub).ValidateBasic())
	require.Error(t, types.NewMsgRegisterCloudSigner(node, "zz").ValidateBasic())

	payload := hex.EncodeToString(make([]byte, 32))
	sig := hex.EncodeToString(make([]byte, 64))
	require.NoError(t, types.NewMsgSubmitCosign(node, payload, sig).ValidateBasic())
	require.Error(t, types.NewMsgSubmitCosign(node, "abcd", sig).ValidateBasic())
	require.Error(t, types.NewMsgSubmitCosign(node, payload, "abcd").ValidateBasic())
}
