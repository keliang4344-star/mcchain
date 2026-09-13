package keeper

import (
	"fmt"
	"testing"
	"time"

	tmdb "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/libs/log"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/store"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"mcchain/x/depin/types"
)

// newGCKeeper 构造只带 storeKey 的最小 keeper（GC 路径只碰 KVStore）。
func newGCKeeper(t *testing.T) (Keeper, sdk.Context) {
	t.Helper()
	storeKey := sdk.NewKVStoreKey(types.StoreKey)
	db := tmdb.NewMemDB()
	cms := store.NewCommitMultiStore(db)
	cms.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, cms.LoadLatestVersion())
	ctx := sdk.NewContext(cms, tmproto.Header{ChainID: "gc-test", Height: 1000}, false, log.NewNopLogger())
	ctx = ctx.WithBlockTime(time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC))
	return Keeper{storeKey: storeKey}, ctx
}

// TestGCSuffixedDateKeysBudgetCountsScannedNotDeleted 是容量预算口径的回归闸门。
//
// 回归守则：循环上界不能用「已删除条数」计（len(deleteKeys) < budget），
// 未过期键走 continue 不消耗预算。稳态下（几乎所有键都未过期）这个循环会一路
// 扫到前缀末尾 —— 单块工作量与历史实体总量成正比。而 BeginBlock 跑在
// InfiniteGasMeter 下、不受 max_gas 约束，结果是出块超时、全网停块且不可自愈。
//
// 判据：全部写入未过期键，跑一轮 GC，游标必须停在「第 budget 条」处；
// 若退化成整前缀扫描，游标会落在最后一条（= n-1），两者可区分。
func TestGCSuffixedDateKeysBudgetCountsScannedNotDeleted(t *testing.T) {
	k, ctx := newGCKeeper(t)
	store := ctx.KVStore(k.storeKey)

	const today = "20260911" // 与 ctx.BlockTime 同日 ⇒ 全部未过期
	n := DefenseGCPerBlock * 4
	for i := 0; i < n; i++ {
		store.Set(append(append([]byte{}, DefenseDailyPrefix...),
			[]byte(fmt.Sprintf("addr%06d:%s", i, today))...), []byte{1})
	}

	// cutoff 早于 today，因此没有一条会被删。
	k.gcSuffixedDateKeys(ctx, DefenseDailyPrefix, 8, "20260909")

	cursor := store.Get(append(append([]byte{}, defenseGCCursorKey...), DefenseDailyPrefix...))
	require.NotNil(t, cursor, "未扫到预算耗尽就结束说明循环没有按扫描条数收敛")
	require.Equal(t, fmt.Sprintf("addr%06d:%s", DefenseGCPerBlock, today), string(cursor),
		"游标必须停在预算耗尽处（第 %d 条），落在末条说明已退化为整前缀扫描（停链级回归）", DefenseGCPerBlock)

	// 未过期键一条都不能被删。
	for i := 0; i < n; i++ {
		key := append(append([]byte{}, DefenseDailyPrefix...), []byte(fmt.Sprintf("addr%06d:%s", i, today))...)
		require.NotNil(t, store.Get(key), "第 %d 条未过期却被删除", i)
	}
}

// TestGCSuffixedDateKeysDeletesExpiredWithinBudget 证明过期键确实会被回收，
// 且单轮删除量同样受预算约束（不影响正确性，只是限速）。
func TestGCSuffixedDateKeysDeletesExpiredWithinBudget(t *testing.T) {
	k, ctx := newGCKeeper(t)
	store := ctx.KVStore(k.storeKey)

	const stale = "20200101"
	n := DefenseGCPerBlock * 2
	for i := 0; i < n; i++ {
		store.Set(append(append([]byte{}, DefenseDailyPrefix...),
			[]byte(fmt.Sprintf("addr%06d:%s", i, stale))...), []byte{1})
	}

	k.gcSuffixedDateKeys(ctx, DefenseDailyPrefix, 8, "20260909")

	deleted := 0
	for i := 0; i < n; i++ {
		key := append(append([]byte{}, DefenseDailyPrefix...), []byte(fmt.Sprintf("addr%06d:%s", i, stale))...)
		if store.Get(key) == nil {
			deleted++
		}
	}
	require.Equal(t, DefenseGCPerBlock, deleted, "单轮删除量不得超过预算")
}
