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

func KeyPrefix(p string) []byte {
	return []byte(p)
}

func PoolKey(poolID uint64) []byte {
	return []byte(fmt.Sprintf("%s%d", DenomPrefix, poolID))
}

func PoolDenom(poolID uint64) string {
	return fmt.Sprintf("%s%d", DenomPrefix, poolID)
}
