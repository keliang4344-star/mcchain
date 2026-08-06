package keeper_test

import (
	"testing"

	tmdb "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/libs/log"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/store"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	typesparams "github.com/cosmos/cosmos-sdk/x/params/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	"mcchain/x/phonenode/keeper"
	"mcchain/x/phonenode/types"
	tokenomicstypes "mcchain/x/tokenomics/types"
)

// ---------------------------------------------------------------------------
// mocks：只实现 phonenode 罚没路径真正依赖的最小接口面
// ---------------------------------------------------------------------------

type routedSend struct {
	from   string
	to     string
	amount int64
}

type mockSlashBank struct {
	modToMod  []routedSend
	modToAcct []routedSend
}

func (m *mockSlashBank) SpendableCoins(_ sdk.Context, _ sdk.AccAddress) sdk.Coins {
	return sdk.NewCoins()
}

func (m *mockSlashBank) GetBalance(_ sdk.Context, _ sdk.AccAddress, denom string) sdk.Coin {
	return sdk.NewCoin(denom, sdk.ZeroInt())
}

func (m *mockSlashBank) SendCoinsFromModuleToModule(_ sdk.Context, from, to string, amt sdk.Coins) error {
	m.modToMod = append(m.modToMod, routedSend{from: from, to: to, amount: amt.AmountOf("stake").Int64()})
	return nil
}

func (m *mockSlashBank) SendCoinsFromModuleToAccount(_ sdk.Context, from string, to sdk.AccAddress, amt sdk.Coins) error {
	m.modToAcct = append(m.modToAcct, routedSend{from: from, to: to.String(), amount: amt.AmountOf("stake").Int64()})
	return nil
}

// noopHooks 满足 stakingtypes.StakingHooks，只需 BeforeValidatorSlashed 可被调用。
type noopHooks struct{ slashedCalls int }

func (h *noopHooks) AfterValidatorCreated(sdk.Context, sdk.ValAddress) error { return nil }
func (h *noopHooks) BeforeValidatorModified(sdk.Context, sdk.ValAddress) error {
	return nil
}
func (h *noopHooks) AfterValidatorRemoved(sdk.Context, sdk.ConsAddress, sdk.ValAddress) error {
	return nil
}
func (h *noopHooks) AfterValidatorBonded(sdk.Context, sdk.ConsAddress, sdk.ValAddress) error {
	return nil
}
func (h *noopHooks) AfterValidatorBeginUnbonding(sdk.Context, sdk.ConsAddress, sdk.ValAddress) error {
	return nil
}
func (h *noopHooks) BeforeDelegationCreated(sdk.Context, sdk.AccAddress, sdk.ValAddress) error {
	return nil
}
func (h *noopHooks) BeforeDelegationSharesModified(sdk.Context, sdk.AccAddress, sdk.ValAddress) error {
	return nil
}
func (h *noopHooks) BeforeDelegationRemoved(sdk.Context, sdk.AccAddress, sdk.ValAddress) error {
	return nil
}
func (h *noopHooks) AfterDelegationModified(sdk.Context, sdk.AccAddress, sdk.ValAddress) error {
	return nil
}
func (h *noopHooks) AfterUnbondingInitiated(sdk.Context, uint64) error { return nil }
func (h *noopHooks) BeforeValidatorSlashed(_ sdk.Context, _ sdk.ValAddress, _ sdk.Dec) error {
	h.slashedCalls++
	return nil
}

type mockSlashStaking struct {
	val     stakingtypes.Validator
	hooks   *noopHooks
	removed sdk.Int
	// notFound 模拟「该地址并非验证人」：Validator 返回 nil、GetValidator 返回 false。
	notFound bool
}

func (m *mockSlashStaking) Validator(_ sdk.Context, _ sdk.ValAddress) stakingtypes.ValidatorI {
	if m.notFound {
		return nil
	}
	return m.val
}

func (m *mockSlashStaking) ValidatorByConsAddr(_ sdk.Context, _ sdk.ConsAddress) stakingtypes.ValidatorI {
	if m.notFound {
		return nil
	}
	return m.val
}

func (m *mockSlashStaking) BondDenom(sdk.Context) string { return "stake" }

func (m *mockSlashStaking) GetValidator(_ sdk.Context, _ sdk.ValAddress) (stakingtypes.Validator, bool) {
	if m.notFound {
		return stakingtypes.Validator{}, false
	}
	return m.val, true
}

func (m *mockSlashStaking) RemoveValidatorTokens(_ sdk.Context, validator stakingtypes.Validator, tokensToRemove sdk.Int) stakingtypes.Validator {
	m.removed = tokensToRemove
	validator.Tokens = validator.Tokens.Sub(tokensToRemove)
	m.val = validator
	return validator
}

func (m *mockSlashStaking) Hooks() stakingtypes.StakingHooks { return m.hooks }

