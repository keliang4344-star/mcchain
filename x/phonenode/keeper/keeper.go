package keeper

import (
	"fmt"

	"github.com/cometbft/cometbft/libs/log"
	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"

	"mcchain/x/phonenode/types"
)

type (
	Keeper struct {
		cdc            codec.BinaryCodec
		storeKey       storetypes.StoreKey
		memKey         storetypes.StoreKey
		paramstore     paramtypes.Subspace
		bankKeeper     types.BankKeeper
		stakingKeeper  types.StakingKeeper
		slashingKeeper types.SlashingKeeper
		// depinKeeper 提供设备池每日释放额度闸门。depin 依赖 phonenode，
		// 故 phonenode 必须先于 depin 构造，此依赖只能事后经 SetDepinKeeper 注入。
		depinKeeper types.DepinReleaseKeeper
	}
)

func NewKeeper(
	cdc codec.BinaryCodec,
	storeKey,
	memKey storetypes.StoreKey,
	ps paramtypes.Subspace,
	bankKeeper types.BankKeeper,
	stakingKeeper types.StakingKeeper,
	slashingKeeper types.SlashingKeeper,
) *Keeper {
	// set KeyTable if it has not already been set
	if !ps.HasKeyTable() {
		ps = ps.WithKeyTable(types.ParamKeyTable())
	}

	return &Keeper{
		cdc:            cdc,
		storeKey:       storeKey,
		memKey:         memKey,
		paramstore:     ps,
		bankKeeper:     bankKeeper,
		stakingKeeper:  stakingKeeper,
		slashingKeeper: slashingKeeper,
	}
}

// SetDepinKeeper 在 depin keeper 构造完成后回填设备池释放闸门。
//
// 铁律：调用它之后才可以构造 phonenode 的 AppModule——AppModule 以「值」持有
// keeper，先构造就会拷贝到一个 depinKeeper 为 nil 的副本，BeginBlock 里的津贴
// 分发会静默退回「无闸门直发」。app.go 已按此顺序接线。
func (k *Keeper) SetDepinKeeper(dk types.DepinReleaseKeeper) {
	k.depinKeeper = dk
}

func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}
