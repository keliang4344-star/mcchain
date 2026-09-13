package keeper_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tmdb "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/libs/log"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/store"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	typesparams "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/stretchr/testify/require"
	"mcchain/x/phonenode/keeper"
	"mcchain/x/phonenode/types"
	tokenomicstypes "mcchain/x/tokenomics/types"
)

// ---- mock bank keeper（仅实现节点津贴发放所需的接口子集）----

type mockBankAllowance struct {
	balances map[string]sdk.Coins
	sent     []struct {
		from, to string
		amt      sdk.Coins
	}
}

func newMockBankAllowance() *mockBankAllowance {
	return &mockBankAllowance{balances: map[string]sdk.Coins{}}
}

func (m *mockBankAllowance) mint(addr string, amt sdk.Coins) {
	m.balances[addr] = m.balances[addr].Add(amt...)
}

func (m *mockBankAllowance) SpendableCoins(_ sdk.Context, addr sdk.AccAddress) sdk.Coins {
	return m.balances[addr.String()]
}

// GetBalance returns the mock's tracked balance for the given denom.
func (m *mockBankAllowance) GetBalance(_ sdk.Context, addr sdk.AccAddress, denom string) sdk.Coin {
	return sdk.NewCoin(denom, m.balances[addr.String()].AmountOf(denom))
}

func (m *mockBankAllowance) SendCoinsFromModuleToModule(_ sdk.Context, _, _ string, _ sdk.Coins) error {
	return nil
}

func (m *mockBankAllowance) BurnCoins(_ sdk.Context, _ string, _ sdk.Coins) error {
	return nil
}

func (m *mockBankAllowance) SendCoinsFromModuleToAccount(_ sdk.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	from := authtypes.NewModuleAddress(senderModule).String()
	have := m.balances[from]
	if !have.IsAllGTE(amt) {
		return fmt.Errorf("insufficient funds in module %s", senderModule)
	}
	m.balances[from] = have.Sub(amt...)
	m.balances[recipientAddr.String()] = m.balances[recipientAddr.String()].Add(amt...)
	m.sent = append(m.sent, struct {
		from, to string
		amt      sdk.Coins
	}{from, recipientAddr.String(), amt})
	return nil
}

// ---- mock depin 释放闸门（津贴必须走设备池每日线性释放额度）----

type mockDepinGate struct {
	remaining uint64 // 当日剩余可释放额度（umc）
	recorded  uint64 // 累计已记账释放（umc）
	err       error  // 注入查询故障
}

func newMockDepinGate(remaining uint64) *mockDepinGate {
	return &mockDepinGate{remaining: remaining}
}

func (m *mockDepinGate) CheckDailyReleaseCap(_ sdk.Context, amount uint64) (bool, uint64, uint64, error) {
	if m.err != nil {
		return false, 0, 0, m.err
	}
	return amount <= m.remaining, m.remaining + m.recorded, m.remaining, nil
}

func (m *mockDepinGate) RecordDailyRelease(_ sdk.Context, amount uint64) {
	m.recorded += amount
	if amount >= m.remaining {
		m.remaining = 0
		return
	}
	m.remaining -= amount
}

// newKeeperWithBank 构造带 mock bank + 宽松释放闸门的 phonenode keeper。
func newKeeperWithBank(t *testing.T, bank types.BankKeeper) (*keeper.Keeper, sdk.Context) {
	k, ctx, _ := newKeeperWithBankAndGate(t, bank, newMockDepinGate(1<<62))
	return k, ctx
}

