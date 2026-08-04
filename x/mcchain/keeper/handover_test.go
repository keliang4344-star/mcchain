package keeper_test

import (
	"testing"

	tmdb "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/libs/log"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/store"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	typesparams "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/stretchr/testify/require"

	"mcchain/x/mcchain/keeper"
	"mcchain/x/mcchain/types"
)

// setupHandoverKeeper 内联构建 mcchain keeper（内存 DB + 独立 store key），
// 与 x/phonenode 的测试脚手架保持同一模式，避免测试间状态串扰。
func setupHandoverKeeper(t testing.TB) (*keeper.Keeper, sdk.Context) {
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

	paramsSubspace := typesparams.NewSubspace(cdc,
		types.Amino,
		storeKey,
		memStoreKey,
		"McchainParams",
	)
	k := keeper.NewKeeper(cdc, storeKey, memStoreKey, paramsSubspace)

	ctx := sdk.NewContext(stateStore, tmproto.Header{}, false, log.NewNopLogger())
	k.SetParams(ctx, types.DefaultParams())

	return k, ctx
}

// newGovernorAddr 生成一个合法 bech32 地址作为新治理主体。
func newGovernorAddr() string {
	return sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
}

func TestDefaultGovernanceHandoverConfig(t *testing.T) {
	k, ctx := setupHandoverKeeper(t)

	cfg := k.GetGovernanceHandoverConfig(ctx)
	require.False(t, cfg.Enabled)
	require.EqualValues(t, 43200, cfg.TimelockBlocks)
	require.EqualValues(t, 3, cfg.RequiredSigners)
	require.Empty(t, cfg.NewGovernor)
	require.EqualValues(t, 0, cfg.ActivationHeight)
	require.False(t, cfg.Executed)
	require.False(t, k.IsHandoverPending(ctx))
	require.False(t, k.IsHandoverComplete(ctx))
}

// 默认禁用时，发起移交必须失败。
func TestInitiateHandoverDisabled(t *testing.T) {
	k, ctx := setupHandoverKeeper(t)

	require.Error(t, k.InitiateHandover(ctx, newGovernorAddr()))
	require.False(t, k.IsHandoverPending(ctx))
}

// 启用后，非法 bech32 地址必须被拒绝。
func TestInitiateHandoverInvalidAddress(t *testing.T) {
	k, ctx := setupHandoverKeeper(t)

	cfg := k.GetGovernanceHandoverConfig(ctx)
	cfg.Enabled = true
	k.SetGovernanceHandoverConfig(ctx, cfg)

	require.Error(t, k.InitiateHandover(ctx, "not-a-bech32-address"))
	require.False(t, k.IsHandoverPending(ctx))
}

// 完整流程：启用 -> 发起（设置 ActivationHeight）-> 时间锁内失败 -> 越过后成功。
func TestHandoverTimelockFlow(t *testing.T) {
	k, ctx := setupHandoverKeeper(t)

	cfg := k.GetGovernanceHandoverConfig(ctx)
	cfg.Enabled = true
	cfg.TimelockBlocks = 100
	k.SetGovernanceHandoverConfig(ctx, cfg)

	startHeight := int64(10)
	ctx = ctx.WithBlockHeight(startHeight)

	gov := newGovernorAddr()
	require.NoError(t, k.InitiateHandover(ctx, gov))

	got := k.GetGovernanceHandoverConfig(ctx)
	require.Equal(t, gov, got.NewGovernor)
	require.Equal(t, startHeight+100, got.ActivationHeight)
	require.False(t, got.Executed)
	require.True(t, k.IsHandoverPending(ctx))
	require.False(t, k.IsHandoverComplete(ctx))

	// 时间锁未到：执行必须失败，且状态不变。
	beforeCtx := ctx.WithBlockHeight(got.ActivationHeight - 1)
	require.Error(t, k.CompleteHandover(beforeCtx))
	require.False(t, k.IsHandoverComplete(beforeCtx))

	// 越过生效高度：执行成功并进入终态。
	afterCtx := ctx.WithBlockHeight(got.ActivationHeight)
	require.NoError(t, k.CompleteHandover(afterCtx))
	require.True(t, k.IsHandoverComplete(afterCtx))
	require.False(t, k.IsHandoverPending(afterCtx))

	final := k.GetGovernanceHandoverConfig(afterCtx)
	require.True(t, final.Executed)
	require.Equal(t, gov, final.NewGovernor)

	// 已执行后再次调用为幂等空操作。
	require.NoError(t, k.CompleteHandover(afterCtx))
}

func TestHandoverEventsEmitted(t *testing.T) {
	k, ctx := setupHandoverKeeper(t)

	cfg := k.GetGovernanceHandoverConfig(ctx)
	cfg.Enabled = true
	cfg.TimelockBlocks = 5
	k.SetGovernanceHandoverConfig(ctx, cfg)

	ctx = ctx.WithBlockHeight(1)
	gov := newGovernorAddr()
	require.NoError(t, k.InitiateHandover(ctx, gov))
	require.True(t, hasEvent(ctx, "mcchain.GovernanceHandoverInitiated"))

	doneCtx := ctx.WithBlockHeight(6)
	require.NoError(t, k.CompleteHandover(doneCtx))
	require.True(t, hasEvent(doneCtx, "mcchain.GovernanceHandoverCompleted"))
}

func hasEvent(ctx sdk.Context, typ string) bool {
	for _, ev := range ctx.EventManager().Events() {
		if ev.Type == typ {
			return true
		}
	}
	return false
}
