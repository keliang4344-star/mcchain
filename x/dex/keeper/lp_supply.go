package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/dex/types"
)

// mintLPShares 铸造 LP 份额代币。
//
// 铸币铁律：dex 模块账户持有的 Minter 权限是「模块级」而非「denom 级」，
// bank 不会替我们区分铸的是 LP 份额还是 umc。此处是本模块唯一允许调用
// bank.MintCoins 的入口，且强制校验每一个 denom 都是本模块自己发行的
// `dex/pool/<id>` 份额；任何非 LP denom 一律拒绝，绝不给「DEX 增发 MC」
// 留下任何一条代码路径。
func (k Keeper) mintLPShares(ctx sdk.Context, coins sdk.Coins) error {
	if err := assertLPCoins(coins, "mint"); err != nil {
		return err
	}
	return k.bankKeeper.MintCoins(ctx, types.ModuleName, coins)
}

// burnLPShares 销毁 LP 份额代币，同样只接受 LP denom。
//
// 销毁侧的约束同等重要：Burner 权限若被用在 umc 上，会在 §24 的三条销毁路径
// 之外多出一条不受账的销毁口，破坏「总量恒定 10 亿」这一不变量。
func (k Keeper) burnLPShares(ctx sdk.Context, coins sdk.Coins) error {
	if err := assertLPCoins(coins, "burn"); err != nil {
		return err
	}
	return k.bankKeeper.BurnCoins(ctx, types.ModuleName, coins)
}

func assertLPCoins(coins sdk.Coins, op string) error {
	for _, c := range coins {
		if !types.IsPoolDenom(c.Denom) {
			return fmt.Errorf("dex: refusing to %s non-LP denom %q (only %s* is allowed)",
				op, c.Denom, types.DenomPrefix)
		}
	}
	return nil
}