// newKeeperWithBankAndGate 允许显式注入释放闸门，用于额度限流用例。
func newKeeperWithBankAndGate(t *testing.T, bank types.BankKeeper, gate *mockDepinGate) (*keeper.Keeper, sdk.Context, *mockDepinGate) {
	t.Helper()
	storeKey := sdk.NewKVStoreKey(types.StoreKey)
	memStoreKey := storetypes.NewMemoryStoreKey(types.MemStoreKey)
	db := tmdb.NewMemDB()
	stateStore := store.NewCommitMultiStore(db)
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	stateStore.MountStoreWithDB(memStoreKey, storetypes.StoreTypeMemory, nil)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	paramsSubspace := typesparams.NewSubspace(cdc, types.Amino, storeKey, memStoreKey, "PhonenodeParams")
	k := keeper.NewKeeper(cdc, storeKey, memStoreKey, paramsSubspace, bank, nil, nil)
	if gate != nil {
		k.SetDepinKeeper(gate)
	}
	ctx := sdk.NewContext(stateStore, tmproto.Header{}, false, log.NewNopLogger())
	k.SetParams(ctx, types.DefaultParams())
	return k, ctx, gate
}

func fundDepin(t *testing.T, bank *mockBankAllowance, mc uint64) {
	bank.mint(authtypes.NewModuleAddress(tokenomicstypes.DepinModuleName).String(),
		sdk.NewCoins(sdk.NewCoin(tokenomicstypes.DefaultDenom, sdk.NewIntFromUint64(mc*1_000_000))))
}

// attestVerified 注册一个「预言机已背书」的节点（身份闸门之后的合格形态）。
//
// 节点资本津贴是无条件的 30 MC/日 现金流，身份闸门已提升到 TierOracle，
// 因此测试用例里只做设备自签（TierSelf）是不够的 —— 那正是被女巫利用的形态。
//
// 之后背书自带 7 天有效期，因此背书时点必须贴近「领取时点」：
// 默认按用例通用的第 100 天背书，需要别的时点请用 attestVerifiedAt。
func attestVerified(t *testing.T, k *keeper.Keeper, ctx sdk.Context, op sdk.AccAddress, model string) {
	t.Helper()
	attestVerifiedAt(t, k, ctx, op, model, 86400*100)
}

// attestVerifiedAt 与 attestVerified 相同，但显式指定背书发生的 Unix 时间，
// 便于断言「背书过期后不再具备领取资格」。
func attestVerifiedAt(t *testing.T, k *keeper.Keeper, ctx sdk.Context, op sdk.AccAddress, model string, endorseAtUnix int64) {
	t.Helper()
	_, err := k.RegisterNode(ctx, op.String(), model, "android", "validator")
	require.NoError(t, err)
	// expiry 设在第 200 天，晚于用例里第 100/101 天的发放时点，确保未过期。
	k.SetAttestation(ctx, op.String(), types.NewValidAttestation("roothash", "nonce", "deviceidhash", 86400*200))
	// 背书写入的是「当时的区块时间」，因此必须在对应时点的 ctx 上调用。
	signedCtx := ctx.WithBlockTime(time.Unix(endorseAtUnix, 0))
	require.NoError(t, k.MarkOracleVerified(signedCtx, op.String(), "roothash"),
		"预言机背书必须成功，否则节点停留在 TierSelf、领不到津贴")
}

func TestNodeCapitalAllowanceDailyPayout(t *testing.T) {
	bank := newMockBankAllowance()
	k, ctx := newKeeperWithBank(t, bank)
	fundDepin(t, bank, 1000) // 1000 MC 进设备池

	priv := secp256k1.GenPrivKey()
	op := sdk.AccAddress(priv.PubKey().Address())
	// 设备认证有效 + 预言机已背书（白皮书：已注册 + 认证有效 + 未被 jail 才可领津贴）。
	attestVerified(t, k, ctx, op, "pixel8")

	// 第 100 天
	ctx = ctx.WithBlockTime(time.Unix(86400*100, 0))
	require.NoError(t, k.DistributeNodeCapitalAllowances(ctx))
	require.Equal(t, int64(30_000_000), bank.balances[op.String()].AmountOf(tokenomicstypes.DefaultDenom).Int64())

	// 同日再分发 → 不应重复发放
	require.NoError(t, k.DistributeNodeCapitalAllowances(ctx))
	require.Equal(t, int64(30_000_000), bank.balances[op.String()].AmountOf(tokenomicstypes.DefaultDenom).Int64(),
		"同日重复分发不应叠加")

	// 第 101 天 → 再发一日
	ctx = ctx.WithBlockTime(time.Unix(86400*101, 0))
	require.NoError(t, k.DistributeNodeCapitalAllowances(ctx))
	require.Equal(t, int64(60_000_000), bank.balances[op.String()].AmountOf(tokenomicstypes.DefaultDenom).Int64())
}

