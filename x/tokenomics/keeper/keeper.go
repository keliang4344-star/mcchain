package keeper

import (
	"fmt"

	"github.com/cometbft/cometbft/libs/log"
	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/tokenomics/types"
)

type (
	// Keeper 是 tokenomics 的「发行与分配总账」keeper。
	// 唯一持 Minter 的模块；无 Msg service，运行期不增发/不销毁（R3）。
	Keeper struct {
		cdc           codec.BinaryCodec
		storeKey      storetypes.StoreKey
		accountKeeper types.AccountKeeper
		bankKeeper    types.BankKeeper
	}
)

// NewKeeper 构造 tokenomics keeper。
// 生态切片拨付目标模块固定为 types.DepinModuleName（C2：编译期常量，不再依赖调用方传入字符串）。
func NewKeeper(
	cdc codec.BinaryCodec,
	storeKey storetypes.StoreKey,
	accountKeeper types.AccountKeeper,
	bankKeeper types.BankKeeper,
) *Keeper {
	return &Keeper{
		cdc:           cdc,
		storeKey:      storeKey,
		accountKeeper: accountKeeper,
		bankKeeper:    bankKeeper,
	}
}

// Logger 返回模块日志器。
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// MintCoins 是整条链唯一的铸币入口。任何铸币都必须使累计 minted_supply <= TotalSupplyCap，
// 否则 panic（R1：总量固化，不可突破）。铸造后累加并持久化 minted_supply。
func (k Keeper) MintCoins(ctx sdk.Context, amt sdk.Coins) error {
	if err := k.bankKeeper.MintCoins(ctx, types.ModuleName, amt); err != nil {
		return err
	}
	oldMinted := k.GetMintedSupply(ctx)
	newMinted := oldMinted.Add(amt.AmountOf(types.DefaultDenom))
	if newMinted.GT(sdk.NewIntFromUint64(types.TotalSupplyCap)) {
		panic(fmt.Sprintf(
			"tokenomics: minted supply %s would exceed cap %d",
			newMinted.String(), types.TotalSupplyCap,
		))
	}
	k.SetMintedSupply(ctx, newMinted)
	return nil
}
