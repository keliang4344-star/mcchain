package keeper_test

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	keepertest "mcchain/testutil/keeper"
	"mcchain/x/phonenode/keeper"
	"mcchain/x/phonenode/types"
)

// ---------------------------------------------------------------------------
// attestation 验证分级（分级信任）+ 打破循环信任链
//
// 冷启动口径必须保持不变：任何注册设备都能用「设备自签」完成认证并开始挖矿
// （IsAttested）。真正需要真实设备背书的经济动作走 IsVerifiedAttested，
// 其证据来自外部预言机回写的 TierOracle。
// ---------------------------------------------------------------------------

// TestAttestationTierDefaultIsSelf 自签认证完成后仍是基础层。
func TestAttestationTierDefaultIsSelf(t *testing.T) {
	k, ctx := keepertest.PhonenodeKeeper(t)
	priv := secp256k1.GenPrivKey()
	addr := sdk.AccAddress(priv.PubKey().Address()).String()
	_, err := k.RegisterNode(ctx, addr, "pixel8", "android", "contributor")
	require.NoError(t, err)

	devPriv := secp256k1.GenPrivKey()
	devPubHex := hex.EncodeToString(devPriv.PubKey().(*secp256k1.PubKey).Key)
	deviceHash := strings.Repeat("ab", 32)
	nonce := "nonce-tier-0001"
	sig, err := devPriv.Sign([]byte(deviceHash + "|" + nonce))
	require.NoError(t, err)

	require.NoError(t, k.SubmitAttestation(ctx, addr, "root-a", nonce, deviceHash, devPubHex, hex.EncodeToString(sig)))
	require.True(t, k.IsAttested(ctx, addr), "自签认证必须足以开始参与（冷启动门槛不变）")
	require.False(t, k.IsVerifiedAttested(ctx, addr), "未经预言机背书不得进入增强层")
	require.EqualValues(t, types.AttestationTierSelf, k.GetAttestationTier(ctx, addr))
}

// TestMarkOracleVerifiedPromotesTier 预言机验签成功后必须把层级提升为 TierOracle。
func TestMarkOracleVerifiedPromotesTier(t *testing.T) {
	k, ctx := keepertest.PhonenodeKeeper(t)
	priv := secp256k1.GenPrivKey()
	addr := sdk.AccAddress(priv.PubKey().Address()).String()
	_, err := k.RegisterNode(ctx, addr, "pixel8", "android", "contributor")
	require.NoError(t, err)

	// 未认证时不得凭空提升（否则外部调用可造壳）。
	require.Error(t, k.MarkOracleVerified(ctx, addr, "challenge-x"),
		"没有有效自签认证时不得提升层级")

	expiry := ctx.BlockTime().Unix() + types.DefaultParams().AttestationValidity
	k.SetAttestation(ctx, addr, types.NewValidAttestation("root-a", "nonce-a", strings.Repeat("ab", 32), expiry))
	require.True(t, k.IsAttested(ctx, addr))

	require.NoError(t, k.MarkOracleVerified(ctx, addr, "challenge-x"))
	require.True(t, k.IsVerifiedAttested(ctx, addr))
	require.EqualValues(t, types.AttestationTierOracle, k.GetAttestationTier(ctx, addr))
}

// TestSubmitAttestationResetsTierToSelf 新认证周期必须把层级打回基础层 ——
// 预言机背书只对「它验过的那一次 challenge」有效，不可跨周期继承。
func TestSubmitAttestationResetsTierToSelf(t *testing.T) {
	k, ctx := keepertest.PhonenodeKeeper(t)
	priv := secp256k1.GenPrivKey()
	addr := sdk.AccAddress(priv.PubKey().Address()).String()
	_, err := k.RegisterNode(ctx, addr, "pixel8", "android", "contributor")
	require.NoError(t, err)

	devPriv := secp256k1.GenPrivKey()
	devPubHex := hex.EncodeToString(devPriv.PubKey().(*secp256k1.PubKey).Key)
	deviceHash := strings.Repeat("cd", 32)

	nonce1 := "nonce-cycle-0001"
	sig1, _ := devPriv.Sign([]byte(deviceHash + "|" + nonce1))
	require.NoError(t, k.SubmitAttestation(ctx, addr, "root-1", nonce1, deviceHash, devPubHex, hex.EncodeToString(sig1)))
	require.NoError(t, k.MarkOracleVerified(ctx, addr, "challenge-1"))
	require.True(t, k.IsVerifiedAttested(ctx, addr))

	nonce2 := "nonce-cycle-0002"
	sig2, _ := devPriv.Sign([]byte(deviceHash + "|" + nonce2))
	require.NoError(t, k.SubmitAttestation(ctx, addr, "root-2", nonce2, deviceHash, devPubHex, hex.EncodeToString(sig2)))

	require.True(t, k.IsAttested(ctx, addr), "新周期仍是有效认证")
	require.False(t, k.IsVerifiedAttested(ctx, addr), "新周期必须重新取得预言机背书")
}