func TestNodeCapitalAllowanceDisabled(t *testing.T) {
	bank := newMockBankAllowance()
	k, ctx := newKeeperWithBank(t, bank)
	fundDepin(t, bank, 1000)

	priv := secp256k1.GenPrivKey()
	op := sdk.AccAddress(priv.PubKey().Address())
	_, err := k.RegisterNode(ctx, op.String(), "pixel8", "android", "validator")
	require.NoError(t, err)

	k.SetNodeAllowanceConfig(ctx, keeper.NodeAllowanceConfig{Enabled: false, PerDay: 30_000_000})
	ctx = ctx.WithBlockTime(time.Unix(86400*100, 0))
	require.NoError(t, k.DistributeNodeCapitalAllowances(ctx))
	require.Empty(t, bank.balances[op.String()], "禁用后不应发放")
}

func TestNodeCapitalAllowanceSkipsJailed(t *testing.T) {
	bank := newMockBankAllowance()
	k, ctx := newKeeperWithBank(t, bank)
	fundDepin(t, bank, 1000)

	priv := secp256k1.GenPrivKey()
	op := sdk.AccAddress(priv.PubKey().Address())
	attestVerified(t, k, ctx, op, "pixel8")
	// 标记 jail
	node, _ := k.GetNode(ctx, op.String())
	node.VerifierStatus = "jailed"
	require.NoError(t, k.SetNode(ctx, node))

	ctx = ctx.WithBlockTime(time.Unix(86400*100, 0))
	require.NoError(t, k.DistributeNodeCapitalAllowances(ctx))
	require.Empty(t, bank.balances[op.String()], "jail 节点不应领取津贴")
}

// 节点资本津贴必须受设备池「每日线性释放额度」约束。
// 闸门只剩 30 MC 时，两个合格节点里只能有一个拿到，且释放额被如实记账。
func TestNodeCapitalAllowanceRespectsDepinDailyCap(t *testing.T) {
	bank := newMockBankAllowance()
	gate := newMockDepinGate(30_000_000) // 当日仅剩 30 MC 额度
	k, ctx, _ := newKeeperWithBankAndGate(t, bank, gate)
	fundDepin(t, bank, 1000)

	ops := make([]sdk.AccAddress, 0, 2)
	for i := 0; i < 2; i++ {
		op := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
		attestVerified(t, k, ctx, op, fmt.Sprintf("pixel8-%d", i))
		ops = append(ops, op)
	}

	ctx = ctx.WithBlockTime(time.Unix(86400*100, 0))
	require.NoError(t, k.DistributeNodeCapitalAllowances(ctx))

	paid := 0
	for _, op := range ops {
		if bank.balances[op.String()].AmountOf(tokenomicstypes.DefaultDenom).IsPositive() {
			paid++
		}
	}
	require.Equal(t, 1, paid, "额度只够一个节点，第二个必须被闸门拦下")
	require.Equal(t, uint64(30_000_000), gate.recorded, "实际出账必须如实计入设备池当日释放")
	require.Zero(t, gate.remaining, "额度应被用尽")

	// 额度耗尽后再分发一次，不得再出账。
	require.NoError(t, k.DistributeNodeCapitalAllowances(ctx))
	require.Equal(t, uint64(30_000_000), gate.recorded, "额度用尽后不得继续释放")
}

