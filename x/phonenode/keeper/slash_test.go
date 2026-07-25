package keeper_test

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	keepertest "mcchain/testutil/keeper"
	"mcchain/x/phonenode/types"
)

// TestSlashCooldownBlocksReAttest 锁定 B2 非验证人细则：
// 节点被 slash 后，在冷却期内禁止再认证（SubmitAttestation 返回 ErrSlashCooldown）。
func TestSlashCooldownBlocksReAttest(t *testing.T) {
	k, ctx := keepertest.PhonenodeKeeper(t)
	priv := secp256k1.GenPrivKey()
	addr := sdk.AccAddress(priv.PubKey().Address()).String()

	// 注册节点 + 有效 attestation
	_, err := k.RegisterNode(ctx, addr, "pixel8", "android", "contributor")
	require.NoError(t, err)
	expiry := ctx.BlockTime().Unix() + types.DefaultParams().AttestationValidity
	k.SetAttestation(ctx, addr, types.NewValidAttestation("root1", "nonce1", "devhash1", expiry))
	require.True(t, k.IsAttested(ctx, addr))

	// slash（非验证人路径：普通账户地址，不触发 staking/slashing）
	err = k.SlashIfBad(ctx, addr, "offline", 500)
	require.NoError(t, err)
	require.False(t, k.IsAttested(ctx, addr)) // attestation 已吊销
	require.True(t, k.InSlashCooldown(ctx, addr), "被 slash 后应处于冷却期")

	// 冷却期内再认证应被拒
	err = k.SubmitAttestation(ctx, addr, "root2", "nonce2", "devhash2")
	require.ErrorIs(t, err, types.ErrSlashCooldown)

	// 推进区块越过冷却，冷却解除，再认证放行
	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + k.GetParams(ctx).SlashCooldownBlocks + 1)
	require.False(t, k.InSlashCooldown(ctx, addr), "冷却期过后应解除")
	err = k.SubmitAttestation(ctx, addr, "root3", "nonce3", "devhash3")
	require.NoError(t, err)
	require.True(t, k.IsAttested(ctx, addr))
}

// TestSlashIfBadNonValidatorNoStaking 确保非验证人 slash 不触碰 staking keeper（nil 安全）。
func TestSlashIfBadNonValidatorNoStaking(t *testing.T) {
	k, ctx := keepertest.PhonenodeKeeper(t)
	priv := secp256k1.GenPrivKey()
	addr := sdk.AccAddress(priv.PubKey().Address()).String()
	// 普通账户地址、无 attestation：应仅记录 slash + 写冷却，不 panic
	err := k.SlashIfBad(ctx, addr, "fake_attest", 2000)
	require.NoError(t, err)
	require.True(t, k.InSlashCooldown(ctx, addr))
}
