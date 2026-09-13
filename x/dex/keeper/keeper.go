package keeper

import (
	"fmt"

	"github.com/cometbft/cometbft/libs/log"
	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"

	"mcchain/x/dex/types"
)

type Keeper struct {
	cdc        codec.BinaryCodec
	storeKey   storetypes.StoreKey
	paramstore paramtypes.Subspace

	bankKeeper    types.BankKeeper
	accountKeeper types.AccountKeeper

	// depinKeeper 设备激励池的每日线性释放闸门。
	//
	// dex 不依赖 depin，depin 也不依赖 dex，但 dex 的 LP 激励是从 depin 模块
	// 账户出账的，因此这里以接口形式事后注入（与 phonenode 同一手法）。
	// 未注入时 fail-closed：宁可不发激励，也绝不允许绕过释放账本直抽设备池。
	depinKeeper types.DepinReleaseKeeper
}

// SetDepinKeeper 在 depin keeper 构造完成后回填设备池释放闸门。
//
// 必须在 dex 的 AppModule 构造之前调用 —— AppModule 以值持有 keeper，
// 先构造会拷贝进一个 depinKeeper 为 nil 的副本（见 app.go 接线注释）。
func (k *Keeper) SetDepinKeeper(dk types.DepinReleaseKeeper) {
	k.depinKeeper = dk
}

func NewKeeper(
	cdc codec.BinaryCodec,
	storeKey storetypes.StoreKey,
	ps paramtypes.Subspace,
	bankKeeper types.BankKeeper,
	accountKeeper types.AccountKeeper,
) *Keeper {
	if !ps.HasKeyTable() {
		ps = ps.WithKeyTable(types.ParamKeyTable())
	}

	return &Keeper{
		cdc:           cdc,
		storeKey:      storeKey,
		paramstore:    ps,
		bankKeeper:    bankKeeper,
		accountKeeper: accountKeeper,
	}
}

func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

func (k Keeper) GetParams(ctx sdk.Context) types.Params {
	var params types.Params
	k.paramstore.GetParamSet(ctx, &params)
	return params
}

func (k Keeper) SetParams(ctx sdk.Context, params types.Params) {
	k.paramstore.SetParamSet(ctx, &params)
}
