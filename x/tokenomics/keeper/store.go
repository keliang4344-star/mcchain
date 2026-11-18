package keeper

import (
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/tokenomics/types"
)

// KVStore 键定义（与 depin 保持一致：json 编码简单类型）。
var (
	// KeyMintedSupply 累计已发行量（sdk.Int 的字符串形式）。
	KeyMintedSupply = []byte("MintedSupply")
	// KeyAllocations 三大池分配记录（[]PoolAllocation 的 JSON）。
	KeyAllocations = []byte("Allocations")
	// KeyReleaseSchedule 团队池释放曲线元数据（ReleaseSchedule 的 JSON）。
	KeyReleaseSchedule = []byte("ReleaseSchedule")
)

// SetMintedSupply 持久化累计已发行量。
func (k Keeper) SetMintedSupply(ctx sdk.Context, minted sdk.Int) {
	ctx.KVStore(k.storeKey).Set(KeyMintedSupply, []byte(minted.String()))
}

// GetMintedSupply 读取累计已发行量（未设置返回 0）。
//
// 状态存在但无法解析属账本损坏：静默返回 0 会让 MintCoins 把累计已发行量
// 当作 0 重算，从而绕过总量硬顶（见 MintCoins 的 cap 校验）。此处与 Set 系列
// 的 fail-closed 语义一致——账本损坏宁可停机也不静默错账。
func (k Keeper) GetMintedSupply(ctx sdk.Context) sdk.Int {
	bz := ctx.KVStore(k.storeKey).Get(KeyMintedSupply)
	if bz == nil {
		return sdk.ZeroInt()
	}
	minted, ok := sdk.NewIntFromString(string(bz))
	if !ok {
		panic(fmt.Sprintf("tokenomics: corrupted minted supply state %q: not a valid integer", string(bz)))
	}
	return minted
}

// SetAllocations 持久化三大池分配记录。
func (k Keeper) SetAllocations(ctx sdk.Context, allocs []types.PoolAllocation) {
	bz, err := json.Marshal(allocs)
	if err != nil {
		panic(fmt.Sprintf("tokenomics: marshal allocations: %v", err))
	}
	ctx.KVStore(k.storeKey).Set(KeyAllocations, bz)
}

// GetAllocations 读取三大池分配记录（未设置返回 nil）。
func (k Keeper) GetAllocations(ctx sdk.Context) []types.PoolAllocation {
	bz := ctx.KVStore(k.storeKey).Get(KeyAllocations)
	if bz == nil {
		return nil
	}
	var allocs []types.PoolAllocation
	if err := json.Unmarshal(bz, &allocs); err != nil {
		panic(fmt.Sprintf("tokenomics: corrupted allocations state: %v", err))
	}
	return allocs
}

// SetReleaseSchedule 持久化团队池释放曲线元数据。
func (k Keeper) SetReleaseSchedule(ctx sdk.Context, rs types.ReleaseSchedule) {
	bz, err := json.Marshal(rs)
	if err != nil {
		panic(fmt.Sprintf("tokenomics: marshal release schedule: %v", err))
	}
	ctx.KVStore(k.storeKey).Set(KeyReleaseSchedule, bz)
}

// GetReleaseSchedule 读取团队池释放曲线元数据（未设置返回零值）。
func (k Keeper) GetReleaseSchedule(ctx sdk.Context) types.ReleaseSchedule {
	var rs types.ReleaseSchedule
	bz := ctx.KVStore(k.storeKey).Get(KeyReleaseSchedule)
	if bz == nil {
		return rs
	}
	if err := json.Unmarshal(bz, &rs); err != nil {
		panic(fmt.Sprintf("tokenomics: corrupted release schedule state: %v", err))
	}
	return rs
}
