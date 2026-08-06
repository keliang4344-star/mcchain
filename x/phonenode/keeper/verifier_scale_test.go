package keeper_test

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	keepertest "mcchain/testutil/keeper"
	"mcchain/x/phonenode/keeper"
	"mcchain/x/phonenode/types"
)

// ---------------------------------------------------------------------------
// 上线前审计致命项回归测试（phonenode）
//
//	VALADDR-1 节点以账户地址登记，罚没与验证者遴选必须按同字节换算出验证人地址；
//	          旧实现用 ValAddressFromBech32 解析账户地址，恒失败 →
//	          验证人作恶不掉一分质押、验证者集合恒为空。
//	SCALE-2   验证者遴选必须遍历「验证人集合」（受 MaxValidators 封顶），
//	          而不是遍历「全量注册设备」。
// ---------------------------------------------------------------------------

// bondedValidator 造一个 bonded、自质押 tokens 的验证人，
// 同时返回其账户地址形态（phonenode 的登记主键）。
func bondedValidator(t *testing.T, tokens int64) (stakingtypes.Validator, string) {
	t.Helper()
	consPub := ed25519.GenPrivKey().PubKey()
	valAddr := sdk.ValAddress(consPub.Address())
	val, err := stakingtypes.NewValidator(valAddr, consPub, stakingtypes.Description{Moniker: "v"})
	require.NoError(t, err)
	val.Status = stakingtypes.Bonded
	val.Tokens = sdk.NewInt(tokens)
	return val, sdk.AccAddress(consPub.Address()).String()
}

// attestNode 注册节点并写入一条有效 attestation（心跳高度取当前块高）。
func attestNode(t *testing.T, k *keeper.Keeper, ctx sdk.Context, addr string) {
	t.Helper()
	_, err := k.RegisterNode(ctx, addr, "pixel8", "android", "contributor")
	require.NoError(t, err)
	expiry := ctx.BlockTime().Unix() + types.DefaultParams().AttestationValidity
	k.SetAttestation(ctx, addr, types.NewValidAttestation("root", "nonce", "devhash", expiry))
}

// TestVALADDR1VerifierNodesAcceptAccountAddress 锁定 VALADDR-1：
// 节点用账户地址（mc1...）登记，仍必须被识别为合格验证者。
// 旧实现在此处对每个节点都解码失败，本函数恒返回空集，
// EdgeAI 第三阶段抽检从未真正运行。
func TestVALADDR1VerifierNodesAcceptAccountAddress(t *testing.T) {
	val, accAddr := bondedValidator(t, 40_000_000_000) // 40000 MC > 30000 MC 门槛

	stk := &mockSlashStaking{val: val, hooks: &noopHooks{}}
	k, ctx := newSlashSplitKeeper(t, &mockSlashBank{}, stk, &mockSlashSlashing{})
	attestNode(t, k, ctx, accAddr)

	require.Equal(t, []string{accAddr}, k.GetVerifierNodes(ctx),
		"以账户地址登记的合格验证人节点必须进入验证者集合")
}

// TestVALADDR1SlashHitsValidatorRegisteredByAccountAddress 锁定罚没侧：
// 以账户地址发起 slash，必须真正扣到该验证人的自质押。
func TestVALADDR1SlashHitsValidatorRegisteredByAccountAddress(t *testing.T) {
	val, accAddr := bondedValidator(t, 1000)

	bank := &mockSlashBank{}
	stk := &mockSlashStaking{val: val, hooks: &noopHooks{}}
	slk := &mockSlashSlashing{}
	k, ctx := newSlashSplitKeeper(t, bank, stk, slk)

	require.NoError(t, k.SlashIfBad(ctx, accAddr, "double_sign", 2000))

	require.Equal(t, int64(200), stk.removed.Int64(),
		"账户地址形态的验证人节点必须被真正罚没自质押（旧实现在此静默跳过）")
	require.Len(t, slk.jailed, 1, "作恶验证人应被 jail")
}

// TestVerifierNodesExcludeUnderStaked 未达 30000 MC 门槛的验证人不得入选。
func TestVerifierNodesExcludeUnderStaked(t *testing.T) {
	val, accAddr := bondedValidator(t, 29_999_999_999)

	stk := &mockSlashStaking{val: val, hooks: &noopHooks{}}
	k, ctx := newSlashSplitKeeper(t, &mockSlashBank{}, stk, &mockSlashSlashing{})
	attestNode(t, k, ctx, accAddr)

	require.Empty(t, k.GetVerifierNodes(ctx), "自质押低于门槛不得成为验证者")
}

// TestVerifierNodesExcludeUnregisteredValidator 只做验证人、未注册为移动节点的，
// 不进入 EdgeAI 验证者集合。
func TestVerifierNodesExcludeUnregisteredValidator(t *testing.T) {
	val, _ := bondedValidator(t, 40_000_000_000)

	stk := &mockSlashStaking{val: val, hooks: &noopHooks{}}
	k, ctx := newSlashSplitKeeper(t, &mockSlashBank{}, stk, &mockSlashSlashing{})

	require.Empty(t, k.GetVerifierNodes(ctx), "未注册为移动节点的验证人不得入选")
}

// TestVerifierNodesExcludeStaleHeartbeat 心跳超出离线宽限的节点不得入选。
func TestVerifierNodesExcludeStaleHeartbeat(t *testing.T) {
	val, accAddr := bondedValidator(t, 40_000_000_000)

	stk := &mockSlashStaking{val: val, hooks: &noopHooks{}}
	k, ctx := newSlashSplitKeeper(t, &mockSlashBank{}, stk, &mockSlashSlashing{})
	attestNode(t, k, ctx, accAddr)

	stale := ctx.WithBlockHeight(ctx.BlockHeight() + types.DefaultParams().OfflineGraceBlocks + 1)
	require.Empty(t, k.GetVerifierNodes(stale), "心跳超时的节点不得继续担任验证者")
}

// TestSCALE2VerifierSelectionIsBoundedByValidatorSet 锁定 SCALE-2：
// 遴选成本只与验证人集合规模相关，与注册设备总量无关。
// 这里注册大量普通设备，验证者集合仍只含那一个合格验证人。
func TestSCALE2VerifierSelectionIsBoundedByValidatorSet(t *testing.T) {
	val, accAddr := bondedValidator(t, 40_000_000_000)

	stk := &mockSlashStaking{val: val, hooks: &noopHooks{}}
	k, ctx := newSlashSplitKeeper(t, &mockSlashBank{}, stk, &mockSlashSlashing{})
	attestNode(t, k, ctx, accAddr)

	// 注册 300 台普通设备（非验证人）——旧实现会把它们全部读进内存逐个筛，
	// 现实现根本不会碰到它们。
	for i := 0; i < 300; i++ {
		_, other := bondedValidator(t, 1)
		attestNode(t, k, ctx, other)
	}

	require.Equal(t, []string{accAddr}, k.GetVerifierNodes(ctx),
		"验证者集合应只由 staking 的 bonded 验证人决定，不随设备总量变化")
}

// TestVerifierNodesNilStakingKeeperSafe 未接 staking 时安全返回空集（不 panic）。
func TestVerifierNodesNilStakingKeeperSafe(t *testing.T) {
	k, ctx := keepertest.PhonenodeKeeper(t)
	require.Empty(t, k.GetVerifierNodes(ctx))
}
