package keeper

import (
	"testing"

	tmdb "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/libs/log"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/store"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	typesparams "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/stretchr/testify/require"

	"mcchain/x/edgeai/types"
)

// ---------------------------------------------------------------------------
// Verifier-enabled mock phonenode
// ---------------------------------------------------------------------------

// mockPhonenodeFull is a full mock that also supports GetVerifierNodes for
// cross-module verifier sampling tests. It embeds mockPhonenode behaviour.
type mockPhonenodeFull struct {
	slashed       []string
	verifierNodes []string
}

func (m *mockPhonenodeFull) HasNode(ctx sdk.Context, addr string) bool    { return true }
func (m *mockPhonenodeFull) IsAttested(ctx sdk.Context, addr string) bool { return true }
func (m *mockPhonenodeFull) SlashIfBad(ctx sdk.Context, addr, reason string, bps uint32) error {
	m.slashed = append(m.slashed, addr)
	return nil
}
func (m *mockPhonenodeFull) GetVerifierNodes(ctx sdk.Context) []string {
	return m.verifierNodes
}

// ---------------------------------------------------------------------------
// Shared test setup
// ---------------------------------------------------------------------------
//
// setupEdgeai and addrOf are defined in resolve_dispute_test.go — no need to
// redefine them here. This file adds only the "full" (verifier-aware) variants.

// setupEdgeaiFull constructs an edgeai keeper with a mockPhonenodeFull that
// supports GetVerifierNodes for verifier sampling tests.
func setupEdgeaiFull(t *testing.T, verifierNodes []string) (*Keeper, sdk.Context, *mockPhonenodeFull) {
	storeKey := sdk.NewKVStoreKey(types.StoreKey)
	memKey := storetypes.NewMemoryStoreKey(types.MemStoreKey)
	db := tmdb.NewMemDB()
	cs := store.NewCommitMultiStore(db)
	cs.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	cs.MountStoreWithDB(memKey, storetypes.StoreTypeMemory, nil)
	require.NoError(t, cs.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	ps := typesparams.NewSubspace(cdc, types.Amino, storeKey, memKey, "EdgeaiParams")

	m := &mockPhonenodeFull{verifierNodes: verifierNodes}
	k := NewKeeper(cdc, storeKey, memKey, ps, m, mockBank{}, nil)
	ctx := sdk.NewContext(cs, tmproto.Header{}, false, log.NewNopLogger())
	k.SetParams(ctx, types.DefaultParams())
	return k, ctx, m
}

// setupEdgeaiWithBankFull constructs an edgeai keeper with both verifier support
// and a configurable bank keeper for escrow/payout tracking.
func setupEdgeaiWithBankFull(t *testing.T, verifierNodes []string, bk types.BankKeeper) (*Keeper, sdk.Context, *mockPhonenodeFull) {
	storeKey := sdk.NewKVStoreKey(types.StoreKey)
	memKey := storetypes.NewMemoryStoreKey(types.MemStoreKey)
	db := tmdb.NewMemDB()
	cs := store.NewCommitMultiStore(db)
	cs.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	cs.MountStoreWithDB(memKey, storetypes.StoreTypeMemory, nil)
	require.NoError(t, cs.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	ps := typesparams.NewSubspace(cdc, types.Amino, storeKey, memKey, "EdgeaiParams")

	m := &mockPhonenodeFull{verifierNodes: verifierNodes}
	k := NewKeeper(cdc, storeKey, memKey, ps, m, bk, nil)
	ctx := sdk.NewContext(cs, tmproto.Header{}, false, log.NewNopLogger())
	k.SetParams(ctx, types.DefaultParams())
	return k, ctx, m
}

// advanceBlocks returns a new context with block height incremented by n.
func advanceBlocks(ctx sdk.Context, n int64) sdk.Context {
	return ctx.WithBlockHeight(ctx.BlockHeight() + n)
}

// quickCreateTask is a helper that calls SetTask directly and advances the
// task counter, bypassing MsgServer for tests that need precise state setup.
func quickCreateTask(t *testing.T, k *Keeper, ctx sdk.Context, id string, creator string, reward uint64, status string, createdAtBlock int64) {
	t.Helper()
	task := &Task{
		Id:             id,
		Creator:        creator,
		Description:    "test task",
		Reward:         reward,
		Status:         status,
		CreatedAt:      ctx.BlockTime().Unix(),
		CreatedAtBlock: createdAtBlock,
	}
	require.NoError(t, k.SetTask(ctx, task))
}

// quickCreateResult is a helper to set a result directly.
func quickCreateResult(t *testing.T, k *Keeper, ctx sdk.Context, taskID, submitter, resultHash, status string, submittedAtBlock int64) {
	t.Helper()
	r := &Result{
		TaskId:           taskID,
		Submitter:        submitter,
		ResultHash:       resultHash,
		Status:           status,
		SubmittedAt:      ctx.BlockTime().Unix(),
		SubmittedAtBlock: submittedAtBlock,
	}
	require.NoError(t, k.SetResult(ctx, r))
}
