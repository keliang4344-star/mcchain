package types

import "fmt"

const (
	ModuleName = "dex"
	StoreKey   = ModuleName
	RouterKey  = ModuleName

	DefaultFeeRateBps = 30
	MaxPoolID         = 1000

	// MaxFeeRateBps is the absolute upper bound of any fee rate: 10000 bps =
	// 100%. Every basis-point calculation in this module divides by this
	// constant, and `MaxFeeRateBps - feeRateBps` is evaluated on an unsigned
	// integer, so a rate above this bound would wrap around and corrupt the
	// AMM pricing. Never relax it.
	MaxFeeRateBps = 10000

	// MaxPoolFeeRateBps caps the fee a pool creator may choose: 1000 bps = 10%.
	// Whitepaper §24 fixes the canonical swap fee at 0.30% (30 bps); this cap
	// exists only so a pool owner cannot set a confiscatory rate that would
	// silently expropriate traders.
	MaxPoolFeeRateBps = 1000

	// MinimumLiquidity is the LP share permanently locked on a pool's first
	// deposit (Uniswap V2 口径, 1000 units).
	//
	// 首存路径 lpMinted = sqrt(addedA*addedB) 且
	// 无任何永久锁定份额时，攻击者可以 1:1 极小额建池拿走 100% LP，再单边打入
	// 大额资产把每份 LP 的价值抬到远高于 1 单位，使后续任何「按比例折算后不足
	// 1 份」的正常存款被舍入为 0 份而资产照收（first-depositor / share-inflation
	// 攻击）；同时 TotalLp 可被完全赎回归零，让同一个池子被反复按操纵价重新播种。
	//
	// 修复：首存时额外铸出 MinimumLiquidity 份并直接打入黑洞地址（由
	// black_hole 派生、无私钥、不可支出），使 TotalLp 永远 > 0、任何单份 LP 的
	// 价值都被这笔不可赎回的份额稀释住，价格无法被首存者单方面抬到任意高。
	// 相应地，首存必须至少铸出 MinimumLiquidity+1 份，否则拒绝建池。
	MinimumLiquidity = 1000

	// MaxSettlementEntries caps how many recipients a single off-chain
	// settlement batch may carry. FinalizeBatch iterates every entry inside one
	// transaction, so an unbounded batch is a block-time / gas-griefing vector.
	MaxSettlementEntries = 10000

	DenomPrefix = "dex/pool/"
)

// SettlementConfigKey stores the DEX settlement authority & circuit-breaker state
// as JSON (see x/dex/keeper/settlement_config.go). Not in proto Params by design.
var SettlementConfigKey = []byte("SettleConfig:")

// LPIncentiveDayKey stores the UTC day index of the most recent daily LP
// incentive distribution, as a decimal string.
//
// The daily trigger must not be `height % BlocksPerDay == 0`, which assumes
// genesis lands exactly on a UTC day boundary. Real deployments never do, so the
// DEX "day" drifted permanently out of phase with the day boundary used by
// referral / phonenode / depin (all of which key on block time / 86400).
// Persisting the last distributed day lets BeginBlock fire exactly once per UTC
// day, using the same window as every other module.
var LPIncentiveDayKey = []byte("LPIncentiveDay:")

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

// IsPoolDenom 判断一个 denom 是否为本模块自己发行的 LP 份额代币。
//
// 铸币铁律：dex 模块账户持有 Minter/Burner，仅因为 LP 份额需要铸销。
// bank 的权限是「按模块」而非「按 denom」授予的，也就是说这份权限在类型层面
// 同样能铸出 umc——一旦未来某条代码路径把非 LP 的 denom 传进 MintCoins，
// 10 亿硬顶就被击穿且没有任何编译期报错。故所有铸销必须先过此判据。
func IsPoolDenom(denom string) bool {
	return len(denom) > len(DenomPrefix) && denom[:len(DenomPrefix)] == DenomPrefix
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
