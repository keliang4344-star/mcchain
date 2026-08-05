package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	depinmoduletypes "mcchain/x/depin/types"
	dexmoduletypes "mcchain/x/dex/types"
	referralmoduletypes "mcchain/x/referral/types"
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
	// 初始化运行期经济参数（默认值与创世常量一致；后续可治理更新）。
	if err := k.SetParams(ctx, types.DefaultParams()); err != nil {
		return fmt.Errorf("tokenomics: set default params: %w", err)
	}

	// 占位密钥告警（A2）：团队多签与三个基金会/早期开发拨付地址若仍走源码
	// 固定种子派生，其私钥可由任何读过源码的人还原。测试网允许如此，主网
	// 创世前必须替换为真实公钥，否则 30% 总量落在公开可推导的私钥上。
	// 这里不硬失败——否则本地开发网与 CI 都无法启动——但必须在日志里喊出来。
	if !types.TeamPubKeysConfigured() {
		ctx.Logger().Error("tokenomics: TEAM MULTISIG USES SOURCE-DERIVABLE PLACEHOLDER KEYS — never use this genesis on mainnet")
	}
	if !types.FoundationOverridesConfigured() {
		ctx.Logger().Error("tokenomics: EARLY-DEV/FOUNDATION PAYOUT ADDRESSES USE SOURCE-DERIVABLE PLACEHOLDER KEYS — replace them in x/tokenomics/types/foundation_addrs_gen.go before mainnet genesis")
	}

	// 恢复模式（幂等保护，R1 总量固化的必要条件）：
	// minted_supply 非零的创世文件来自 `export` 导出的既有链状态（链升级、
	// 分叉、重启、状态迁移都走这条路径），而非全新启动。此时代币早已在源链
	// 铸造并拨付完毕，余额随 auth/bank 创世原样带入；若在这里重放①~④的
	// 一次性铸造与拨付，总量会翻倍、vesting 账户会被二次创建，固定总量铁律
	// 当场破裂。恢复模式下只重建本模块自己的账本元数据。
	if genState.MintedSupply > 0 {
		k.SetMintedSupply(ctx, sdk.NewIntFromUint64(genState.MintedSupply))
		k.SetAllocations(ctx, genState.Allocations)
		k.SetReleaseSchedule(ctx, genState.Release)
		return nil
	}

	denom := genState.Denom

	// ① 一次性铸造总量上限到 tokenomics 模块账户（R1：总量固化）。
	capCoins := sdk.NewCoins(sdk.NewCoin(denom, sdk.NewIntFromUint64(genState.TotalSupplyCap)))
	if err := k.MintCoins(ctx, capCoins); err != nil {
		return fmt.Errorf("tokenomics: mint cap: %w", err)
	}

	// 解析各池拨付额（五池）。设备激励池(deviceCoins)的切分与拨付见步骤③。
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

	// ③ 设备激励池：整体 55% cap（= DepinInitialPoolSlice）由 tokenomics 模块账户托管，
	// 创世时切出「推荐返佣生态预算（ReferralEcosystemBudget，白皮书 §25，= 55% 的 15%）」
	// 拨付到 referral.EcosystemModuleAccount（供三级返佣 ClaimRewards 领取，修复「奖励领不出」P0），
	// 剩余部分注入 depin 模块账户作为 DePIN 挖矿奖励金库（= depin.DefaultInitialPool）。
	// 校验：分配额 == DepinInitialPoolSlice，且不变量
	//   DepinInitialPoolSlice - ReferralEcosystemBudget == depin.DefaultInitialPool 成立（防漂移）。
	if allocAmount(genState, types.DeviceIncentivePoolName) != types.DepinInitialPoolSlice {
		return fmt.Errorf("tokenomics: device_incentive alloc %d != DepinInitialPoolSlice %d",
			allocAmount(genState, types.DeviceIncentivePoolName), types.DepinInitialPoolSlice)
	}
	if types.DepinInitialPoolSlice-types.ReferralEcosystemBudget != depinmoduletypes.DefaultInitialPool {
		return fmt.Errorf("tokenomics: invariant broken: DepinInitialPoolSlice(%d) - ReferralEcosystemBudget(%d) != depin.DefaultInitialPool(%d)",
			types.DepinInitialPoolSlice, types.ReferralEcosystemBudget, depinmoduletypes.DefaultInitialPool)
	}

	// 3a 推荐返佣生态预算：从设备激励池切出，拨付到 referral 生态模块账户（可支出，供返佣领取）。
	ecosystemCoins := sdk.NewCoins(sdk.NewCoin(denom, sdk.NewIntFromUint64(types.ReferralEcosystemBudget)))
	if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, referralmoduletypes.EcosystemModuleAccount, ecosystemCoins); err != nil {
		return fmt.Errorf("tokenomics: fund referral ecosystem account: %w", err)
	}

	// 3b 剩余设备激励：注入 depin 模块账户（= DePIN 挖矿奖励金库）。
	deviceForDepin := allocAmount(genState, types.DeviceIncentivePoolName) - types.ReferralEcosystemBudget
	deviceCoins := sdk.NewCoins(sdk.NewCoin(denom, sdk.NewIntFromUint64(deviceForDepin)))
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

	// 基金会池：拆分为「运营流动（T0 即时，含划拨 DEX 初始流动性）」+「2 年期线性释放」。
	// 白皮书 §24：DEX 初始流动性池的 MC 部分（500 万 MC）从基金会 T0 解锁额中划拨，
	// 转账至 dex 模块账户（计入 1B 上限，链上不新铸）。剩余 T0 拨至基金会运营地址。
	foundationOpsCoins := sdk.NewCoins(sdk.NewCoin(denom, sdk.NewIntFromUint64(types.FoundationT0Unlock-types.DexInitialLiquidityMC)))
	foundationVestingCoins := sdk.NewCoins(sdk.NewCoin(denom, sdk.NewIntFromUint64(allocAmount(genState, types.FoundationPoolName)-types.FoundationT0Unlock)))
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, types.FoundationOpsAddress, foundationOpsCoins); err != nil {
		return fmt.Errorf("tokenomics: send to foundation ops address: %w", err)
	}

	// DEX 初始流动性（MC 部分）：从基金会 T0 解锁额中划拨，转账至 dex 模块账户。
	// 计入 1B 总量上限，链上不新铸（修复 DEX 创世新铸 MC 突破 10亿 上限的缺陷）。
	dexLiquidityCoins := sdk.NewCoins(sdk.NewCoin(denom, sdk.NewIntFromUint64(types.DexInitialLiquidityMC)))
	if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, dexmoduletypes.ModuleName, dexLiquidityCoins); err != nil {
		return fmt.Errorf("tokenomics: fund dex initial liquidity: %w", err)
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
