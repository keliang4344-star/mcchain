package types

import (
	"fmt"

	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
)

var _ paramtypes.ParamSet = (*Params)(nil)

// LP lock and incentive constants (whitepaper aligned).
const (
	// LpLockBlocksDefault is the default LP token lock duration: 100,800 blocks
	// (≈ 4.7 days at the ~4 s block time assumed in the whitepaper).
	LpLockBlocksDefault = 100800

	// LpIncentivePerDayDefault is the default daily LP incentive: 5,000 MC
	// (whitepaper §24, sourced from the device incentive pool / depin module account).
	// In umc (6 decimal places): 5,000 × 10^6 = 5,000,000,000.
	LpIncentivePerDayDefault = "5000000000"

	// LpIncentiveEndHeightDefault is the default end height for LP incentives.
	// 180 days × 21,600 blocks/day (at the ~4 s block time per whitepaper §A.1) = 3,888,000 blocks.
	LpIncentiveEndHeightDefault = 3888000

	// BlocksPerDay is the approximate number of blocks per day.
	// Aligned to the chain's ~4 s block time (whitepaper §A.1): 86,400 s / 4 s = 21,600 blocks/day.
	BlocksPerDay = 21600
)

func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

func NewParams() Params {
	return Params{
		DefaultFeeRateBps:    DefaultFeeRateBps,
		MaxPools:             MaxPoolID,
		LpIncentivePerDay:    LpIncentivePerDayDefault,
		LpIncentiveEndHeight: LpIncentiveEndHeightDefault,
		LpLockBlocks:         LpLockBlocksDefault,
	}
}

func DefaultParams() Params {
	return NewParams()
}

func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{
		paramtypes.NewParamSetPair(
			[]byte("DefaultFeeRateBps"),
			&p.DefaultFeeRateBps,
			validateFeeRateBps,
		),
		paramtypes.NewParamSetPair(
			[]byte("MaxPools"),
			&p.MaxPools,
			validateMaxPools,
		),
		paramtypes.NewParamSetPair(
			[]byte("LpIncentivePerDay"),
			&p.LpIncentivePerDay,
			validateLpIncentivePerDay,
		),
		paramtypes.NewParamSetPair(
			[]byte("LpIncentiveEndHeight"),
			&p.LpIncentiveEndHeight,
			validateLpIncentiveEndHeight,
		),
		paramtypes.NewParamSetPair(
			[]byte("LpLockBlocks"),
			&p.LpLockBlocks,
			validateLpLockBlocks,
		),
	}
}

func (p Params) Validate() error {
	if err := validateFeeRateBps(p.DefaultFeeRateBps); err != nil {
		return err
	}
	if err := validateMaxPools(p.MaxPools); err != nil {
		return err
	}
	if err := validateLpIncentivePerDay(p.LpIncentivePerDay); err != nil {
		return err
	}
	if err := validateLpIncentiveEndHeight(p.LpIncentiveEndHeight); err != nil {
		return err
	}
	return validateLpLockBlocks(p.LpLockBlocks)
}

func validateFeeRateBps(i interface{}) error {
	v, ok := i.(uint32)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	if v > MaxFeeRateBps {
		return fmt.Errorf("fee rate bps must be <= %d, got %d", MaxFeeRateBps, v)
	}
	return nil
}

func validateMaxPools(i interface{}) error {
	v, ok := i.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	if v == 0 {
		return fmt.Errorf("max pools must be > 0")
	}
	return nil
}

func validateLpIncentivePerDay(i interface{}) error {
	v, ok := i.(string)
	if !ok {
		return fmt.Errorf("invalid parameter type for LpIncentivePerDay: %T", i)
	}
	if v == "" {
		return fmt.Errorf("lp_incentive_per_day must not be empty")
	}
	return nil
}

func validateLpIncentiveEndHeight(i interface{}) error {
	if _, ok := i.(uint64); !ok {
		return fmt.Errorf("invalid parameter type for LpIncentiveEndHeight: %T", i)
	}
	// 0 means never end (valid for testing)
	return nil
}

func validateLpLockBlocks(i interface{}) error {
	v, ok := i.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type for LpLockBlocks: %T", i)
	}
	if v == 0 {
		return fmt.Errorf("lp_lock_blocks must be > 0")
	}
	return nil
}
