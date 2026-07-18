package types

import (
	"fmt"

	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"gopkg.in/yaml.v2"
)

var _ paramtypes.ParamSet = (*Params)(nil)

// Params defines the parameters for the referral module.
// Mirrors proto/mcchain/referral/params.proto.
type Params struct {
	RewardRateBps       uint32 `json:"reward_rate_bps" yaml:"reward_rate_bps"`
	MinPayout           string `json:"min_payout" yaml:"min_payout"`
	MaxReferralsPerUser uint64 `json:"max_referrals_per_user" yaml:"max_referrals_per_user"`
	CooldownBlocks      uint64 `json:"cooldown_blocks" yaml:"cooldown_blocks"`
}

func (p *Params) Reset()         { *p = Params{} }
func (p *Params) String() string { out, _ := yaml.Marshal(p); return string(out) }
func (p *Params) ProtoMessage()  {}

func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

func NewParams() Params {
	return Params{
		RewardRateBps:       DefaultRewardRateBps,
		MinPayout:           DefaultMinPayout,
		MaxReferralsPerUser: DefaultMaxReferralsPerUser,
		CooldownBlocks:      DefaultCooldownBlocks,
	}
}

func DefaultParams() Params {
	return NewParams()
}

func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{
		paramtypes.NewParamSetPair(
			[]byte("RewardRateBps"),
			&p.RewardRateBps,
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
	}
}

func (p Params) Validate() error {
	if err := validateRewardRateBps(p.RewardRateBps); err != nil {
		return err
	}
	if err := validateMinPayout(p.MinPayout); err != nil {
		return err
	}
	if err := validateMaxReferralsPerUser(p.MaxReferralsPerUser); err != nil {
		return err
	}
	return validateCooldownBlocks(p.CooldownBlocks)
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
	v, ok := i.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type for CooldownBlocks: %T", i)
	}
	// cooldown of 0 is allowed (no cooldown)
	return nil
}
