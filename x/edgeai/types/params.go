package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"gopkg.in/yaml.v2"
)

var _ paramtypes.ParamSet = (*Params)(nil)

var (
	KeyDisputePeriodBlocks     = []byte("DisputePeriodBlocks")
	KeyAntiCheatThresholdBps   = []byte("AntiCheatThresholdBps")
	KeyMaxTaskReward           = []byte("MaxTaskReward")
	KeyArbitrator              = []byte("Arbitrator")
)

func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

func NewParams() Params {
	return Params{}
}

func DefaultParams() Params {
	return Params{
		DisputePeriodBlocks:   100,    // ~100 blocks dispute window
		AntiCheatThresholdBps: 5000,   // 50% threshold
		MaxTaskReward:         "1000000000", // 1e9 umc = 1000 MC
		Arbitrator:            "",     // B3.1：部署时必须设为团队多签地址
	}
}

func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{
		paramtypes.NewParamSetPair(KeyDisputePeriodBlocks, &p.DisputePeriodBlocks, validateDisputePeriodBlocks),
		paramtypes.NewParamSetPair(KeyAntiCheatThresholdBps, &p.AntiCheatThresholdBps, validateBps),
		paramtypes.NewParamSetPair(KeyMaxTaskReward, &p.MaxTaskReward, validateReward),
		paramtypes.NewParamSetPair(KeyArbitrator, &p.Arbitrator, validateArbitrator),
	}
}

func validateDisputePeriodBlocks(i interface{}) error {
	v, ok := i.(int64)
	if !ok { return fmt.Errorf("invalid type: %T", i) }
	if v <= 0 { return fmt.Errorf("dispute_period_blocks must be positive: %d", v) }
	return nil
}

func validateBps(i interface{}) error {
	v, ok := i.(uint32)
	if !ok { return fmt.Errorf("invalid type: %T", i) }
	if v > 10000 { return fmt.Errorf("bps must be <= 10000: %d", v) }
	return nil
}

func validateReward(i interface{}) error {
	_, ok := i.(string)
	if !ok { return fmt.Errorf("invalid type: %T", i) }
	return nil
}

// validateArbitrator: 允许空串（未配置）；非空时必须是合法 bech32 地址。
func validateArbitrator(i interface{}) error {
	v, ok := i.(string)
	if !ok { return fmt.Errorf("invalid type: %T", i) }
	if v == "" {
		return nil
	}
	if _, err := sdk.AccAddressFromBech32(v); err != nil {
		return fmt.Errorf("arbitrator must be empty or a valid bech32 address: %w", err)
	}
	return nil
}

func (p Params) Validate() error {
	if p.DisputePeriodBlocks <= 0 { return fmt.Errorf("dispute_period_blocks must be positive") }
	if p.AntiCheatThresholdBps > 10000 { return fmt.Errorf("anti_cheat_threshold_bps must be <= 10000") }
	return nil
}

func (p Params) String() string {
	out, _ := yaml.Marshal(p)
	return string(out)
}
