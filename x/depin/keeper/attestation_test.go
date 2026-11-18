package keeper_test

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	keepertest "mcchain/testutil/keeper"
	"mcchain/x/depin/keeper"
	"mcchain/x/depin/types"
)

// mockPhonenode 是 PhononenodeKeeper 的极简实现，用于驱动 SubmitAttestation 的
// 全链路（白名单通过后需经 phonenode 链上 attestation 状态校验）。
//
// 接口新增 IsVerifiedAttested / MarkOracleVerified 两个方法后，mock 必须同步实现，
// 否则「预言机验签结果回写 phonenode 层级」这一步在测试里无法被观测。
type mockPhonenode struct {
	hasNode  bool
	attested bool
	verified bool

	// promoteErr 非 nil 时模拟回写失败（一致性故障必须向上冒泡，不得静默吞掉）。
	promoteErr error
	// promotions 记录被成功回写为 TierOracle 的 challenge，供断言使用。
	promotions []string
}

func (m *mockPhonenode) HasNode(_ sdk.Context, _ string) bool    { return m.hasNode }
func (m *mockPhonenode) IsAttested(_ sdk.Context, _ string) bool { return m.attested }
func (m *mockPhonenode) IsVerifiedAttested(_ sdk.Context, _ string) bool {
	return m.attested && m.verified
}

func (m *mockPhonenode) MarkOracleVerified(_ sdk.Context, _ string, challenge string) error {
	if m.promoteErr != nil {
		return m.promoteErr
	}
	m.promotions = append(m.promotions, challenge)
	m.verified = true
	return nil
}

func newAttestKeeper(t testing.TB) (*keeper.Keeper, sdk.Context) {
	k, ctx := keepertest.DepinKeeper(t)
	return k, ctx
}

func oracleAddr(seed string) string {
	return sdk.AccAddress([]byte(seed)).String()
}

// TestSubmitAttestation_RejectsUnauthorizedOracle：生产链下，签名预言机不在白名单
// 内应被拒（闸门在 VerifyDeviceAttestation 之前，无需 phonenode mock）。
func TestSubmitAttestation_RejectsUnauthorizedOracle(t *testing.T) {
	k, ctx := newAttestKeeper(t)
	auth := oracleAddr("auth-oracle-address-1234567890")
	rogue := oracleAddr("rogue-oracle-address-123456789")

	k.SetOracleWhitelist(ctx, []string{auth})
	srv := keeper.NewMsgServerImpl(*k)

	msg := types.NewMsgSubmitAttestation("device-1", "proof", "sig", rogue)
	_, err := srv.SubmitAttestation(sdk.WrapSDKContext(ctx), msg)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrUnauthorizedOracle)
}

// TestSubmitAttestation_RejectsWhenOracleNotConfigured：生产链白名单为空 → fail-closed。
func TestSubmitAttestation_RejectsWhenOracleNotConfigured(t *testing.T) {
	k, ctx := newAttestKeeper(t)
	rogue := oracleAddr("rogue-oracle-address-123456789")

	k.SetOracleWhitelist(ctx, nil) // 空白名单
	srv := keeper.NewMsgServerImpl(*k)

	msg := types.NewMsgSubmitAttestation("device-1", "proof", "sig", rogue)
	_, err := srv.SubmitAttestation(sdk.WrapSDKContext(ctx), msg)
	require.ErrorIs(t, err, types.ErrOracleNotConfigured)
}

