package keeper

import (
	"encoding/binary"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/dex/types"
)

// SetLiquidityLock stores a liquidity lock position for an LP.
func (k Keeper) SetLiquidityLock(ctx sdk.Context, lock types.LiquidityLock) {
	store := ctx.KVStore(k.storeKey)
	key := types.LiquidityLockKey(lock.LpAddress, lock.PoolId)

	bz := make([]byte, 0, 32)
	bz = binary.BigEndian.AppendUint64(bz, lock.LockHeight)
	bz = binary.BigEndian.AppendUint64(bz, lock.UnlockHeight)
	// Append lp_amount as length-prefixed string
	amountBytes := []byte(lock.LpAmount)
	bz = binary.BigEndian.AppendUint16(bz, uint16(len(amountBytes)))
	bz = append(bz, amountBytes...)

	store.Set(key, bz)
}

// GetLiquidityLock retrieves a liquidity lock position.
// Returns the lock and true if found, zero value and false otherwise.
func (k Keeper) GetLiquidityLock(ctx sdk.Context, lpAddress string, poolID uint64) (types.LiquidityLock, bool) {
	store := ctx.KVStore(k.storeKey)
	key := types.LiquidityLockKey(lpAddress, poolID)

	bz := store.Get(key)
	if len(bz) < 18 {
		return types.LiquidityLock{}, false
	}

	lockHeight := binary.BigEndian.Uint64(bz[0:8])
	unlockHeight := binary.BigEndian.Uint64(bz[8:16])
	amountLen := binary.BigEndian.Uint16(bz[16:18])
	if int(amountLen)+18 > len(bz) {
		return types.LiquidityLock{}, false
	}
	lpAmount := string(bz[18 : 18+int(amountLen)])

	return types.LiquidityLock{
		LpAddress:    lpAddress,
		PoolId:       poolID,
		LockHeight:   lockHeight,
		UnlockHeight: unlockHeight,
		LpAmount:     lpAmount,
	}, true
}

// HasActiveLock checks if the LP position is still locked.
// Returns true if a lock exists and the current block height is below unlock height.
func (k Keeper) HasActiveLock(ctx sdk.Context, lpAddress string, poolID uint64) bool {
	lock, found := k.GetLiquidityLock(ctx, lpAddress, poolID)
	if !found {
		return false
	}
	return uint64(ctx.BlockHeight()) < lock.UnlockHeight
}

// DeleteLiquidityLock removes a liquidity lock entry from the store.
func (k Keeper) DeleteLiquidityLock(ctx sdk.Context, lpAddress string, poolID uint64) {
	store := ctx.KVStore(k.storeKey)
	key := types.LiquidityLockKey(lpAddress, poolID)
	store.Delete(key)
}
