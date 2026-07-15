package app

import (
	"testing"

	tmdb "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/libs/log"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/store"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

// testTx is a minimal sdk.Tx implementation that only exposes a fixed list of
// messages. MinSelfDelegationDecorator relies solely on tx.GetMsgs().
type testTx struct {
	msgs []sdk.Msg
}

func (t testTx) GetMsgs() []sdk.Msg { return t.msgs }

// GetMsgsV2 is part of the sdk.Tx interface in cosmos-sdk v0.47.
func (t testTx) GetMsgsV2() []proto.Message {
	out := make([]proto.Message, len(t.msgs))
	for i, m := range t.msgs {
		out[i] = m.(proto.Message)
	}
	return out
}

// ValidateBasic is part of the sdk.Tx interface in cosmos-sdk v0.47.
func (t testTx) ValidateBasic() error { return nil }

// nextRecorder wraps an sdk.AnteHandler that records whether it was invoked.
type nextRecorder struct {
	called bool
}

func (r *nextRecorder) handler() sdk.AnteHandler {
	return func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
		r.called = true
		return ctx, nil
	}
}

// newAnteTestContext builds a minimal (store-backed) sdk.Context sufficient for
// exercising the decorator, which does not read from the store.
func newAnteTestContext(t *testing.T) sdk.Context {
	t.Helper()
	db := tmdb.NewMemDB()
	stateStore := store.NewCommitMultiStore(db)
	stateStore.MountStoreWithDB(sdk.NewKVStoreKey("test"), storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())
	return sdk.NewContext(stateStore, tmproto.Header{}, false, log.NewNopLogger())
}

// P0-1: MsgCreateValidator MinSelfDelegation floor enforcement.
func TestMinSelfDelegationDecorator_CreateValidator(t *testing.T) {
	decorator := MinSelfDelegationDecorator{}
	ctx := newAnteTestContext(t)

	// below the 1e11 umc floor -> rejected before next is called
	rec := &nextRecorder{}
	tx := testTx{msgs: []sdk.Msg{
		&stakingtypes.MsgCreateValidator{MinSelfDelegation: sdk.NewInt(100)},
	}}
	_, err := decorator.AnteHandle(ctx, tx, false, rec.handler())
	require.Error(t, err)
	require.False(t, rec.called)

	// exactly at the floor -> accepted, next invoked
	rec = &nextRecorder{}
	tx = testTx{msgs: []sdk.Msg{
		&stakingtypes.MsgCreateValidator{MinSelfDelegation: sdk.NewInt(MinSelfDelegationLowerBound)},
	}}
	_, err = decorator.AnteHandle(ctx, tx, false, rec.handler())
	require.NoError(t, err)
	require.True(t, rec.called)

	// above the floor -> accepted, next invoked
	rec = &nextRecorder{}
	tx = testTx{msgs: []sdk.Msg{
		&stakingtypes.MsgCreateValidator{MinSelfDelegation: sdk.NewInt(MinSelfDelegationLowerBound + 1)},
	}}
	_, err = decorator.AnteHandle(ctx, tx, false, rec.handler())
	require.NoError(t, err)
	require.True(t, rec.called)
}

// P0-1: MsgEditValidator MinSelfDelegation floor enforcement + zero-value skip.
func TestMinSelfDelegationDecorator_EditValidator(t *testing.T) {
	decorator := MinSelfDelegationDecorator{}
	ctx := newAnteTestContext(t)

	// explicit value below the floor -> rejected
	rec := &nextRecorder{}
	lowMsd := sdk.NewInt(1)
	tx := testTx{msgs: []sdk.Msg{
		&stakingtypes.MsgEditValidator{MinSelfDelegation: &lowMsd},
	}}
	_, err := decorator.AnteHandle(ctx, tx, false, rec.handler())
	require.Error(t, err)
	require.False(t, rec.called)

	// explicit value at the floor -> accepted
	rec = &nextRecorder{}
	floorMsd := sdk.NewInt(MinSelfDelegationLowerBound)
	tx = testTx{msgs: []sdk.Msg{
		&stakingtypes.MsgEditValidator{MinSelfDelegation: &floorMsd},
	}}
	_, err = decorator.AnteHandle(ctx, tx, false, rec.handler())
	require.NoError(t, err)
	require.True(t, rec.called)

	// zero / unset MinSelfDelegation means "do not change" -> skip, accepted
	rec = &nextRecorder{}
	tx = testTx{msgs: []sdk.Msg{
		&stakingtypes.MsgEditValidator{MinSelfDelegation: nil},
	}}
	_, err = decorator.AnteHandle(ctx, tx, false, rec.handler())
	require.NoError(t, err)
	require.True(t, rec.called)
}

// P0-1: tx with no relevant messages must pass straight through to next.
func TestMinSelfDelegationDecorator_Passthrough(t *testing.T) {
	decorator := MinSelfDelegationDecorator{}
	ctx := newAnteTestContext(t)

	rec := &nextRecorder{}
	tx := testTx{msgs: []sdk.Msg{}}
	_, err := decorator.AnteHandle(ctx, tx, false, rec.handler())
	require.NoError(t, err)
	require.True(t, rec.called)
}
