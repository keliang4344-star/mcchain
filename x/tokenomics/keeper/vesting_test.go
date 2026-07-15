package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	tokenomicskeeper "mcchain/x/tokenomics/keeper"
)

// TestComputeVested 验证释放曲线计算（Q3/Q9，共建模口径：1 年 cliff + 3 年线性 = 总 4 年）：
//   - cliff 前（now <= start）：vested = 0，remaining = 全额，progress = 0；
//   - 线性区间：vested 与 elapsed 成比例；中点处 vested = 一半，progress = 5000；
//   - 结束后（now >= end）：vested = 全额，remaining = 0，progress = 10000。
func TestComputeVested(t *testing.T) {
	total := uint64(1.5e14)
	start := int64(1_000_000)
	end := start + int64(3*365*24*3600) // 3 年线性窗口
	span := end - start

	// cliff 前（now < start）：0。
	vested, remaining, prog := tokenomicskeeper.ComputeVested(total, start, end, start-1)
	require.Equal(t, uint64(0), vested)
	require.Equal(t, total, remaining)
	require.Equal(t, uint32(0), prog)

	// cliff 边界（now == start）：0。
	vested, remaining, prog = tokenomicskeeper.ComputeVested(total, start, end, start)
	require.Equal(t, uint64(0), vested)
	require.Equal(t, total, remaining)
	require.Equal(t, uint32(0), prog)

	// 线性中点（now == start + span/2）：一半。
	mid := start + span/2
	vested, remaining, prog = tokenomicskeeper.ComputeVested(total, start, end, mid)
	require.Equal(t, total/2, vested)
	require.Equal(t, total-total/2, remaining)
	require.Equal(t, uint32(5000), prog)

	// 结束后（now == end）：全额。
	vested, remaining, prog = tokenomicskeeper.ComputeVested(total, start, end, end)
	require.Equal(t, total, vested)
	require.Equal(t, uint64(0), remaining)
	require.Equal(t, uint32(10000), prog)

	// 超过 end（now > end）：仍全额，不溢出。
	vested, remaining, prog = tokenomicskeeper.ComputeVested(total, start, end, end+1)
	require.Equal(t, total, vested)
	require.Equal(t, uint32(10000), prog)
}
