package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"mcchain/x/tokenomics/types"
)

// TestDefaultGenesis_Validate 验证默认创世状态通过校验（R1/R2 双保险）：
//   - bps 和 = 10000（团队15/社区35/生态50）；
//   - 各池分配额之和 = 总量上限 cap；
//   - cap 必须等于 Go 常量 TotalSupplyCap（Q8 不可治理）。
func TestDefaultGenesis_Validate(t *testing.T) {
	gs := types.DefaultGenesis()
	require.NoError(t, gs.Validate())

	// cap 常量双保险。
	require.Equal(t, types.TotalSupplyCap, gs.TotalSupplyCap)

	// 占比和 = 10000。
	var bpsSum uint32
	for _, a := range gs.Allocations {
		bpsSum += a.PercentBps
	}
	require.Equal(t, uint32(10000), bpsSum)

	// 分配额和 = cap。
	var sum uint64
	for _, a := range gs.Allocations {
		sum += a.AllocatedAmount
	}
	require.Equal(t, types.TotalSupplyCap, sum)
}

// TestGenesis_Validate_Negative 验证默认创世状态的负向用例：
// 任何破坏 bps 和 / 分配和 / cap 常量 / denom 的改动都应校验失败。
func TestGenesis_Validate_Negative(t *testing.T) {
	// 负向①：破坏 bps 之和（!= 10000）。
	gs := types.DefaultGenesis()
	gs.Allocations[0].PercentBps = 9999
	require.Error(t, gs.Validate())

	// 负向②：破坏分配额之和（!= cap）。
	gs2 := types.DefaultGenesis()
	gs2.Allocations[0].AllocatedAmount += 1
	require.Error(t, gs2.Validate())

	// 负向③：cap 与常量不一致。
	gs3 := types.DefaultGenesis()
	gs3.TotalSupplyCap = types.TotalSupplyCap + 1
	require.Error(t, gs3.Validate())

	// 负向④：空 denom。
	gs4 := types.DefaultGenesis()
	gs4.Denom = ""
	require.Error(t, gs4.Validate())
}
