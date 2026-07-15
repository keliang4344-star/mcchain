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

// InitGenesis 执行 tokenomics 模块的创世编排（R1/R2，五池模型）。
// 顺序：①一次性铸 cap → ②团队 vesting 账户 + 拨付 →
// ③设备激励(→depin) / 质押安全 / 基金会 / 早期开发 四池拨付 →
// ④写 Allocations/ReleaseSchedule。
// 本方法须在 depin.InitGenesis 之前被调用（genesis 顺序铁律）：
// 设备激励池即 DePIN 挖矿奖励金库，全额注入 depin 模块账户后 depin 仅 SetParams、不再自铸。
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

	// 解析各池拨付额（五池）。
	deviceCoins := sdk.NewCoins(sdk.NewCoin(denom, sdk.NewIntFromUint64(allocAmount(genState, types.DeviceIncentivePoolName))))
	stakingCoins := sdk.NewCoins(sdk.NewCoin(denom, sdk.NewIntFromUint64(allocAmount(genState, types.StakingSecurityPoolName))))
	teamCoins := sdk.NewCoins(sdk.NewCoin(denom, sdk.NewIntFromUint64(allocAmount(genState, types.TeamPoolName))))
	earlyDevCoins := sdk.NewCoins(sdk.NewCoin(denom, sdk.NewIntFromUint64(allocAmount(genState, types.EarlyDevPoolName))))

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

	// ③ 设备激励池：全额注入 depin 模块账户（= DePIN 挖矿奖励金库，55% cap）。
	// 校验分配额与 DepinInitialPoolSlice / depin.DefaultInitialPool 三者一致（防漂移）。
	if allocAmount(genState, types.DeviceIncentivePoolName) != types.DepinInitialPoolSlice {
		return fmt.Errorf("tokenomics: device_incentive alloc %d != DepinInitialPoolSlice %d",
			allocAmount(genState, types.DeviceIncentivePoolName), types.DepinInitialPoolSlice)
	}
	if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, types.DepinModuleName, deviceCoins); err != nil {
		return fmt.Errorf("tokenomics: send device incentive pool to depin: %w", err)
	}

	// ④ 质押安全（模块账户）/ 基金会（拆分 EOA）/ 早期开发（开发资助地址）拨付。
	if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, types.StakingSecurityPoolName, stakingCoins); err != nil {
		return fmt.Errorf("tokenomics: send to staking security pool: %w", err)
	}

	// 早期开发池：T0 全额拨付到开发资助地址（可支出，无锁仓）。
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, types.EarlyDevAddress, earlyDevCoins); err != nil {
		return fmt.Errorf("tokenomics: send to early dev address: %w", err)
	}

	// 基金会池：拆分为「运营流动（T0 即时 5000 万）」+「2 年期线性释放（8000 万）」。
	foundationOpsCoins := sdk.NewCoins(sdk.NewCoin(denom, sdk.NewIntFromUint64(types.FoundationT0Unlock)))
	foundationVestingCoins := sdk.NewCoins(sdk.NewCoin(denom, sdk.NewIntFromUint64(allocAmount(genState, types.FoundationPoolName)-types.FoundationT0Unlock)))
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, types.FoundationOpsAddress, foundationOpsCoins); err != nil {
		return fmt.Errorf("tokenomics: send to foundation ops address: %w", err)
	}
	foundationVestingStart := genesisTime.Unix()
	foundationVestingEnd := genesisTime.AddDate(2, 0, 0).Unix()
	if err := k.CreateVestingAccount(ctx, types.FoundationVestingAddress, types.FoundationVestingPubKey, foundationVestingCoins, foundationVestingStart, foundationVestingEnd); err != nil {
		return fmt.Errorf("tokenomics: create foundation vesting account: %w", err)
	}
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, types.FoundationVestingAddress, foundationVestingCoins); err != nil {
		return fmt.Errorf("tokenomics: send to foundation vesting address: %w", err)
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
