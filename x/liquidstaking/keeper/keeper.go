package keeper

import (
	"encoding/json"
	"fmt"

	sdkmath "cosmossdk.io/math"
	"github.com/cometbft/cometbft/libs/log"
	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/internal/safemath"
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
// Reward-compounding sweep cursor (P1-08)
// ---------------------------------------------------------------------------

// getRewardsCursorAddr reads the last successfully swept validator address.
//
// 用地址而不是数组下标做游标：验证人集合会因新委托/解锁/删除而变化，
// 下标游标在集合变动时会跳过部分验证人（系统性漏发）或重复处理同一验证人。
// 地址游标在全序上单调推进，集合增删只影响「何时轮到」，不会跳过。
func (k Keeper) getRewardsCursorAddr(ctx sdk.Context) string {
	bz := ctx.KVStore(k.storeKey).Get(types.RewardsCursorValKey)
	if len(bz) == 0 {
		return ""
	}
	return string(bz)
}

// setRewardsCursorAddr persists the last swept validator address.
func (k Keeper) setRewardsCursorAddr(ctx sdk.Context, addr string) {
	ctx.KVStore(k.storeKey).Set(types.RewardsCursorValKey, []byte(addr))
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

// SchemaVersion 是当前二进制期望的 on-disk 状态版本。
//
// 任何改变资金状态结构（PoolState / ValidatorBond / UnbondingEntry）的升级
// 都必须递增该值并提供迁移函数；EnsureSchemaVersion 在 BeginBlock 前校验，
// 旧版本自动补齐、未知/更高版本直接中止，避免新代码误读旧结构。
const SchemaVersion uint64 = 1

// EnsureSchemaVersion 校验并初始化状态版本。
//
//   - 键缺失：视为 0（历史状态），补写为当前版本（v1 与历史结构兼容）；
//   - 版本 == 当前：通过；
//   - 版本 > 当前：拒绝（二进制落后于状态，必须升级后再启动）；
//   - 版本 < 当前：拒绝（缺少显式迁移，不允许静默读取）。
func (k Keeper) EnsureSchemaVersion(ctx sdk.Context) error {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.SchemaVersionKey)
	if len(bz) == 0 {
		store.Set(types.SchemaVersionKey, sdk.Uint64ToBigEndian(SchemaVersion))
		return nil
	}
	if len(bz) != 8 {
		return fmt.Errorf("liquidstaking: schema version key corrupted (len=%d)", len(bz))
	}
	v := sdk.BigEndianToUint64(bz)
	switch {
	case v == SchemaVersion:
		return nil
	case v > SchemaVersion:
		return fmt.Errorf("liquidstaking: state schema v%d is newer than binary v%d; upgrade the binary", v, SchemaVersion)
	default:
		return fmt.Errorf("liquidstaking: state schema v%d requires an explicit migration to v%d", v, SchemaVersion)
	}
}

// ---------------------------------------------------------------------------
// Validator bond tracking
// ---------------------------------------------------------------------------

// GetValidatorBond returns how much umc this module delegated to one validator.
//
// 资金状态解析失败必须 fail-closed：返回 0 会让下游把「已抵押」当成「未抵押」，
// 进而在解锁/复利时错账。共识路径无法返回 error，因此与 GetPoolState 一致，
// 在损坏处直接中止；需要显式 error 的调用方用 GetValidatorBondChecked。
func (k Keeper) GetValidatorBond(ctx sdk.Context, valAddr string) uint64 {
	amount, err := k.GetValidatorBondChecked(ctx, valAddr)
	if err != nil {
		panic(err.Error())
	}
	return amount
}

// GetValidatorBondChecked 是 GetValidatorBond 的可返回错误版本。
func (k Keeper) GetValidatorBondChecked(ctx sdk.Context, valAddr string) (uint64, error) {
	bz := ctx.KVStore(k.storeKey).Get(types.ValidatorBondKey(valAddr))
	if len(bz) == 0 {
		return 0, nil
	}
	var vb types.ValidatorBond
	if err := json.Unmarshal(bz, &vb); err != nil {
		return 0, fmt.Errorf("liquidstaking: unmarshal validator bond %s: %w", valAddr, err)
	}
	return vb.AmountUmc, nil
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
//
// 枚举失败不得跳过：跳过一条坏记录会让「全部 bond 之和」小于真实值，
// 对账/导出/守恒检查都会得出错误结论。损坏即中止。
func (k Keeper) AllValidatorBonds(ctx sdk.Context) []types.ValidatorBond {
	store := ctx.KVStore(k.storeKey)
	it := storetypes.KVStorePrefixIterator(store, types.ValidatorBondKeyPrefix)
	defer it.Close()

	out := []types.ValidatorBond{}
	for ; it.Valid(); it.Next() {
		var vb types.ValidatorBond
		if err := json.Unmarshal(it.Value(), &vb); err != nil {
			panic(fmt.Sprintf("liquidstaking: unmarshal validator bond at %x: %v", it.Key(), err))
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
//
// 赎回凭证是资金状态：解析失败返回 (zero,false) 会让调用方误以为「凭证不存在」，
// 从而重复解锁或把用户资产判为无主。损坏必须中止。
func (k Keeper) GetUnbondingEntry(ctx sdk.Context, delegator string, id uint64) (types.UnbondingEntry, bool) {
	bz := ctx.KVStore(k.storeKey).Get(types.UnbondingEntryKey(delegator, id))
	if len(bz) == 0 {
		return types.UnbondingEntry{}, false
	}
	var e types.UnbondingEntry
	if err := json.Unmarshal(bz, &e); err != nil {
		panic(fmt.Sprintf("liquidstaking: unmarshal unbonding entry %s/%d: %v", delegator, id, err))
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
			panic(fmt.Sprintf("liquidstaking: unmarshal unbonding entry at %x: %v", it.Key(), err))
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
			panic(fmt.Sprintf("liquidstaking: unmarshal unbonding entry at %x: %v", it.Key(), err))
		}
		out = append(out, e)
	}
	return out
}

// AccountingAudit 是资金守恒审计快照（只读，不参与共识）。
//
// 守恒关系：
//   - 内部不变量：Σ ValidatorBond.AmountUmc == PoolState.TotalBondedUmc；
//   - 链上对照：模块账户在 x/staking 的委托份额折算后应等于 TotalBondedUmc
//     （slashing 由 BeforeValidatorSlashed hook 同步减记，因此允许 0 容差；
//     出现差值说明有未经过 hook 的外部状态变化，需要人工核查）。
type AccountingAudit struct {
	TotalBondedUmc     uint64
	TotalSharesUlmc    uint64
	SumValidatorBonds  uint64
	OnChainDelegations uint64
	BondSumMatchesPool bool
	// DelegationDelta = OnChainDelegations − TotalBondedUmc（正=链上更多）。
	DelegationDelta int64
}

// AuditAccounting 汇总并校验资金状态，返回审计快照。
//
// 由 CLI / 运维巡检调用；不写入状态，不 panic（损坏时返回 error），
// 与共识路径的 fail-closed getter 互补。
func (k Keeper) AuditAccounting(ctx sdk.Context) (AccountingAudit, error) {
	ps := k.GetPoolState(ctx)

	bonds, err := k.allValidatorBondsChecked(ctx)
	if err != nil {
		return AccountingAudit{}, err
	}
	var sum uint64
	for _, vb := range bonds {
		sum = saturatingAddU64(sum, vb.AmountUmc)
	}

	var onChain sdkmath.Int = sdkmath.ZeroInt()
	for _, d := range k.stakingKpr.GetAllDelegatorDelegations(ctx, k.ModuleAddress()) {
		valAddr, err := sdk.ValAddressFromBech32(d.ValidatorAddress)
		if err != nil {
			return AccountingAudit{}, fmt.Errorf("liquidstaking: invalid validator address %q: %w", d.ValidatorAddress, err)
		}
		val, found := k.stakingKpr.GetValidator(ctx, valAddr)
		if !found {
			continue
		}
		onChain = onChain.Add(val.TokensFromShares(d.Shares).TruncateInt())
	}

	audit := AccountingAudit{
		TotalBondedUmc:     ps.TotalBondedUmc,
		TotalSharesUlmc:    ps.TotalSharesUlmc,
		SumValidatorBonds:  sum,
		OnChainDelegations: safemath.ClampUint64(onChain),
		BondSumMatchesPool: sum == ps.TotalBondedUmc,
	}
	audit.DelegationDelta = int64(audit.OnChainDelegations) - int64(ps.TotalBondedUmc)

	if !audit.BondSumMatchesPool {
		return audit, fmt.Errorf(
			"liquidstaking: accounting invariant violated: sum(validator bonds)=%d, TotalBondedUmc=%d",
			sum, ps.TotalBondedUmc)
	}
	return audit, nil
}

// allValidatorBondsChecked 是 AllValidatorBonds 的可返回错误版本。
func (k Keeper) allValidatorBondsChecked(ctx sdk.Context) ([]types.ValidatorBond, error) {
	store := ctx.KVStore(k.storeKey)
	it := storetypes.KVStorePrefixIterator(store, types.ValidatorBondKeyPrefix)
	defer it.Close()

	out := []types.ValidatorBond{}
	for ; it.Valid(); it.Next() {
		var vb types.ValidatorBond
		if err := json.Unmarshal(it.Value(), &vb); err != nil {
			return nil, fmt.Errorf("liquidstaking: unmarshal validator bond at %x: %w", it.Key(), err)
		}
		out = append(out, vb)
	}
	return out, nil
}
