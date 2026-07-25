package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// P1-2 / D2: DefaultParams must lock InitialPool=4.675e14 (umc) — 即设备激励池整体
// 55%（5.5e14）切出推荐返佣生态预算（8.25e13，= 55% 的 15%）后的 DePIN 挖矿奖励金库余额，
// 由 tokenomics.InitGenesis 一次性注入 depin 模块账户；RewardDenom="umc"。
func TestDefaultParams(t *testing.T) {
	p := DefaultParams()
	require.Equal(t, uint64(467_500_000_000_000), p.InitialPool)
	require.Equal(t, "umc", p.RewardDenom)
}

// P1-2 / D2: NewParams must equal DefaultParams.
func TestNewParams(t *testing.T) {
	require.Equal(t, DefaultParams(), NewParams())
}

// P1-2: ParamSetPairs must register both params with the correct keys, and the
// per-field validators must accept the defaults and reject bad values.
func TestParamSetPairs(t *testing.T) {
	p := DefaultParams()
	pairs := p.ParamSetPairs()
	require.Len(t, pairs, 2)

	keys := map[string]bool{}
	for _, pair := range pairs {
		keys[string(pair.Key)] = true
	}
	require.True(t, keys[string(ParamsKeyInitialPool)])
	require.True(t, keys[string(ParamsKeyRewardDenom)])

	// validators accept the default values
	require.NoError(t, validateInitialPool(uint64(467_500_000_000_000)))
	require.NoError(t, validateRewardDenom("umc"))

	// validators reject bad values
	require.Error(t, validateInitialPool(uint64(0)))
	require.Error(t, validateInitialPool("not uint64"))
	require.Error(t, validateRewardDenom(""))
	require.Error(t, validateRewardDenom(123))
}

// P1-2: Params.Validate must pass for defaults and fail for empty pool/denom.
func TestParamsValidate(t *testing.T) {
	require.NoError(t, DefaultParams().Validate())

	badPool := DefaultParams()
	badPool.InitialPool = 0
	require.Error(t, badPool.Validate())

	badDenom := DefaultParams()
	badDenom.RewardDenom = ""
	require.Error(t, badDenom.Validate())
}
