package types

import (
	"fmt"

	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
)

var _ paramtypes.ParamSet = (*Params)(nil)

func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

func NewParams() Params {
	return Params{
		Level1RewardRateBps: DefaultLevel1RewardRateBps,
		Level2RewardRateBps: DefaultLevel2RewardRateBps,
		Level3RewardRateBps: DefaultLevel3RewardRateBps,
		MinPayout:           DefaultMinPayout,
		MaxReferralsPerUser: DefaultMaxReferralsPerUser,
		CooldownBlocks:      DefaultCooldownBlocks,
		DailyPerUserCap:     DefaultDailyPerUserCap,
		DailyNetworkCap:     DefaultDailyNetworkCap,
	}
}

func DefaultParams() Params {
	return NewParams()
}

func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{
		paramtypes.NewParamSetPair(
			[]byte("Level1RewardRateBps"),
			&p.Level1RewardRateBps,
			validateRewardRateBps,
		),
		paramtypes.NewParamSetPair(
			[]byte("Level2RewardRateBps"),
			&p.Level2RewardRateBps,
			validateRewardRateBps,
		),
		paramtypes.NewParamSetPair(
			[]byte("Level3RewardRateBps"),
			&p.Level3RewardRateBps,
			validateRewardRateBps,
		),
		paramtypes.NewParamSetPair(
			[]byte("MinPayout"),
			&p.MinPayout,
			validateMinPayout,
		),
		paramtypes.NewParamSetPair(
			[]byte("MaxReferralsPerUser"),
			&p.MaxReferralsPerUser,
			validateMaxReferralsPerUser,
		),
		paramtypes.NewParamSetPair(
			[]byte("CooldownBlocks"),
			&p.CooldownBlocks,
			validateCooldownBlocks,
		),
		paramtypes.NewParamSetPair(
			[]byte("DailyPerUserCap"),
			&p.DailyPerUserCap,
			validateUint64Positive,
		),
		paramtypes.NewParamSetPair(
			[]byte("DailyNetworkCap"),
			&p.DailyNetworkCap,
			validateUint64Positive,
		),
	}
}

func (p Params) Validate() error {
	if err := validateRewardRateBps(p.Level1RewardRateBps); err != nil {
		return err
	}
	if err := validateRewardRateBps(p.Level2RewardRateBps); err != nil {
		return err
	}
	if err := validateRewardRateBps(p.Level3RewardRateBps); err != nil {
		return err
	}
	if err := validateMinPayout(p.MinPayout); err != nil {
		return err
	}
	if err := validateMaxReferralsPerUser(p.MaxReferralsPerUser); err != nil {
		return err
	}
	if err := validateCooldownBlocks(p.CooldownBlocks); err != nil {
		return err
	}
	if err := validateUint64Positive(p.DailyPerUserCap); err != nil {
		return err
	}
	return validateUint64Positive(p.DailyNetworkCap)
}

func validateRewardRateBps(i interface{}) error {
	v, ok := i.(uint32)
	if !ok {
		return fmt.Errorf("invalid parameter type for RewardRateBps: %T", i)
	}
	if v > 10000 {
		return fmt.Errorf("reward rate bps must be <= 10000, got %d", v)
	}
	return nil
}

func validateMinPayout(i interface{}) error {
	v, ok := i.(string)
	if !ok {
		return fmt.Errorf("invalid parameter type for MinPayout: %T", i)
	}
	if v == "" {
		return fmt.Errorf("min payout must not be empty")
	}
	return nil
}

func validateMaxReferralsPerUser(i interface{}) error {
	v, ok := i.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type for MaxReferralsPerUser: %T", i)
	}
	if v == 0 {
		return fmt.Errorf("max referrals per user must be > 0")
	}
	return nil
}

func validateCooldownBlocks(i interface{}) error {
	_, ok := i.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type for CooldownBlocks: %T", i)
	}
	// cooldown of 0 is allowed (no cooldown)
	return nil
}

func validateUint64Positive(i interface{}) error {
	v, ok := i.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type for daily cap: %T", i)
	}
	if v == 0 {
		return fmt.Errorf("daily cap must be > 0")
	}
	return nil
}
