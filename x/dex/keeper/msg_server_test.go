package keeper

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"mcchain/x/dex/types"
)

// P1-01：最小输出解析失败绝不能降级为 0。
//
// 覆盖：空字符串、负数、非数字、小数、空白、极大值、零值（合法）与合法边界。
// 非法输入必须在进入 keeper 之前被 MsgServer 拒绝，错误类型为 ErrInvalidMinOut；
// 合法输入（含显式的 "0"）应继续走业务路径（此处因池不存在而返回其他错误）。
func TestMsgServer_MinOutParsingIsFailClosed(t *testing.T) {
	k, ctx, _ := setupDex(t)
	ms := NewMsgServerImpl(*k)
	creator := addrOfDex(t)

	invalid := []string{"", " ", "-1", "abc", "1.5", "1e18", "0x10", "-0"}
	for _, bad := range invalid {
		_, err := ms.AddLiquidity(sdk.WrapSDKContext(ctx), &types.MsgAddLiquidity{
			Creator: creator, PoolId: 1, AmountAMax: "100", AmountBMax: "100", MinLpOut: bad,
		})
		require.ErrorIs(t, err, types.ErrInvalidMinOut, "AddLiquidity min_lp_out=%q must be rejected", bad)

		_, err = ms.RemoveLiquidity(sdk.WrapSDKContext(ctx), &types.MsgRemoveLiquidity{
			Creator: creator, PoolId: 1, LpAmount: "100", MinAOut: bad, MinBOut: "1",
		})
		require.ErrorIs(t, err, types.ErrInvalidMinOut, "RemoveLiquidity min_a_out=%q must be rejected", bad)

		_, err = ms.RemoveLiquidity(sdk.WrapSDKContext(ctx), &types.MsgRemoveLiquidity{
			Creator: creator, PoolId: 1, LpAmount: "100", MinAOut: "1", MinBOut: bad,
		})
		require.ErrorIs(t, err, types.ErrInvalidMinOut, "RemoveLiquidity min_b_out=%q must be rejected", bad)

		_, err = ms.SwapExactIn(sdk.WrapSDKContext(ctx), &types.MsgSwapExactIn{
			Creator: creator, PoolId: 1, DenomIn: "umc", DenomOut: "uatom", AmountIn: "100", MinAmountOut: bad,
		})
		require.ErrorIs(t, err, types.ErrInvalidMinOut, "SwapExactIn min_amount_out=%q must be rejected", bad)
	}

	// 合法值（含显式 0 = 明确接受任意滑点）：不得被 ErrInvalidMinOut 拦截。
	legal := []string{"0", "1", "000001", "18446744073709551615"}
	for _, ok := range legal {
		_, err := ms.AddLiquidity(sdk.WrapSDKContext(ctx), &types.MsgAddLiquidity{
			Creator: creator, PoolId: 1, AmountAMax: "100", AmountBMax: "100", MinLpOut: ok,
		})
		require.NotErrorIs(t, err, types.ErrInvalidMinOut, "min_lp_out=%q must be accepted as syntactically valid", ok)
	}
}
