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
// 4.675e14 umc == 4.675e8 MC — 即设备激励池整体额度（DepinInitialPoolSlice = 5.5e14，
// 占 1B 总量 55%）切出「推荐返佣生态预算（ReferralEcosystemBudget = 8.25e13，= 55% 的 15%）」
// 之后的剩余部分，由 tokenomics.InitGenesis 一次性注入 depin 模块账户（tokenomics → depin）。
// 不变量（防漂移）：DepinInitialPoolSlice - ReferralEcosystemBudget == DefaultInitialPool。
const DefaultInitialPool uint64 = 467_500_000_000_000

// DefaultRewardDenom is the denom used for DePIN reward payouts.
const DefaultRewardDenom = "umc"

// DefaultOracleAddresses is the set of oracle signer addresses authorized to
// submit device attestation results via MsgSubmitAttestation.
//
// 故意默认为空：开发/测试链豁免白名单闸门（兼容 SoftOracle），而生产链若白名单
// 为空将在 SubmitAttestation 处 fail-closed（拒绝一切提交），直到构建主网前由
// 部署方把真实预言机地址写入本常量。
//
// 该清单为编译期常量，随版本发布更新；主网启动前由部署方将真实
// 预言机地址写入本常量，轮换同样通过发布新版本完成。
var DefaultOracleAddresses = []string{}

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
