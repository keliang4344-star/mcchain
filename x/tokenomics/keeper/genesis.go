package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/tokenomics/types"
)

// allocAmount 从创世分配列表中按池名取出拨付额。
func allocAmount(gs types.GenesisState, name string) uint64 {
	for _, a := range gs.Allocations {
		if a.Name == name {
			return a.AllocatedAmount
		}
	}
	return 0
}

// InitGenesis 执行 tokenomics 模块的创世编排（R1/R2）。
// 顺序：①一次性铸 cap → ②团队 vesting 账户 + 拨付 → ③社区/生态拨付 →
// ④生态→depin 切片 → ⑤写 Allocations/ReleaseSchedule。
// 本方法须在 depin.InitGenesis 之前被调用（genesis 顺序铁律）。
func (k Keeper) InitGenesis(ctx sdk.Context, genState types.GenesisState) error {
	if err := genState.Validate(); err != nil {
		return err
	}
	denom := genState.Denom

	// ① 一次性铸造总量上限到 tokenomics 模块账户（R1：总量固化）。
	capCoins := sdk.NewCoins(sdk.NewCoin(denom, sdk.NewIntFromUint64(genState.TotalSupplyCap)))
	if err := k.MintCoins(ctx, capCoins); err != nil {
		return fmt.Errorf("tokenomics: mint cap: %w", err)
	}

	// 解析各池拨付额。
	teamCoins := sdk.NewCoins(sdk.NewCoin(denom, sdk.NewIntFromUint64(allocAmount(genState, types.TeamPoolName))))
	communityCoins := sdk.NewCoins(sdk.NewCoin(denom, sdk.NewIntFromUint64(allocAmount(genState, types.CommunityPoolName))))
	ecosystemCoins := sdk.NewCoins(sdk.NewCoin(denom, sdk.NewIntFromUint64(allocAmount(genState, types.EcosystemPoolName))))
	initialPoolCoins := sdk.NewCoins(sdk.NewCoin(denom, sdk.NewIntFromUint64(types.DepinInitialPoolSlice)))

	// ② 团队池：创建多签 vesting 账户（1 年 cliff + 3 年线性）并拨付。
	genesisTime := ctx.BlockTime()
	startTime := genesisTime.AddDate(1, 0, 0).Unix() // genesis + 1yr（cliff 结束，线性起点）
	endTime := genesisTime.AddDate(4, 0, 0).Unix()   // genesis + 4yr（释放结束）
	if err := k.CreateTeamVestingAccount(ctx, teamCoins, startTime, endTime); err != nil {
		return fmt.Errorf("tokenomics: create team vesting account: %w", err)
	}
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, types.TeamAddress, teamCoins); err != nil {
		return fmt.Errorf("tokenomics: send to team vesting account: %w", err)
	}

	// ③ 社区 / 生态 模块账户拨付。
	if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, types.CommunityPoolName, communityCoins); err != nil {
		return fmt.Errorf("tokenomics: send to community pool: %w", err)
	}
	if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, types.EcosystemPoolName, ecosystemCoins); err != nil {
		return fmt.Errorf("tokenomics: send to ecosystem pool: %w", err)
	}

	// ④ 生态池切片转给 depin 模块账户（InitialPool，Q7）。
	// 此时 depin 模块账户尚未经其自身 InitGenesis 处理，但余额会落地到其模块地址，
	// 随后 depin.InitGenesis 仅 SetParams，不再自铸。
	if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.EcosystemPoolName, types.DepinModuleName, initialPoolCoins); err != nil {
		return fmt.Errorf("tokenomics: send initial pool to depin: %w", err)
	}

	// ⑤ 持久化分配与释放曲线元数据（进度查询实时算，Q9）。
	k.SetAllocations(ctx, genState.Allocations)
	release := types.ReleaseSchedule{
		TeamAddress: types.TeamAddress.String(),
		StartTime:   startTime,
		CliffTime:   startTime,
		EndTime:     endTime,
		TotalLocked: allocAmount(genState, types.TeamPoolName),
	}
	k.SetReleaseSchedule(ctx, release)
	return nil
}

// ExportGenesis 导出 tokenomics 模块创世状态（从 KVStore 读取）。
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	return &types.GenesisState{
		Denom:          types.DefaultDenom,
		TotalSupplyCap: types.TotalSupplyCap,
		MintedSupply:   k.GetMintedSupply(ctx).Uint64(),
		Allocations:    k.GetAllocations(ctx),
		Release:        k.GetReleaseSchedule(ctx),
	}
}
