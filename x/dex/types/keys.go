package types

import "fmt"

const (
	ModuleName = "dex"
	StoreKey   = ModuleName
	RouterKey  = ModuleName

	DefaultFeeRateBps = 30
	MaxPoolID         = 1000

	DenomPrefix = "dex/pool/"
)

// KVStore key prefixes for state persistence.
var (
	// LiquidityLockKeyPrefix stores LP lock positions.
	// Format: 0x03 + len(lp_address) + lp_address + pool_id (8 bytes big-endian)
	LiquidityLockKeyPrefix = []byte{0x03}
)

func KeyPrefix(p string) []byte {
	return []byte(p)
}

func PoolKey(poolID uint64) []byte {
	return []byte(fmt.Sprintf("%s%d", DenomPrefix, poolID))
}

func PoolDenom(poolID uint64) string {
	return fmt.Sprintf("%s%d", DenomPrefix, poolID)
}

// ---------------------------------------------------------------------------
// LiquidityLock key helpers
// ---------------------------------------------------------------------------

// LiquidityLockKey builds a key for a liquidity lock entry.
// Format: prefix (0x03) | lp_address | pool_id (big-endian uint64).
func LiquidityLockKey(lpAddress string, poolID uint64) []byte {
	addrLen := len(lpAddress)
	key := make([]byte, 1+1+addrLen+8)
	key[0] = LiquidityLockKeyPrefix[0]
	key[1] = byte(addrLen)
	copy(key[2:], []byte(lpAddress))
	copy(key[2+addrLen:], Uint64ToBigEndian(poolID))
	return key
}

// Uint64ToBigEndian encodes a uint64 as 8 big-endian bytes.
func Uint64ToBigEndian(v uint64) []byte {
	b := make([]byte, 8)
	b[0] = byte(v >> 56)
	b[1] = byte(v >> 48)
	b[2] = byte(v >> 40)
	b[3] = byte(v >> 32)
	b[4] = byte(v >> 24)
	b[5] = byte(v >> 16)
	b[6] = byte(v >> 8)
	b[7] = byte(v)
	return b
}
