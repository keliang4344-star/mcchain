package types

import (
	"encoding/json"
	"fmt"
)

// =============================================================================
// Pool — mirrors proto/mcchain/dex/pool.proto
// =============================================================================

// Pool defines a constant-product AMM liquidity pool.
type Pool struct {
	Id         uint64 `json:"id"`
	DenomA     string `json:"denom_a"`
	DenomB     string `json:"denom_b"`
	ReserveA   string `json:"reserve_a"`
	ReserveB   string `json:"reserve_b"`
	TotalLp    string `json:"total_lp"`
	FeeRateBps uint32 `json:"fee_rate_bps"`
	Owner      string `json:"owner"`
}

func (p *Pool) Reset()         { *p = Pool{} }
func (p *Pool) String() string { return fmt.Sprintf("Pool{id:%d %s/%s}", p.Id, p.DenomA, p.DenomB) }
func (p *Pool) ProtoMessage()  {}

// MarshalJSON for Pool.
func (p Pool) MarshalJSON() ([]byte, error) {
	type alias Pool
	return json.Marshal(alias(p))
}

// UnmarshalJSON for Pool.
func (p *Pool) UnmarshalJSON(b []byte) error {
	type alias Pool
	return json.Unmarshal(b, (*alias)(p))
}

// =============================================================================
// GenesisState — mirrors proto/mcchain/dex/genesis.proto
// =============================================================================

// GenesisState defines the dex module's genesis state.
type GenesisState struct {
	Pools      []Pool `json:"pools"`
	NextPoolId uint64 `json:"next_pool_id"`
	Params     Params `json:"params"`
}

func (gs *GenesisState) Reset()        { *gs = GenesisState{} }
func (gs *GenesisState) String() string { return fmt.Sprintf("GenesisState{pools:%d}", len(gs.Pools)) }
func (gs *GenesisState) ProtoMessage()  {}

// =============================================================================
// Params — mirrors proto/mcchain/dex/genesis.proto (Params message)
// Extended with LP incentive and lock parameters from the whitepaper.
// =============================================================================

// Params defines the parameters for the dex module.
type Params struct {
	DefaultFeeRateBps   uint32 `json:"default_fee_rate_bps"`
	MaxPools            uint64 `json:"max_pools"`
	LpIncentivePerDay   string `json:"lp_incentive_per_day"`   // umc per day (default 5_000_000_000_000)
	LpIncentiveEndHeight uint64 `json:"lp_incentive_end_height"` // block height when LP incentive ends
	LpLockBlocks        uint64 `json:"lp_lock_blocks"`          // LP token lock duration in blocks (default ~100800 = 7 days)
}

func (p *Params) Reset()        { *p = Params{} }
func (p *Params) ProtoMessage() {}

// =============================================================================
// LiquidityLock — tracks LP token lock positions for the 7-day lock rule.
// =============================================================================

// LiquidityLock records an LP lock position created when a user adds liquidity.
// The LP tokens cannot be removed until unlock_height is reached.
type LiquidityLock struct {
	LpAddress   string `json:"lp_address"`   // bech32 address of the LP
	PoolId      uint64 `json:"pool_id"`       // pool identifier
	LockHeight  uint64 `json:"lock_height"`   // block height when the lock was created
	UnlockHeight uint64 `json:"unlock_height"` // block height when the lock expires
	LpAmount    string `json:"lp_amount"`     // amount of LP tokens locked (as string for sdk.Int)
}

func (l *LiquidityLock) Reset()         { *l = LiquidityLock{} }
func (l *LiquidityLock) String() string {
	return fmt.Sprintf("LiquidityLock{lp:%s pool:%d unlock:%d amount:%s}",
		l.LpAddress, l.PoolId, l.UnlockHeight, l.LpAmount)
}
func (l *LiquidityLock) ProtoMessage() {}