// TestSubmitAttestation_AcceptedWhenAuthorized：白名单内预言机 + 设备已认证 → 通过。
// 生产链上真实验签须经 TeeOracle（secp256k1 预言机签名），而非 SoftOracle 的
// 非空校验——这同时覆盖了 P0-01 修复后的生产 happy path。
func TestSubmitAttestation_AcceptedWhenAuthorized(t *testing.T) {
	k, ctx := newAttestKeeper(t)
	auth := oracleAddr("auth-oracle-address-1234567890")

	// 注入真实验签预言机：用预言机私钥对 "device-1|proof" 签名，base64 上链。
	priv := secp256k1.GenPrivKey()
	types.SetOracle(types.NewTeeOracle(priv.PubKey()))
	t.Cleanup(func() { types.SetOracle(&types.SoftOracle{}) })

	sigBz, err := priv.Sign([]byte("device-1|proof"))
	require.NoError(t, err)

	k.SetOracleWhitelist(ctx, []string{auth})
	k.SetPhonenodeKeeper(&mockPhonenode{hasNode: true, attested: true})
	srv := keeper.NewMsgServerImpl(*k)

	msg := types.NewMsgSubmitAttestation("device-1", "proof", base64.StdEncoding.EncodeToString(sigBz), auth)
	resp, err := srv.SubmitAttestation(sdk.WrapSDKContext(ctx), msg)
	require.NoError(t, err)
	require.True(t, resp.Passed)
}

// TestSubmitAttestation_DevChainExempt：非生产链豁免白名单闸门（兼容 SoftOracle）。
func TestSubmitAttestation_DevChainExempt(t *testing.T) {
	k, ctx := newAttestKeeper(t)
	rogue := oracleAddr("rogue-oracle-address-123456789")

	k.SetOracleWhitelist(ctx, nil) // 空白名单
	k.SetPhonenodeKeeper(&mockPhonenode{hasNode: true, attested: true})
	srv := keeper.NewMsgServerImpl(*k)

	devCtx := ctx.WithChainID("mcchain-testnet-1")
	msg := types.NewMsgSubmitAttestation("device-1", "proof", "sig", rogue)
	resp, err := srv.SubmitAttestation(sdk.WrapSDKContext(devCtx), msg)
	require.NoError(t, err) // 闸门跳过
	require.True(t, resp.Passed)
}

// TestStoreAttestationResult_BoundedHistory：历史记录无界追加被截断为最近 N 条。
func TestStoreAttestationResult_BoundedHistory(t *testing.T) {
	k, ctx := newAttestKeeper(t)
	deviceID := "device-bounded"
	total := keeper.MaxAttestationHistory + 50
	for i := 0; i < total; i++ {
		r := types.NewAttestationResult(deviceID, true, "ok", "oracle", int64(i))
		require.NoError(t, k.StoreAttestationResult(ctx, deviceID, r))
	}

	hist := k.GetAttestationHistory(ctx, deviceID)
	require.Len(t, hist.Results, keeper.MaxAttestationHistory)
	// 最旧记录（timestamp=0）应已被丢弃；首条应为第 50 条（timestamp=50）
	require.Equal(t, int64(50), hist.Results[0].Timestamp)
	require.Equal(t, int64(total-1), hist.Results[keeper.MaxAttestationHistory-1].Timestamp)
}

// ---------------------------------------------------------------------------
// 打破循环信任链 —— 预言机的独立证明材料必须被真正消费
//
// 背景：修复前 depin.VerifyDeviceAttestation 把入参 proof / signature 整段丢弃，
// 只回读 phonenode.IsAttested；而 phonenode 的 attestation 本身只是设备自签。
// 于是「depin 信任 phonenode、phonenode 只信设备自己」构成闭环空转，预言机白名单
// 虽在但不含任何独立信息。以下用例锁定修复后的单向信任方向：
// 设备自签（TierSelf） → 预言机真机校验（TierOracle） → 经济权重。
// ---------------------------------------------------------------------------

// 生产链 + 无独立证明材料 + 仅有自签 → 必须拒绝。
func TestVerifyDeviceAttestation_SelfTierRejectedOnProductionChain(t *testing.T) {
	k, ctx := newAttestKeeper(t)
	pn := &mockPhonenode{hasNode: true, attested: true} // 只有自签，无预言机背书
	k.SetPhonenodeKeeper(pn)

	passed, reason := k.VerifyDeviceAttestation(ctx, "device-1", "", "")
	require.False(t, passed)
	require.Contains(t, reason, "self attestation only")
	require.Empty(t, pn.promotions, "被拒时不应发生层级回写")
}

