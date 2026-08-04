package keeper_test

import (
	"encoding/hex"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	keepertest "mcchain/testutil/keeper"
	"mcchain/x/phonenode/types"
)

// TestPathCCosign 验证手机-云端共签（Path C）增强层：
// 绑定云端共签方 → 用其私钥对载荷哈希签名 → 链上验签通过并存证。
func TestPathCCosign(t *testing.T) {
	k, ctx := keepertest.PhonenodeKeeper(t)
	nodePriv := secp256k1.GenPrivKey()
	node := sdk.AccAddress(nodePriv.PubKey().Address()).String()

	// 云端共签方密钥
	cloudPriv := secp256k1.GenPrivKey()
	cloudPub := cloudPriv.PubKey().(*secp256k1.PubKey)

	// 1) 绑定云端共签方（公钥以 33 字节 hex 存储）
	require.NoError(t, k.RegisterCloudSigner(ctx, node, hex.EncodeToString(cloudPub.Key)))

	// 2) 云端对载荷哈希签名
	payload := make([]byte, 32)
	for i := range payload {
		payload[i] = byte(i)
	}
	sig, err := cloudPriv.Sign(payload)
	require.NoError(t, err)

	// 3) 提交共签 → 链上验签通过
	require.NoError(t, k.SubmitCosign(ctx, node, hex.EncodeToString(payload), hex.EncodeToString(sig)))
	att, ok := k.GetCosign(ctx, node)
	require.True(t, ok)
	require.Equal(t, hex.EncodeToString(payload), att.PayloadHash)

	// 4) 篡改载荷 → 验签失败
	bad := make([]byte, 32)
	bad[0] = 0xFF
	badSig, _ := cloudPriv.Sign(bad)
	require.Error(t, k.SubmitCosign(ctx, node, hex.EncodeToString(payload), hex.EncodeToString(badSig)),
		"用错误载荷的签名不应通过")

	// 5) 未绑定云端共签方的节点提交共签 → 报错
	other := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
	require.ErrorIs(t, k.SubmitCosign(ctx, other, hex.EncodeToString(payload), hex.EncodeToString(sig)),
		types.ErrNoCloudSigner)
}
