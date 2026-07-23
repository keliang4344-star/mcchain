package keeper

import (
	"encoding/binary"
	"fmt"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/referral/types"
)

// dailyCapKey builds a per-user daily cap key suffixed with current block height → day.
func dailyCapKey(perUserKey string, ctx sdk.Context) []byte {
	day := uint64(ctx.BlockHeight()) / uint64(BlockDayDivisor())
	return append([]byte(perUserKey), uint64Bytes(day)...)
}

// BlockDayDivisor returns the number of blocks per "day". Overridable for testing.
// Default assumes 6s blocks → 14,400 blocks/day.
func BlockDayDivisor() int64 {
	return 14400
}

// ---- per-user daily cap ----

func (k Keeper) getDailyPerUser(ctx sdk.Context, user string) uint64 {
	store := ctx.KVStore(k.storeKey)
	key := dailyCapKey(types.DailyPerUserCapKeyPrefix+user, ctx)
	bz := store.Get(key)
	if bz == nil {
		return 0
	}
	return binary.BigEndian.Uint64(bz)
}

func (k Keeper) setDailyPerUser(ctx sdk.Context, user string, amount uint64) {
	store := ctx.KVStore(k.storeKey)
	key := dailyCapKey(types.DailyPerUserCapKeyPrefix+user, ctx)
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, amount)
	store.Set(key, b)
}

// ---- network-wide daily cap ----

func (k Keeper) getDailyNetwork(ctx sdk.Context) uint64 {
	store := ctx.KVStore(k.storeKey)
	key := dailyCapKey(types.DailyNetworkCapKey, ctx)
	bz := store.Get(key)
	if bz == nil {
		return 0
	}
	return binary.BigEndian.Uint64(bz)
}

func (k Keeper) setDailyNetwork(ctx sdk.Context, amount uint64) {
	store := ctx.KVStore(k.storeKey)
	key := dailyCapKey(types.DailyNetworkCapKey, ctx)
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, amount)
	store.Set(key, b)
}

// ---- cap check (called before TrackReward) ----

// CheckDailyCaps returns an error if the pending bonus would exceed
// either the per-user daily cap or the network-wide daily cap.
//
// bonus is denominated in umc (the smallest unit).
func (k Keeper) CheckDailyCaps(ctx sdk.Context, inviter string, bonus sdkmath.Int) error {
	if bonus.IsZero() {
		return nil
	}
	params := k.GetParams(ctx)

	// Per-user cap
	if params.DailyPerUserCap > 0 {
		used := k.getDailyPerUser(ctx, inviter)
		newUsed := used + bonus.Uint64()
		if newUsed > params.DailyPerUserCap {
			return fmt.Errorf("referral reward exceeds daily-per-user cap: used=%d + bonus=%s > cap=%d (day=%d)",
				used, bonus.String(), params.DailyPerUserCap, uint64(ctx.BlockHeight())/uint64(BlockDayDivisor()))
		}
	}

	// Network cap
	if params.DailyNetworkCap > 0 {
		used := k.getDailyNetwork(ctx)
		newUsed := used + bonus.Uint64()
		if newUsed > params.DailyNetworkCap {
			return fmt.Errorf("referral reward exceeds daily-network cap: used=%d + bonus=%s > cap=%d (day=%d)",
				used, bonus.String(), params.DailyNetworkCap, ctx.BlockHeight()/uint64(BlockDayDivisor()))
		}
	}

	return nil
}

// RecordDailyCapUsage records the bonus against the daily counters.
func (k Keeper) RecordDailyCapUsage(ctx sdk.Context, inviter string, bonus sdkmath.Int) {
	b := bonus.Uint64()
	if b == 0 {
		return
	}
	used := k.getDailyPerUser(ctx, inviter) + b
	k.setDailyPerUser(ctx, inviter, used)

	totalUsed := k.getDailyNetwork(ctx) + b
	k.setDailyNetwork(ctx, totalUsed)
}

// ResetDailyCaps clears all daily cap counters at the day boundary.
// Called in BeginBlock; only performs the sweep on the first block of a new
// "day" (height % BlockDayDivisor == 0), otherwise it is a no-op.
func (k Keeper) ResetDailyCaps(ctx sdk.Context) {
	if ctx.BlockHeight()%BlockDayDivisor() != 0 {
		return
	}
	// The per-user caps are keyed by (prefix + user + day), and the network
	// cap by (prefix + day).  The KV store doesn't support efficient range
	// deletion of arbitrary compound keys easily without prefix scan
	// (given the day is at the end).  The simplest correct implementation
	// is a delete-all sweep.  Because this runs once per block and the
	// number of distinct user-days is bounded by the active users on-chain,
	// the cost is acceptable.
	store := ctx.KVStore(k.storeKey)

	// Delete per-user daily caps
	userStore := prefix.NewStore(store, []byte(types.DailyPerUserCapKeyPrefix))
	it := userStore.Iterator(nil, nil)
	defer it.Close()
	keys := make([][]byte, 0)
	for ; it.Valid(); it.Next() {
		k := make([]byte, len(it.Key()))
		copy(k, it.Key())
		keys = append(keys, k)
	}
	for _, key := range keys {
		userStore.Delete(key)
	}

	// Delete network daily cap
	netStore := prefix.NewStore(store, []byte(types.DailyNetworkCapKey))
	nit := netStore.Iterator(nil, nil)
	defer nit.Close()
	netKeys := make([][]byte, 0)
	for ; nit.Valid(); nit.Next() {
		k := make([]byte, len(nit.Key()))
		copy(k, nit.Key())
		netKeys = append(netKeys, k)
	}
	for _, key := range netKeys {
		netStore.Delete(key)
	}
}
