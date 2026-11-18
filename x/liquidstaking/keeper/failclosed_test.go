package keeper

import (
	"encoding/json"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"mcchain/x/liquidstaking/types"
)

// P1-02：资金状态损坏必须 fail-closed，绝不能当成 0 或跳过。
func TestValidatorBondCorruptionFailsClosed(t *testing.T) {
	f := setupLiquidStaking(t)
	val := "mcvaloper1testvalidator0000000000000000000000"

	// 写入坏 JSON（绕过 setter）。
	f.ctx.KVStore(f.k.storeKey).Set(types.ValidatorBondKey(val), []byte("{not-json"))

	require.Panics(t, func() { f.k.GetValidatorBond(f.ctx, val) },
		"corrupt validator bond must not be silently treated as zero")
	_, err := f.k.GetValidatorBondChecked(f.ctx, val)
	require.Error(t, err)

	require.Panics(t, func() { f.k.AllValidatorBonds(f.ctx) },
		"enumeration must not skip corrupt records")
	_, err = f.k.AuditAccounting(f.ctx)
	require.Error(t, err)
}

func TestUnbondingEntryCorruptionFailsClosed(t *testing.T) {
	f := setupLiquidStaking(t)
	delegator := "mc1testdelegator00000000000000000000000000"

	f.ctx.KVStore(f.k.storeKey).Set(types.UnbondingEntryKey(delegator, 7), []byte("[]"))

	require.Panics(t, func() { f.k.GetUnbondingEntry(f.ctx, delegator, 7) })
	require.Panics(t, func() { f.k.GetDelegatorUnbondings(f.ctx, delegator) })
	require.Panics(t, func() { f.k.AllUnbondings(f.ctx) })
}

// SchemaVersion：缺失时初始化为当前版本；未知/更高版本必须拒绝。
func TestEnsureSchemaVersion(t *testing.T) {
	f := setupLiquidStaking(t)

	// 缺失 → 写入当前版本
	require.NoError(t, f.k.EnsureSchemaVersion(f.ctx))
	bz := f.ctx.KVStore(f.k.storeKey).Get(types.SchemaVersionKey)
	require.Equal(t, sdk.Uint64ToBigEndian(SchemaVersion), bz)

	// 更高版本 → 拒绝（二进制落后）
	f.ctx.KVStore(f.k.storeKey).Set(types.SchemaVersionKey, sdk.Uint64ToBigEndian(SchemaVersion+1))
	require.Error(t, f.k.EnsureSchemaVersion(f.ctx))

	// 损坏长度 → 拒绝
	f.ctx.KVStore(f.k.storeKey).Set(types.SchemaVersionKey, []byte{1, 2, 3})
	require.Error(t, f.k.EnsureSchemaVersion(f.ctx))
}

// AuditAccounting：Σ validator bonds == TotalBondedUmc 时必须通过；不等则报错。
func TestAuditAccountingInvariant(t *testing.T) {
	f := setupLiquidStaking(t)

	const val = "mcvaloper1auditvalidator000000000000000000000"
	ps := types.PoolState{TotalBondedUmc: 1_000, TotalSharesUlmc: 1_000}
	f.k.SetPoolState(f.ctx, ps)
	f.k.setValidatorBond(f.ctx, val, 1_000)

	audit, err := f.k.AuditAccounting(f.ctx)
	require.NoError(t, err)
	require.True(t, audit.BondSumMatchesPool)
	require.Equal(t, uint64(1_000), audit.SumValidatorBonds)

	// 破坏守恒：池账本大于分验证人债券之和。
	ps.TotalBondedUmc = 1_001
	f.k.SetPoolState(f.ctx, ps)
	_, err = f.k.AuditAccounting(f.ctx)
	require.Error(t, err, "accounting mismatch must be reported")

	// 直接写入坏记录也必须被审计发现（JSON 损坏）。
	f.ctx.KVStore(f.k.storeKey).Set(types.ValidatorBondKey("mcvaloper1bad"), []byte("{bad"))
	_, err = f.k.AuditAccounting(f.ctx)
	require.Error(t, err)
}

// 保证 AuditAccounting 的结构化输出可被 JSON 序列化（供 CLI/报表使用）。
func TestAccountingAuditJSON(t *testing.T) {
	a := AccountingAudit{TotalBondedUmc: 1, BondSumMatchesPool: true}
	_, err := json.Marshal(a)
	require.NoError(t, err)
}
