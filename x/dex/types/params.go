package types

import (
	"fmt"

	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"gopkg.in/yaml.v2"
)

var _ paramtypes.ParamSet = (*Params)(nil)

func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

func NewParams() Params {
	return Params{
		DefaultFeeRateBps: DefaultFeeRateBps,
		MaxPools:          MaxPoolID,
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
	}
}

func (p Params) Validate() error {
	if err := validateFeeRateBps(p.DefaultFeeRateBps); err != nil {
		return err
	}
	return validateMaxPools(p.MaxPools)
}

func (p Params) String() string {
	out, _ := yaml.Marshal(p)
	return string(out)
}

func validateFeeRateBps(i interface{}) error {
	v, ok := i.(uint32)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	if v > 10000 {
		return fmt.Errorf("fee rate bps must be <= 10000, got %d", v)
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
