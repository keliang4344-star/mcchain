package types

import (
	"fmt"

	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"gopkg.in/yaml.v2"
)

var _ paramtypes.ParamSet = (*Params)(nil)

// Parameter store keys
var (
	KeyChainName    = []byte("ChainName")
	KeyChainVersion = []byte("ChainVersion")
	KeyGenesisTime  = []byte("GenesisTime")
)

// ParamKeyTable the param key table for launch module
func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

// NewParams creates a new Params instance
func NewParams() Params {
	return Params{}
}

// DefaultParams returns a default set of parameters
func DefaultParams() Params {
	return Params{
		ChainName:    "MChain",
		ChainVersion: "1.0.0",
		GenesisTime:  0,
	}
}

// ParamSetPairs get the params.ParamSet
func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{
		paramtypes.NewParamSetPair(KeyChainName, &p.ChainName, validateChainName),
		paramtypes.NewParamSetPair(KeyChainVersion, &p.ChainVersion, validateChainVersion),
		paramtypes.NewParamSetPair(KeyGenesisTime, &p.GenesisTime, validateGenesisTime),
	}
}

func validateChainName(i interface{}) error {
	v, ok := i.(string)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	if len(v) == 0 {
		return fmt.Errorf("chain_name must not be empty")
	}
	return nil
}

func validateChainVersion(i interface{}) error {
	v, ok := i.(string)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	if len(v) == 0 {
		return fmt.Errorf("chain_version must not be empty")
	}
	return nil
}

func validateGenesisTime(i interface{}) error {
	_, ok := i.(int64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	return nil
}

// Validate validates the set of params
func (p Params) Validate() error {
	if len(p.ChainName) == 0 {
		return fmt.Errorf("chain_name must not be empty")
	}
	if len(p.ChainVersion) == 0 {
		return fmt.Errorf("chain_version must not be empty")
	}
	return nil
}

// String implements the Stringer interface.
func (p Params) String() string {
	out, _ := yaml.Marshal(p)
	return string(out)
}
