package keeper

import (
	"encoding/json"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/tokenomics/types"
)

// KeyParams 运行期经济参数存储键（JSON 编码）。
var KeyParams = []byte("Params")

// GetParams 读取运行期经济参数；未设置时返回默认值（与创世常量一致），
// 保证链升级前部署的既有状态行为不变（向后兼容）。
func (k Keeper) GetParams(ctx sdk.Context) types.Params {
	bz := ctx.KVStore(k.storeKey).Get(KeyParams)
	if bz == nil {
		return types.DefaultParams()
	}
	var p types.Params
	if err := json.Unmarshal(bz, &p); err != nil {
		k.Logger(ctx).Error("tokenomics: corrupt params, falling back to defaults",
			"err", err.Error())
		return types.DefaultParams()
	}
	return p
}

// SetParams 持久化运行期经济参数；非法值拒绝写入（fail-closed）。
func (k Keeper) SetParams(ctx sdk.Context, p types.Params) error {
	if err := p.Validate(); err != nil {
		return err
	}
	bz, err := json.Marshal(p)
	if err != nil {
		return err
	}
	ctx.KVStore(k.storeKey).Set(KeyParams, bz)
	return nil
}
