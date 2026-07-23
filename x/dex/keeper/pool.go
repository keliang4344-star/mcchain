package keeper

import (
	"encoding/binary"
	"encoding/json"
	"sort"

	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/dex/types"
)

var (
	PoolKeyPrefix     = []byte{0x01}
	NextPoolIDKey     = []byte{0x02}
)

func (k Keeper) GetNextPoolID(ctx sdk.Context) uint64 {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(NextPoolIDKey)
	if bz == nil {
		return 1
	}
	return binary.BigEndian.Uint64(bz)
}

func (k Keeper) SetNextPoolID(ctx sdk.Context, id uint64) {
	store := ctx.KVStore(k.storeKey)
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, id)
	store.Set(NextPoolIDKey, bz)
}

func (k Keeper) GetPool(ctx sdk.Context, poolID uint64) (types.Pool, bool) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), PoolKeyPrefix)
	bz := store.Get(sdk.Uint64ToBigEndian(poolID))
	if bz == nil {
		return types.Pool{}, false
	}

	var pool types.Pool
	if err := json.Unmarshal(bz, &pool); err != nil {
		panic(err)
	}
	return pool, true
}

func (k Keeper) SetPool(ctx sdk.Context, pool types.Pool) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), PoolKeyPrefix)
	bz, err := json.Marshal(pool)
	if err != nil {
		panic(err)
	}
	store.Set(sdk.Uint64ToBigEndian(pool.Id), bz)
}

func (k Keeper) GetAllPools(ctx sdk.Context) []types.Pool {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), PoolKeyPrefix)
	iter := store.Iterator(nil, nil)
	defer iter.Close()

	var pools []types.Pool
	for ; iter.Valid(); iter.Next() {
		var pool types.Pool
		if err := json.Unmarshal(iter.Value(), &pool); err != nil {
			continue
		}
		pools = append(pools, pool)
	}

	return pools
}

// CreatePool creates a new liquidity pool.
// If poolID is 0, auto-increments the next pool ID.
// Returns the created pool and its ID.
func (k Keeper) CreatePool(
	ctx sdk.Context,
	denomA, denomB string,
	amountA, amountB sdk.Int,
	feeRateBps uint32,
	creator string,
	poolID uint64,
) (types.Pool, error) {
	params := k.GetParams(ctx)

	// Validate denoms are sorted alphabetically
	denoms := []string{denomA, denomB}
	sort.Strings(denoms)
	if denoms[0] != denomA || denoms[1] != denomB {
		return types.Pool{}, types.ErrDenomSortRequired
	}

	// Determine pool ID
	if poolID == 0 {
		poolID = k.GetNextPoolID(ctx)
	}

	// Check max pools
	if poolID > params.MaxPools {
		return types.Pool{}, types.ErrMaxPoolsReached
	}

	// Check pool doesn't already exist
	if _, found := k.GetPool(ctx, poolID); found {
		return types.Pool{}, types.ErrInvalidPoolID
	}

	if feeRateBps == 0 {
		feeRateBps = params.DefaultFeeRateBps
	}

	pool := types.Pool{
		Id:          poolID,
		DenomA:      denoms[0],
		DenomB:      denoms[1],
		ReserveA:    amountA.String(),
		ReserveB:    amountB.String(),
		TotalLp:     "0",
		FeeRateBps:  feeRateBps,
		Owner:       creator,
	}

	// Calculate initial LP tokens
	reserveA := amountA
	reserveB := amountB
	lpMinted, _, _ := CalcAddLiquidity(reserveA, reserveB, amountA, amountB, sdk.ZeroInt())
	pool.TotalLp = lpMinted.String()

	// Transfer assets from creator to dex module
	creatorAddr, err := sdk.AccAddressFromBech32(creator)
	if err != nil {
		return types.Pool{}, err
	}

	coinsA := sdk.NewCoins(sdk.NewCoin(denoms[0], amountA))
	if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, creatorAddr, types.ModuleName, coinsA); err != nil {
		return types.Pool{}, err
	}

	coinsB := sdk.NewCoins(sdk.NewCoin(denoms[1], amountB))
	if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, creatorAddr, types.ModuleName, coinsB); err != nil {
		return types.Pool{}, err
	}

	// Mint LP tokens to creator
	lpDenom := types.PoolDenom(poolID)
	lpCoins := sdk.NewCoins(sdk.NewCoin(lpDenom, lpMinted))
	if err := k.bankKeeper.MintCoins(ctx, types.ModuleName, lpCoins); err != nil {
		return types.Pool{}, err
	}
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, creatorAddr, lpCoins); err != nil {
		return types.Pool{}, err
	}

	// Store pool
	k.SetPool(ctx, pool)

	// Increment next pool ID
	k.SetNextPoolID(ctx, poolID+1)

	return pool, nil
}
