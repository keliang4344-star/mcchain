package types

const (
	// ModuleName defines the module name.
	ModuleName = "liquidstaking"

	// StoreKey defines the primary module store key.
	StoreKey = ModuleName

	// RouterKey is the message route.
	RouterKey = ModuleName

	// MemStoreKey defines the in-memory store key.
	MemStoreKey = "mem_liquidstaking"

	// BondDenom is the native staking denom of MobileChain.
	BondDenom = "umc"

	// LiquidBondDenom is the transferable representation of bonded MC.
	//
	// IMPORTANT (whitepaper §24, zero-inflation hard constraint):
	// ulmc is a derivative receipt token, NOT MC. It is minted only against
	// MC that is already bonded through this module and burned on redemption.
	// It never increases the 1,000,000,000 MC hard cap and is never counted as
	// MC supply. This mirrors the existing DEX LP-share precedent (poolN denom).
	LiquidBondDenom = "ulmc"
)

var (
	// ParamsKey stores the module params (JSON encoded).
	ParamsKey = []byte("Params:")

	// PoolStateKey stores the aggregate bonded/share accounting (JSON encoded).
	PoolStateKey = []byte("PoolState:")

	// RewardsCursorKey stores the persistent reward-compounding sweep cursor
	// (uint64 big-endian index into the module's delegation list). AccrueRewards
	// continues from this position each block so no validator is starved.
	RewardsCursorKey = []byte("RewardsCursor:")

	// UnbondingEntryKeyPrefix stores per-delegator unbonding receipts.
	UnbondingEntryKeyPrefix = []byte("Unbonding:")

	// NextUnbondingIDKey stores the monotonically increasing unbonding id.
	NextUnbondingIDKey = []byte("NextUnbondingID:")

	// ValidatorBondKeyPrefix tracks how much umc this module delegated per validator.
	ValidatorBondKeyPrefix = []byte("ValBond:")

	// SchemaVersionKey stores the on-disk state schema version (uint64 big-endian).
	// 资金状态的结构一旦变更，必须显式迁移；未知/更高版本直接拒绝读取，
	// 绝不让新代码在旧结构上「猜字段」。
	SchemaVersionKey = []byte("SchemaVersion:")

	// RewardsCursorValKey stores the stable reward-sweep cursor as the last
	// processed validator address. 取代易变数组下标：验证人集合增删时，
	// 下标游标会跳过或重复处理，地址游标不会。
	RewardsCursorValKey = []byte("RewardsCursorVal:")
)

// UnbondingEntryKey builds the storage key for one unbonding receipt.
func UnbondingEntryKey(delegator string, id uint64) []byte {
	key := make([]byte, 0, len(UnbondingEntryKeyPrefix)+len(delegator)+1+8)
	key = append(key, UnbondingEntryKeyPrefix...)
	key = append(key, []byte(delegator)...)
	key = append(key, '/')
	key = append(key, uint64ToBigEndian(id)...)
	return key
}

// UnbondingDelegatorPrefix returns the iteration prefix for one delegator.
func UnbondingDelegatorPrefix(delegator string) []byte {
	key := make([]byte, 0, len(UnbondingEntryKeyPrefix)+len(delegator)+1)
	key = append(key, UnbondingEntryKeyPrefix...)
	key = append(key, []byte(delegator)...)
	key = append(key, '/')
	return key
}

// ValidatorBondKey builds the storage key for a validator bond record.
func ValidatorBondKey(valAddr string) []byte {
	return append(append([]byte{}, ValidatorBondKeyPrefix...), []byte(valAddr)...)
}

func uint64ToBigEndian(v uint64) []byte {
	b := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		b[i] = byte(v)
		v >>= 8
	}
	return b
}
