package types

import (
	"fmt"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"gopkg.in/yaml.v2"
)

var _ paramtypes.ParamSet = (*Params)(nil)

var (
	KeyDisputePeriodBlocks   = []byte("DisputePeriodBlocks")
	KeyAntiCheatThresholdBps = []byte("AntiCheatThresholdBps")
	KeyMaxTaskReward         = []byte("MaxTaskReward")
	KeyArbitrator            = []byte("Arbitrator")
)

func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

func NewParams() Params {
	return Params{}
}

func DefaultParams() Params {
	return Params{
		DisputePeriodBlocks:   100,          // ~100 blocks dispute window
		AntiCheatThresholdBps: 5000,         // 50% threshold
		MaxTaskReward:         "1000000000", // 1e9 umc = 1000 MC
		Arbitrator:            "",           // 部署时必须设为团队多签地址
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
	if !ok {
		return fmt.Errorf("invalid type: %T", i)
	}
	if v <= 0 {
		return fmt.Errorf("dispute_period_blocks must be positive: %d", v)
	}
	return nil
}

func validateBps(i interface{}) error {
	v, ok := i.(uint32)
	if !ok {
		return fmt.Errorf("invalid type: %T", i)
	}
	if v > 10000 {
		return fmt.Errorf("bps must be <= 10000: %d", v)
	}
	return nil
}

func validateReward(i interface{}) error {
	v, ok := i.(string)
	if !ok {
		return fmt.Errorf("invalid type: %T", i)
	}
	// MaxTaskReward 是链上真实执行的托管上限，必须是可解析的非负整数。
	// 朴素实现只检查「是不是 string」，治理把它改成 "abc" 也能通过，
	// 上限随即静默失效。
	if v == "" {
		return fmt.Errorf("max_task_reward must not be empty")
	}
	amt, ok := sdkmath.NewIntFromString(v)
	if !ok {
		return fmt.Errorf("max_task_reward must be an integer: %q", v)
	}
	if amt.IsNegative() {
		return fmt.Errorf("max_task_reward must not be negative: %s", amt)
	}
	return nil
}

// validateArbitrator: 允许空串（未配置）；非空时必须是合法 bech32 地址。
func validateArbitrator(i interface{}) error {
	v, ok := i.(string)
	if !ok {
		return fmt.Errorf("invalid type: %T", i)
	}
	if v == "" {
		return nil
	}
	if _, err := sdk.AccAddressFromBech32(v); err != nil {
		return fmt.Errorf("arbitrator must be empty or a valid bech32 address: %w", err)
	}
	return nil
}

func (p Params) Validate() error {
	if p.DisputePeriodBlocks <= 0 {
		return fmt.Errorf("dispute_period_blocks must be positive")
	}
	if p.AntiCheatThresholdBps > 10000 {
		return fmt.Errorf("anti_cheat_threshold_bps must be <= 10000")
	}
	if err := validateReward(p.MaxTaskReward); err != nil {
		return err
	}
	if err := validateArbitrator(p.Arbitrator); err != nil {
		return err
	}
	return nil
}

// ValidateOperational 是「生产就绪」校验，在 Validate 之上额外要求仲裁人已配置。
// Arbitrator 为空时 x/edgeai 的争议裁决入口无人可调用——被质疑的任务会
// 永远卡在争议态，托管金也随之锁死。开发/测试链允许留空以便本地起链，
// 主网创世必须显式配置团队多签地址，由 app.InitChainer 做 fail-closed 拦截。
func (p Params) ValidateOperational() error {
	if err := p.Validate(); err != nil {
		return err
	}
	if p.Arbitrator == "" {
		return fmt.Errorf("edgeai: arbitrator must be configured on a production chain")
	}
	return nil
}

func (p Params) String() string {
	out, _ := yaml.Marshal(p)
	return string(out)
}
