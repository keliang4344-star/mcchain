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

// TestComputeVested_TotalAboveMaxInt64 是 OVF 修复的核心回归：
//
// 旧实现 `sdk.NewInt(int64(totalLocked))` 在 totalLocked > MaxInt64（9.2e18）时
// 把高位截断成负数，vested 变成负值，随后 Int.Uint64() 直接 panic
// （DeliverTx 内 panic = 全网停机）；即使不 panic，释放额也会静默算错。
// 现在用 NewIntFromUint64 全程任意精度，必须给出单调正确的释放曲线。
func TestComputeVested_TotalAboveMaxInt64(t *testing.T) {
	total := uint64(^uint64(0)) // MaxUint64 ≈ 1.8e19，远超 MaxInt64
	start := int64(1_000_000)
	end := start + int64(4*365*24*3600)
	span := end - start

	// cliff 前
	vested, remaining, prog := tokenomicskeeper.ComputeVested(total, start, end, start-1)
	require.Equal(t, uint64(0), vested)
	require.Equal(t, total, remaining)
	require.Equal(t, uint32(0), prog)

	// 1/4 处：vested 应约为 total/4，且绝不 panic、不为 0。
	quarter := start + span/4
	vested, _, _ = tokenomicskeeper.ComputeVested(total, start, end, quarter)
	require.Greater(t, vested, uint64(0), "大锁仓额在 1/4 处必须有释放")
	require.Less(t, vested, total, "1/4 处释放额必须小于总额")

	// 中点：约为一半（允许整数除法 ±1 偏差；progress 为 floor 语义，允许 ±1）。
	mid := start + span/2
	vested, remaining, prog = tokenomicskeeper.ComputeVested(total, start, end, mid)
	require.InDelta(t, float64(total)/2, float64(vested), 1.0)
	require.Equal(t, total-vested, remaining)
	require.InDelta(t, 5000, prog, 1)

	// 结束后：全额，progress=10000，remaining=0。
	vested, remaining, prog = tokenomicskeeper.ComputeVested(total, start, end, end+1)
	require.Equal(t, total, vested)
	require.Equal(t, uint64(0), remaining)
	require.Equal(t, uint32(10000), prog)
}