// 已具备预言机背书层级 → 无独立证明材料也放行。
func TestVerifyDeviceAttestation_OracleTierAccepted(t *testing.T) {
	k, ctx := newAttestKeeper(t)
	k.SetPhonenodeKeeper(&mockPhonenode{hasNode: true, attested: true, verified: true})

	passed, reason := k.VerifyDeviceAttestation(ctx, "device-1", "", "")
	require.True(t, passed)
	require.Contains(t, reason, "oracle-endorsed")
}

// 携带独立证明材料 → 走预言机验签，并把验过的 challenge 回写 phonenode。
// 使用非生产链，使默认 SoftOracle（开发/测试用）可放行，聚焦「证明消费 + 层级回写」语义。
func TestVerifyDeviceAttestation_ProofConsumedAndTierPromoted(t *testing.T) {
	k, ctx := newAttestKeeper(t)
	pn := &mockPhonenode{hasNode: true, attested: true}
	k.SetPhonenodeKeeper(pn)

	devCtx := ctx.WithChainID("mcchain-testnet-1")
	passed, reason := k.VerifyDeviceAttestation(devCtx, "device-1", "challenge-abc", "sig")
	require.True(t, passed)
	require.Contains(t, reason, "oracle + phonenode")
	require.Equal(t, []string{"challenge-abc"}, pn.promotions,
		"预言机验过的 challenge 必须被回写，否则证明仍然没有落入链上状态")
}

// 回写失败属一致性故障，必须让整次验证失败而不是静默降级为「已通过」。
// 非生产链使 SoftOracle 可放行，从而让流程推进到回写步骤并验证其失败冒泡。
func TestVerifyDeviceAttestation_PromotionFailurePropagates(t *testing.T) {
	k, ctx := newAttestKeeper(t)
	k.SetPhonenodeKeeper(&mockPhonenode{hasNode: true, attested: true, promoteErr: errors.New("state boom")})

	devCtx := ctx.WithChainID("mcchain-testnet-1")
	passed, reason := k.VerifyDeviceAttestation(devCtx, "device-1", "challenge-abc", "sig")
	require.False(t, passed)
	require.Contains(t, reason, "failed to record oracle endorsement")
}

// P0-01 回归：生产链上，即便 DefaultOracle 是 SoftOracle（如运维误设
// MC_ORACLE_ALLOW_SOFT=1），携带任意非空 proof/signature 也必须被拒绝——
// 生产链只认 TeeOracle 的真实 secp256k1 验签，不允许非空即通过的软认证。
func TestVerifyDeviceAttestation_SoftOracleRejectedOnProductionChain(t *testing.T) {
	k, ctx := newAttestKeeper(t)
	k.SetPhonenodeKeeper(&mockPhonenode{hasNode: true, attested: true})

	// 显式确保当前预言机为 SoftOracle（默认值），并在生产 chain-id 下提交非空证明。
	types.SetOracle(&types.SoftOracle{})
	t.Cleanup(func() { types.SetOracle(&types.SoftOracle{}) })

	passed, reason := k.VerifyDeviceAttestation(ctx, "device-1", "challenge-abc", "sig")
	require.False(t, passed)
	require.Contains(t, reason, "SoftOracle not permitted on production chain")
}

// 非生产链（test/dev/local/sim）无独立证明材料时放宽到基础层，保持冷启动与 CI 不变。
func TestVerifyDeviceAttestation_DevChainAcceptsSelfTier(t *testing.T) {
	k, ctx := newAttestKeeper(t)
	k.SetPhonenodeKeeper(&mockPhonenode{hasNode: true, attested: true})

	devCtx := ctx.WithChainID("mcchain-testnet-1")
	passed, reason := k.VerifyDeviceAttestation(devCtx, "device-1", "", "")
	require.True(t, passed)
	require.Contains(t, reason, "self tier")
}