func (m *mockSlashStaking) GetBondedValidatorsByPower(_ sdk.Context) []stakingtypes.Validator {
	if m.notFound {
		return nil
	}
	return []stakingtypes.Validator{m.val}
}

type mockSlashSlashing struct{ jailed []string }

func (m *mockSlashSlashing) Jail(_ sdk.Context, consAddr sdk.ConsAddress) {
	m.jailed = append(m.jailed, consAddr.String())
}

// ---------------------------------------------------------------------------

func newSlashSplitKeeper(t *testing.T, bank *mockSlashBank, stk *mockSlashStaking, slk *mockSlashSlashing) (*keeper.Keeper, sdk.Context) {
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

	k := keeper.NewKeeper(cdc, storeKey, memStoreKey, paramsSubspace, bank, stk, slk)
	ctx := sdk.NewContext(stateStore, tmproto.Header{}, false, log.NewNopLogger())
	k.SetParams(ctx, types.DefaultParams())
	return k, ctx
}

// TestSlashSplit40Burn60Security 锁定白皮书《优化定稿版》§24.4 的罚没拆分口径：
// 对 bonded 验证人罚没时，被罚自质押 40% 打入黑洞地址（永久销毁），
// 60% 转入质押安全池（补贴诚实节点），两者之和恰等于被罚总额（零新印、零漏损）。
func TestSlashSplit40Burn60Security(t *testing.T) {
	consPub := ed25519.GenPrivKey().PubKey()
	valAddr := sdk.ValAddress(consPub.Address())
	// VALADDR-1：phonenode 登记的是**账户地址**（mc1...），罚没路径必须能把它
	// 换算成同字节的验证人操作地址。这里刻意用账户地址发起 slash，
	// 复现生产真实调用形态——旧实现在此会静默跳过罚没。
	nodeAddr := sdk.AccAddress(consPub.Address()).String()

	val, err := stakingtypes.NewValidator(valAddr, consPub, stakingtypes.Description{Moniker: "v1"})
	require.NoError(t, err)
	val.Status = stakingtypes.Bonded
	val.Tokens = sdk.NewInt(1000)

	bank := &mockSlashBank{}
	stk := &mockSlashStaking{val: val, hooks: &noopHooks{}}
	slk := &mockSlashSlashing{}
	k, ctx := newSlashSplitKeeper(t, bank, stk, slk)

	// 罚没 20%：1000 × 20% = 200 → 销毁 80（40%）+ 安全池 120（60%）
	require.NoError(t, k.SlashIfBad(ctx, nodeAddr, "double_sign", 2000))

	// 1) distribution 钩子必须在扣减 tokens 前被调用（否则委托人可超额提取历史奖励）
	require.Equal(t, 1, stk.hooks.slashedCalls, "BeforeValidatorSlashed 必须被调用一次")

	// 2) 验证人 tokens 扣减 = 被罚总额
	require.Equal(t, int64(200), stk.removed.Int64(), "应扣减 20% 自质押")

	// 3) 40% 打入黑洞地址（销毁）
	require.Len(t, bank.modToAcct, 1, "应有且仅有一笔销毁转账")
	require.Equal(t, stakingtypes.BondedPoolName, bank.modToAcct[0].from, "销毁份额必须从 bonded pool 扣")
	require.Equal(t, tokenomicstypes.BlackHoleAddress().String(), bank.modToAcct[0].to, "销毁去向必须是黑洞地址")
	require.Equal(t, int64(80), bank.modToAcct[0].amount, "销毁份额应为 40% = 80")

	// 4) 60% 回流质押安全池
	require.Len(t, bank.modToMod, 1, "应有且仅有一笔安全池回流")
	require.Equal(t, stakingtypes.BondedPoolName, bank.modToMod[0].from)
	require.Equal(t, tokenomicstypes.StakingSecurityPoolName, bank.modToMod[0].to)
	require.Equal(t, int64(120), bank.modToMod[0].amount, "回流份额应为 60% = 120")

	// 5) 零漏损：销毁 + 回流 == 被罚总额（bonded pool 出账与验证人 tokens 扣减等额，
	//    staking ModuleAccountInvariants 恒成立）
	require.Equal(t, stk.removed.Int64(), bank.modToAcct[0].amount+bank.modToMod[0].amount)

	// 6) 作恶验证人被 Jail
	require.Len(t, slk.jailed, 1, "作恶验证人应被 jail")
}

// TestSlashSplitRatioConstants 锁定拆分比例常量本身，防止被静默改动。
func TestSlashSplitRatioConstants(t *testing.T) {
	require.Equal(t, uint32(4000), tokenomicstypes.SlashBurnRatioBps, "罚没销毁比例应为 40%")
	require.Equal(t, uint32(6000), tokenomicstypes.SlashSecurityRatioBps, "罚没回流比例应为 60%")
	require.Equal(t, uint32(10000), tokenomicstypes.SlashBurnRatioBps+tokenomicstypes.SlashSecurityRatioBps,
		"两段比例之和必须为 100%，不得出现漏损或超发")
}
