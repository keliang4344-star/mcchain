package types

import (
	"errors"
	"fmt"
)

// Params are the governance-tunable knobs of the liquid staking module.
// Stored JSON-encoded in the module KVStore.
type Params struct {
	// Enabled gates the whole module. Genesis default is true; governance can
	// pause new liquid staking without freezing existing positions.
	Enabled bool `json:"enabled"`
	// MinStakeUmc is the smallest accepted liquid stake, in umc.
	MinStakeUmc uint64 `json:"min_stake_umc"`
	// MaxValidatorShareBps caps the share of the module's total bonded stake
	// that may sit with a single validator, in basis points (10000 = 100%).
	// This prevents liquid staking from silently centralising the validator set.
	MaxValidatorShareBps uint32 `json:"max_validator_share_bps"`
	// UnbondingClaimGraceSeconds is added to the staking unbonding completion
	// time before a receipt becomes claimable, absorbing clock skew.
	UnbondingClaimGraceSeconds int64 `json:"unbonding_claim_grace_seconds"`
}

// DefaultParams returns the launch configuration.
func DefaultParams() Params {
	return Params{
		Enabled:                    true,
		MinStakeUmc:                1_000_000, // 1 MC
		MaxValidatorShareBps:       2_000,     // 20% of module stake per validator
		UnbondingClaimGraceSeconds: 0,
	}
}

// Validate checks parameter sanity.
func (p Params) Validate() error {
	if p.MinStakeUmc == 0 {
		return errors.New("liquidstaking: min_stake_umc must be positive")
	}
	if p.MaxValidatorShareBps == 0 || p.MaxValidatorShareBps > 10_000 {
		return fmt.Errorf("liquidstaking: max_validator_share_bps must be in (0,10000], got %d", p.MaxValidatorShareBps)
	}
	if p.UnbondingClaimGraceSeconds < 0 {
		return errors.New("liquidstaking: unbonding_claim_grace_seconds must not be negative")
	}
	return nil
}

// PoolState is the aggregate accounting used to derive the ulmc/umc exchange rate.
type PoolState struct {
	// TotalBondedUmc is the amount of umc this module currently has delegated
	// (principal plus compounded rewards, minus amounts already undelegated).
	TotalBondedUmc uint64 `json:"total_bonded_umc"`
	// TotalSharesUlmc is the outstanding supply of the liquid receipt token.
	TotalSharesUlmc uint64 `json:"total_shares_ulmc"`
	// TotalUnbondingUmc is umc that left the bonded set and is waiting on the
	// staking unbonding period before it can be claimed.
	TotalUnbondingUmc uint64 `json:"total_unbonding_umc"`
	// CumulativeRewardsUmc is the lifetime staking reward compounded into the pool.
	CumulativeRewardsUmc uint64 `json:"cumulative_rewards_umc"`
}

// UnbondingEntry is a redemption receipt created by Unstake.
type UnbondingEntry struct {
	ID                 uint64 `json:"id"`
	Delegator          string `json:"delegator"`
	Validator          string `json:"validator"`
	AmountUmc          uint64 `json:"amount_umc"`
	SharesBurnedUlmc   uint64 `json:"shares_burned_ulmc"`
	CompletionUnixTime int64  `json:"completion_unix_time"`
	Claimed            bool   `json:"claimed"`
}

// ValidatorBond tracks how much umc this module delegated to one validator.
type ValidatorBond struct {
	Validator string `json:"validator"`
	AmountUmc uint64 `json:"amount_umc"`
}

// GenesisState is the module genesis. Plain JSON: this module intentionally
// carries no protobuf state, so genesis is encoded with encoding/json.
type GenesisState struct {
	Params          Params           `json:"params"`
	PoolState       PoolState        `json:"pool_state"`
	UnbondingQueue  []UnbondingEntry `json:"unbonding_queue"`
	ValidatorBonds  []ValidatorBond  `json:"validator_bonds"`
	NextUnbondingID uint64           `json:"next_unbonding_id"`
}

// DefaultGenesis returns a clean genesis.
func DefaultGenesis() GenesisState {
	return GenesisState{
		Params:          DefaultParams(),
		PoolState:       PoolState{},
		UnbondingQueue:  []UnbondingEntry{},
		ValidatorBonds:  []ValidatorBond{},
		NextUnbondingID: 1,
	}
}

// Validate performs genesis sanity checks.
func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return err
	}
	if gs.PoolState.TotalBondedUmc == 0 && gs.PoolState.TotalSharesUlmc != 0 {
		return errors.New("liquidstaking: shares outstanding with no bonded stake")
	}
	if gs.PoolState.TotalSharesUlmc == 0 && gs.PoolState.TotalBondedUmc != 0 {
		return errors.New("liquidstaking: bonded stake with no shares outstanding")
	}
	seen := make(map[uint64]struct{}, len(gs.UnbondingQueue))
	for _, e := range gs.UnbondingQueue {
		if e.Delegator == "" {
			return errors.New("liquidstaking: unbonding entry with empty delegator")
		}
		if _, dup := seen[e.ID]; dup {
			return fmt.Errorf("liquidstaking: duplicate unbonding id %d", e.ID)
		}
		seen[e.ID] = struct{}{}
		if e.ID >= gs.NextUnbondingID {
			return fmt.Errorf("liquidstaking: unbonding id %d >= next id %d", e.ID, gs.NextUnbondingID)
		}
	}
	return nil
}
