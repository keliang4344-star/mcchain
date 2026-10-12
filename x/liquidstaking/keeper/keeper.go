package keeper

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/cometbft/cometbft/libs/log"
	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/liquidstaking/types"
)

// Keeper owns the liquid staking state machine.
//
// State is persisted as JSON-encoded Go structs in the module KVStore. The
// module deliberately introduces no new protobuf types: it composes existing
// x/staking, x/bank and x/distribution behaviour instead of defining a new
// transaction surface.
type Keeper struct {
	cdc        codec.BinaryCodec
	storeKey   storetypes.StoreKey
	authority  string
	accountKpr types.AccountKeeper
	bankKpr    types.BankKeeper
	stakingKpr types.StakingKeeper
	distrKpr   types.DistributionKeeper
}

// NewKeeper builds the liquid staking keeper.
func NewKeeper(
	cdc codec.BinaryCodec,
	storeKey storetypes.StoreKey,
	authority string,
	accountKpr types.AccountKeeper,
	bankKpr types.BankKeeper,
	stakingKpr types.StakingKeeper,
	distrKpr types.DistributionKeeper,
) *Keeper {
	return &Keeper{
		cdc:        cdc,
		storeKey:   storeKey,
		authority:  authority,
		accountKpr: accountKpr,
		bankKpr:    bankKpr,
		stakingKpr: stakingKpr,
		distrKpr:   distrKpr,
	}
}

// Logger returns a module-scoped logger.
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// Authority returns the address allowed to change module params.
func (k Keeper) Authority() string { return k.authority }

// ModuleAddress returns the module account that holds and delegates the pooled MC.
func (k Keeper) ModuleAddress() sdk.AccAddress {
	return k.accountKpr.GetModuleAddress(types.ModuleName)
}

// ---------------------------------------------------------------------------
// Params
// ---------------------------------------------------------------------------

// GetParams reads module params, falling back to defaults before genesis writes.
func (k Keeper) GetParams(ctx sdk.Context) types.Params {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.ParamsKey)
	if len(bz) == 0 {
		return types.DefaultParams()
	}
	var p types.Params
	if err := json.Unmarshal(bz, &p); err != nil {
		// fail-closed：参数损坏时回退默认值会静默改变费率/上限等治理参数，
		// 让链上行为与治理决议脱节且无人察觉。panic 使损坏在读取处即暴露。
		panic(fmt.Sprintf("liquidstaking: unmarshal params: %v", err))
	}
	return p
}

// SetParams validates and persists module params.
func (k Keeper) SetParams(ctx sdk.Context, p types.Params) error {
	if err := p.Validate(); err != nil {
		return err
	}
	bz, err := json.Marshal(p)
	if err != nil {
		return err
	}
	ctx.KVStore(k.storeKey).Set(types.ParamsKey, bz)
	return nil
}

// ---------------------------------------------------------------------------
// Reward-compounding sweep cursor (P1-06)
// ---------------------------------------------------------------------------

// getRewardsCursor reads the persistent reward-compounding sweep cursor.
// Missing/empty cursor (fresh chain) is 0.
func (k Keeper) getRewardsCursor(ctx sdk.Context) uint64 {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.RewardsCursorKey)
	if len(bz) == 0 {
		return 0
	}
	if len(bz) != 8 {
		// fail-closed：游标损坏时返回 0 只会导致一轮重复扫描（幂等），
		// 但清零重扫总好过把位置推进到错误的越界值造成跳过 validator。
		panic(fmt.Sprintf("liquidstaking: rewards cursor corrupted (len=%d)", len(bz)))
	}
	return binary.BigEndian.Uint64(bz)
}

// setRewardsCursor persists the reward-compounding sweep cursor.
func (k Keeper) setRewardsCursor(ctx sdk.Context, idx uint64) {
	store := ctx.KVStore(k.storeKey)
	var bz [8]byte
	binary.BigEndian.PutUint64(bz[:], idx)
	store.Set(types.RewardsCursorKey, bz[:])
}

// ---------------------------------------------------------------------------
// Pool state
// ---------------------------------------------------------------------------

// GetPoolState reads the aggregate bonded/share accounting.
func (k Keeper) GetPoolState(ctx sdk.Context) types.PoolState {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.PoolStateKey)
	if len(bz) == 0 {
		return types.PoolState{}
	}
	var ps types.PoolState
	if err := json.Unmarshal(bz, &ps); err != nil {
		// fail-closed：聚合账本损坏时返回空状态会让下游把已抵押/已发行
		// 的份额当作零，导致赎回/计息错账。panic 使损坏在读取处即暴露。
		panic(fmt.Sprintf("liquidstaking: unmarshal pool state: %v", err))
	}
	return ps
}

