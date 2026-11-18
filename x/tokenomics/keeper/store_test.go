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
	"github.com/stretchr/testify/require"

	"mcchain/x/tokenomics/types"
)

// newStoreTestKeeper 构造一个仅用于 store 读写 fail-closed 测试的 keeper。
// account/bank keeper 传 nil：被测的 GetMintedSupply / GetReleaseSchedule /
// GetAllocations 都不触碰这两个依赖。
func newStoreTestKeeper(t *testing.T) (*Keeper, sdk.Context) {
	storeKey := sdk.NewKVStoreKey(types.StoreKey)

	db := tmdb.NewMemDB()
	ms := store.NewCommitMultiStore(db)
	ms.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, ms.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	ctx := sdk.NewContext(ms, tmproto.Header{}, false, log.NewNopLogger())

	return NewKeeper(cdc, storeKey, nil, nil), ctx
}

// 读取账本状态 fail-closed：状态存在但无法解析属账本损坏，必须 panic 而非
// 静默返回零值，否则 MintCoins 会把累计已发行量当作 0，从而绕过总量硬顶。

func TestGetMintedSupplyAbsentReturnsZero(t *testing.T) {
	k, ctx := newStoreTestKeeper(t)
	require.Equal(t, sdk.ZeroInt(), k.GetMintedSupply(ctx))
}

func TestGetMintedSupplyRoundTrip(t *testing.T) {
	k, ctx := newStoreTestKeeper(t)
	k.SetMintedSupply(ctx, sdk.NewInt(123456))
	require.Equal(t, sdk.NewInt(123456), k.GetMintedSupply(ctx))
}

func TestGetMintedSupplyCorruptedPanics(t *testing.T) {
	k, ctx := newStoreTestKeeper(t)
	ctx.KVStore(k.storeKey).Set(KeyMintedSupply, []byte("not-a-number"))
	require.Panics(t, func() { k.GetMintedSupply(ctx) })
}

func TestGetReleaseScheduleCorruptedPanics(t *testing.T) {
	k, ctx := newStoreTestKeeper(t)
	ctx.KVStore(k.storeKey).Set(KeyReleaseSchedule, []byte("{invalid json"))
	require.Panics(t, func() { k.GetReleaseSchedule(ctx) })
}

func TestGetAllocationsCorruptedPanics(t *testing.T) {
	k, ctx := newStoreTestKeeper(t)
	ctx.KVStore(k.storeKey).Set(KeyAllocations, []byte("{invalid json"))
	require.Panics(t, func() { k.GetAllocations(ctx) })
}
