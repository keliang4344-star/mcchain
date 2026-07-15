package types

import (
	"fmt"

	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"gopkg.in/yaml.v2"
)

var _ paramtypes.ParamSet = (*Params)(nil)

// Param store keys for the depin module.
var (
	ParamsKeyInitialPool = []byte("InitialPool")
	ParamsKeyRewardDenom = []byte("RewardDenom")
)

// DefaultInitialPool is the default size of the DePIN reward pool, in umc.
// 5.5e14 umc == 5.5e8 MC (55% of the 1B total supply) — the whole device-incentive
// pool of the five-pool model, injected once at genesis by tokenomics.InitGenesis
// (tokenomics → depin module account). Must equal tokenomics.DepinInitialPoolSlice.
const DefaultInitialPool uint64 = 550_000_000_000_000

// DefaultRewardDenom is the denom used for DePIN reward payouts.
const DefaultRewardDenom = "umc"

// ParamKeyTable the param key table for launch module
func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

// NewParams creates a new Params instance
func NewParams() Params {
	return Params{
		InitialPool: DefaultInitialPool,
		RewardDenom: DefaultRewardDenom,
	}
}

// DefaultParams returns a default set of parameters
func DefaultParams() Params {
	return NewParams()
}

// ParamSetPairs get the params.ParamSet
func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{
		paramtypes.NewParamSetPair(ParamsKeyInitialPool, &p.InitialPool, validateInitialPool),
		paramtypes.NewParamSetPair(ParamsKeyRewardDenom, &p.RewardDenom, validateRewardDenom),
	}
}

// validateInitialPool validates the InitialPool param: must be a positive uint64.
func validateInitialPool(i interface{}) error {
	v, ok := i.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	if v == 0 {
		return fmt.Errorf("initial pool must be positive: got %d", v)
	}
	return nil
}

// validateRewardDenom validates the RewardDenom param: must be non-empty.
func validateRewardDenom(i interface{}) error {
	v, ok := i.(string)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	if v == "" {
		return fmt.Errorf("reward denom cannot be empty")
	}
	return nil
}

// Validate validates the set of params
func (p Params) Validate() error {
	if err := validateInitialPool(p.InitialPool); err != nil {
		return err
	}
	return validateRewardDenom(p.RewardDenom)
}

// String implements the Stringer interface.
func (p Params) String() string {
	out, _ := yaml.Marshal(p)
	return string(out)
}