// （fail-closed）：释放闸门未接线时，宁可不发也不得裸抽设备池。
func TestNodeCapitalAllowanceWithheldWhenGateUnwired(t *testing.T) {
	bank := newMockBankAllowance()
	k, ctx, _ := newKeeperWithBankAndGate(t, bank, nil) // 不接线
	fundDepin(t, bank, 1000)

	op := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	attestVerified(t, k, ctx, op, "pixel8")

	ctx = ctx.WithBlockTime(time.Unix(86400*100, 0))
	require.NoError(t, k.DistributeNodeCapitalAllowances(ctx))
	require.Empty(t, bank.balances[op.String()], "闸门未接线时必须 fail-closed，不得发放")
}

// 回归：节点资本津贴不得发给仅持有设备自签
// attestation 的节点。
//
// 攻击形态：自签材料任何人都能自己造（生成一对 secp256k1 密钥即可），女巫成本
// 为零。津贴是无条件的 30 MC/节点/日，且已接设备池日释放闸门，于是批量造节点就
// 能把当日排放额度全部占满，真实设备的 DePIN 贡献拨付被限流到 0 —— 主网上线
// 首日就会出现「真实矿工零收益、女巫躺着领钱」，属于上线即致命的经济缺陷。
func TestNodeCapitalAllowanceRequiresOracleTier(t *testing.T) {
	bank := newMockBankAllowance()
	k, ctx := newKeeperWithBank(t, bank)
	fundDepin(t, bank, 1000)
	ctx = ctx.WithBlockTime(time.Unix(86400*100, 0))

	// 女巫节点：自己造密钥、自己签 attestation，全流程无需任何真实设备。
	sybil := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	_, err := k.RegisterNode(ctx, sybil.String(), "emulator", "android", "validator")
	require.NoError(t, err)
	k.SetAttestation(ctx, sybil.String(), types.NewValidAttestation("roothash", "nonce", "deviceidhash", 86400*200))

	require.True(t, k.IsAttested(ctx, sybil.String()), "自签 attestation 本身是有效的（参与门槛不变）")
	require.False(t, k.IsVerifiedAttested(ctx, sybil.String()), "自签不等于预言机背书")

	require.NoError(t, k.DistributeNodeCapitalAllowances(ctx))
	require.Empty(t, bank.balances[sybil.String()],
		"仅凭自签 attestation 不得领取节点资本津贴（女巫抽干设备池）")

	// 真实设备：预言机验签通过、层级回写为 TierOracle 之后，同一节点应能正常领取。
	require.NoError(t, k.MarkOracleVerified(ctx, sybil.String(), "roothash"))
	require.NoError(t, k.DistributeNodeCapitalAllowances(ctx))
	require.Equal(t, int64(30_000_000),
		bank.balances[sybil.String()].AmountOf(tokenomicstypes.DefaultDenom).Int64(),
		"预言机背书后应正常发放津贴（闸门不是把功能关死）")
}

// 回归：预言机背书是有期限的断言，过期后必须重新走真机校验。
//
// 攻击形态：设备通过一次真机校验拿到背书后长期搁置，期间把设备换成模拟器、
// 或转手给他人，仍继续按「真实设备」身份领取津贴与 DePIN 拨付。
// 背书的有效期让「一次校验」无法兑换「长期收益」。
func TestNodeCapitalAllowanceRejectsExpiredOracleEndorsement(t *testing.T) {
	bank := newMockBankAllowance()
	k, ctx := newKeeperWithBank(t, bank)
	fundDepin(t, bank, 1000)

	op := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	// 背书发生在第 100 天，但领取发生在第 120 天（>7 天有效期）。
	attestVerifiedAt(t, k, ctx, op, "pixel8", 86400*100)

	at100 := ctx.WithBlockTime(time.Unix(86400*100, 0))
	require.True(t, k.IsVerifiedAttested(at100, op.String()), "背书当日应有效")

	at120 := ctx.WithBlockTime(time.Unix(86400*120, 0))
	require.False(t, k.IsVerifiedAttested(at120, op.String()),
		"背书超过 %d 秒后必须失效", types.OracleEndorsementValidity)

	require.NoError(t, k.DistributeNodeCapitalAllowances(at120))
	require.Empty(t, bank.balances[op.String()],
		"背书过期后不得继续领取津贴")

	// 重新认证（新一轮背书）后恢复资格 —— 闸门不是把功能关死。
	require.NoError(t, k.MarkOracleVerified(at120, op.String(), "roothash-fresh"))
	require.NoError(t, k.DistributeNodeCapitalAllowances(at120))
	require.Equal(t, int64(30_000_000),
		bank.balances[op.String()].AmountOf(tokenomicstypes.DefaultDenom).Int64(),
		"重新背书后应恢复领取资格")
}