// TestSubmitAttestationRejectsBoundedInputs 输入字段的长度/字符集闸门：
// nonce 与 device_id_hash 会直接进入 KVStore 键，无上限即等于把状态体积
// 的控制权交给任意提交者。
func TestSubmitAttestationRejectsBoundedInputs(t *testing.T) {
	k, ctx := keepertest.PhonenodeKeeper(t)
	priv := secp256k1.GenPrivKey()
	addr := sdk.AccAddress(priv.PubKey().Address()).String()
	_, err := k.RegisterNode(ctx, addr, "pixel8", "android", "contributor")
	require.NoError(t, err)

	devPriv := secp256k1.GenPrivKey()
	devPubHex := hex.EncodeToString(devPriv.PubKey().(*secp256k1.PubKey).Key)
	goodHash := strings.Repeat("ef", 32)
	goodNonce := "nonce-valid-01"
	sig, _ := devPriv.Sign([]byte(goodHash + "|" + goodNonce))
	sigHex := hex.EncodeToString(sig)

	// 超长 nonce（1 MiB）
	hugeNonce := strings.Repeat("n", 1<<20)
	require.Error(t, k.SubmitAttestation(ctx, addr, "root", hugeNonce, goodHash, devPubHex, sigHex),
		"超长 nonce 必须被拒（否则可确定性地膨胀状态）")

	// 过短 nonce
	require.Error(t, k.SubmitAttestation(ctx, addr, "root", "short", goodHash, devPubHex, sigHex))

	// 大小写不符 / 非 hex 的 device_id_hash
	require.Error(t, k.SubmitAttestation(ctx, addr, "root", goodNonce, strings.ToUpper(goodHash), devPubHex, sigHex))
	require.Error(t, k.SubmitAttestation(ctx, addr, "root", goodNonce, "nothex-device-hash-abcdefghijklmnopqrstuvwxyz", devPubHex, sigHex))

	// 超长 root_hash
	require.Error(t, k.SubmitAttestation(ctx, addr, strings.Repeat("r", types.MaxRootHashLength+1), goodNonce, goodHash, devPubHex, sigHex))

	// 合法输入仍应通过（闸门不能误伤正常路径）
	okNonce := "nonce-valid-02"
	okSig, _ := devPriv.Sign([]byte(goodHash + "|" + okNonce))
	require.NoError(t, k.SubmitAttestation(ctx, addr, "root-ok", okNonce, goodHash, devPubHex, hex.EncodeToString(okSig)))
}

// TestDeviceIDCommitmentIsDomainSeparated 承诺值必须带链 ID 域分隔，
// 否则同一设备在不同链上的承诺可被交叉关联。
func TestDeviceIDCommitmentIsDomainSeparated(t *testing.T) {
	h := strings.Repeat("ab", 32)
	a := keeper.DeviceIDCommitment("mcchain-1", h)
	b := keeper.DeviceIDCommitment("mcchain-testnet-1", h)

	require.Len(t, a, 16) // 8 字节 hex
	require.NotEqual(t, a, b, "不同链 ID 必须产生不同承诺")
	require.Equal(t, a, keeper.DeviceIDCommitment("mcchain-1", h), "同一输入必须稳定")
	require.NotEqual(t, a, h, "承诺不得等于原始设备哈希")
}
