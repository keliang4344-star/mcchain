package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// P1-2 / D2: DefaultParams must lock InitialPool=1e14 (umc) and RewardDenom="umc".
func TestDefaultParams(t *testing.T) {
	p := DefaultParams()
	require.Equal(t, uint64(1e14), p.InitialPool)
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
	require.NoError(t, validateInitialPool(uint64(1e14)))
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