// 回归：同一 challenge 不得被重复消费（重放防护）。
//
// 攻击形态：一对合法的 (challenge, signature) 在被广播后，攻击者原样重放即可
// 反复把节点刷回 TierOracle —— 即使节点已被 slash 进冷却期、或背书已过期降级。
func TestMarkOracleVerifiedRejectsChallengeReplay(t *testing.T) {
	bank := newMockBankAllowance()
	k, ctx := newKeeperWithBank(t, bank)

	op := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	_, err := k.RegisterNode(ctx, op.String(), "pixel8", "android", "validator")
	require.NoError(t, err)
	k.SetAttestation(ctx, op.String(), types.NewValidAttestation("roothash", "nonce", "deviceidhash", 86400*200))

	// 首次背书成功
	require.NoError(t, k.MarkOracleVerified(ctx, op.String(), "challenge-once"))
	require.True(t, k.IsVerifiedAttested(ctx, op.String()))

	// 重放同一 challenge → 必须被拒
	err = k.MarkOracleVerified(ctx, op.String(), "challenge-once")
	require.Error(t, err, "同一 challenge 不得重复消费")

	// 层级回落（模拟新认证周期）后，重放旧 challenge 也不得恢复 TierOracle
	k.SetAttestationTier(ctx, op.String(), types.AttestationTierSelf)
	err = k.MarkOracleVerified(ctx, op.String(), "challenge-once")
	require.Error(t, err, "降级后重放旧 challenge 同样必须被拒")
	require.False(t, k.IsVerifiedAttested(ctx, op.String()))
}

// challenge 输入边界（空 / 超长 / 不可打印）。
func TestMarkOracleVerifiedRejectsInvalidChallenge(t *testing.T) {
	k, ctx := newKeeperWithBank(t, &mockBankAllowance{})
	op := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	_, err := k.RegisterNode(ctx, op.String(), "pixel8", "android", "validator")
	require.NoError(t, err)
	k.SetAttestation(ctx, op.String(), types.NewValidAttestation("roothash", "nonce", "deviceidhash", 86400*200))

	require.Error(t, k.MarkOracleVerified(ctx, op.String(), ""), "空 challenge 必须被拒")
	require.Error(t, k.MarkOracleVerified(ctx, op.String(), strings.Repeat("a", types.MaxOracleChallengeLength+1)),
		"超长 challenge 必须被拒")
	require.Error(t, k.MarkOracleVerified(ctx, op.String(), "bad\nchallenge"),
		"不可打印字符必须被拒")
	require.False(t, k.IsVerifiedAttested(ctx, op.String()))
}

// 没有有效自签 attestation 时不得凭外部调用凭空造壳（前置条件）。
func TestMarkOracleVerifiedRequiresSelfAttestation(t *testing.T) {
	k, ctx := newKeeperWithBank(t, &mockBankAllowance{})
	op := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	_, err := k.RegisterNode(ctx, op.String(), "pixel8", "android", "validator")
	require.NoError(t, err)
	// 未做任何 attestation
	require.Error(t, k.MarkOracleVerified(ctx, op.String(), "challenge-x"),
		"未自签认证的节点不得被提升到 TierOracle")
	require.False(t, k.IsVerifiedAttested(ctx, op.String()))
}