// SetPoolState persists the aggregate accounting.
func (k Keeper) SetPoolState(ctx sdk.Context, ps types.PoolState) {
	bz, err := json.Marshal(ps)
	if err != nil {
		panic(fmt.Sprintf("liquidstaking: marshal pool state: %v", err))
	}
	ctx.KVStore(k.storeKey).Set(types.PoolStateKey, bz)
}

// ---------------------------------------------------------------------------
// Validator bond tracking
// ---------------------------------------------------------------------------

// GetValidatorBond returns how much umc this module delegated to one validator.
func (k Keeper) GetValidatorBond(ctx sdk.Context, valAddr string) uint64 {
	bz := ctx.KVStore(k.storeKey).Get(types.ValidatorBondKey(valAddr))
	if len(bz) == 0 {
		return 0
	}
	var vb types.ValidatorBond
	if err := json.Unmarshal(bz, &vb); err != nil {
		return 0
	}
	return vb.AmountUmc
}

func (k Keeper) setValidatorBond(ctx sdk.Context, valAddr string, amount uint64) {
	store := ctx.KVStore(k.storeKey)
	if amount == 0 {
		store.Delete(types.ValidatorBondKey(valAddr))
		return
	}
	bz, err := json.Marshal(types.ValidatorBond{Validator: valAddr, AmountUmc: amount})
	if err != nil {
		panic(fmt.Sprintf("liquidstaking: marshal validator bond: %v", err))
	}
	store.Set(types.ValidatorBondKey(valAddr), bz)
}

// AllValidatorBonds returns every tracked validator bond.
func (k Keeper) AllValidatorBonds(ctx sdk.Context) []types.ValidatorBond {
	store := ctx.KVStore(k.storeKey)
	it := storetypes.KVStorePrefixIterator(store, types.ValidatorBondKeyPrefix)
	defer it.Close()

	out := []types.ValidatorBond{}
	for ; it.Valid(); it.Next() {
		var vb types.ValidatorBond
		if err := json.Unmarshal(it.Value(), &vb); err != nil {
			continue
		}
		out = append(out, vb)
	}
	return out
}

// ---------------------------------------------------------------------------
// Unbonding queue
// ---------------------------------------------------------------------------

// NextUnbondingID reads the next unbonding receipt id (1-based).
func (k Keeper) NextUnbondingID(ctx sdk.Context) uint64 {
	bz := ctx.KVStore(k.storeKey).Get(types.NextUnbondingIDKey)
	if len(bz) == 0 {
		return 1
	}
	return sdk.BigEndianToUint64(bz)
}

func (k Keeper) setNextUnbondingID(ctx sdk.Context, id uint64) {
	ctx.KVStore(k.storeKey).Set(types.NextUnbondingIDKey, sdk.Uint64ToBigEndian(id))
}

// SetUnbondingEntry persists one redemption receipt.
func (k Keeper) SetUnbondingEntry(ctx sdk.Context, e types.UnbondingEntry) {
	bz, err := json.Marshal(e)
	if err != nil {
		panic(fmt.Sprintf("liquidstaking: marshal unbonding entry: %v", err))
	}
	ctx.KVStore(k.storeKey).Set(types.UnbondingEntryKey(e.Delegator, e.ID), bz)
}

// GetUnbondingEntry loads one redemption receipt.
func (k Keeper) GetUnbondingEntry(ctx sdk.Context, delegator string, id uint64) (types.UnbondingEntry, bool) {
	bz := ctx.KVStore(k.storeKey).Get(types.UnbondingEntryKey(delegator, id))
	if len(bz) == 0 {
		return types.UnbondingEntry{}, false
	}
	var e types.UnbondingEntry
	if err := json.Unmarshal(bz, &e); err != nil {
		return types.UnbondingEntry{}, false
	}
	return e, true
}

// GetDelegatorUnbondings returns every receipt owned by one delegator.
func (k Keeper) GetDelegatorUnbondings(ctx sdk.Context, delegator string) []types.UnbondingEntry {
	store := ctx.KVStore(k.storeKey)
	it := storetypes.KVStorePrefixIterator(store, types.UnbondingDelegatorPrefix(delegator))
	defer it.Close()

	out := []types.UnbondingEntry{}
	for ; it.Valid(); it.Next() {
		var e types.UnbondingEntry
		if err := json.Unmarshal(it.Value(), &e); err != nil {
			continue
		}
		out = append(out, e)
	}
	return out
}

// AllUnbondings returns the whole redemption queue (genesis export / queries).
func (k Keeper) AllUnbondings(ctx sdk.Context) []types.UnbondingEntry {
	store := ctx.KVStore(k.storeKey)
	it := storetypes.KVStorePrefixIterator(store, types.UnbondingEntryKeyPrefix)
	defer it.Close()

	out := []types.UnbondingEntry{}
	for ; it.Valid(); it.Next() {
		var e types.UnbondingEntry
		if err := json.Unmarshal(it.Value(), &e); err != nil {
			continue
		}
		out = append(out, e)
	}
	return out
}
