package types

import (
	"fmt"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// DefaultIndex 默认全局索引（占位，保持与 starport 约定一致）。
const DefaultIndex uint64 = 1

// DefaultGenesis 返回默认创世状态。
// 注意：ReleaseSchedule 的 start/cliff/end 依赖 genesis 区块时间，
// 在 InitGenesis 运行时由 ctx.BlockTime() 计算覆盖，此处置 0。
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Denom:          DefaultDenom,
		TotalSupplyCap: TotalSupplyCap,
		MintedSupply:   0,
		Allocations:    defaultAllocations(),
		Release:        defaultRelease(),
	}
}

// defaultAllocations 按固定占比从总量上限推导各池拨付额与地址。
func defaultAllocations() []PoolAllocation {
	cap := sdk.NewIntFromUint64(TotalSupplyCap)
	mk := func(name string, bps uint32) PoolAllocation {
		amt := cap.Mul(sdk.NewInt(int64(bps))).Quo(sdk.NewInt(10000)).Uint64()
		addr := ""
		switch name {
		case TeamPoolName:
			addr = TeamAddress.String()
		case CommunityPoolName:
			addr = authtypes.NewModuleAddress(CommunityPoolName).String()
		case EcosystemPoolName:
			addr = authtypes.NewModuleAddress(EcosystemPoolName).String()
		}
		return PoolAllocation{
			Name:           name,
			PercentBps:     bps,
			AllocatedAmount: amt,
			Address:        addr,
		}
	}
	return []PoolAllocation{
		mk(TeamPoolName, TeamPercentBps),
		mk(CommunityPoolName, CommunityPercentBps),
		mk(EcosystemPoolName, EcosystemPercentBps),
	}
}

// defaultRelease 返回默认释放曲线元数据（时间字段在 InitGenesis 时覆盖）。
func defaultRelease() ReleaseSchedule {
	totalLocked := sdk.NewIntFromUint64(TotalSupplyCap).
		Mul(sdk.NewInt(int64(TeamPercentBps))).
		Quo(sdk.NewInt(10000)).
		Uint64()
	return ReleaseSchedule{
		TeamAddress: TeamAddress.String(),
		StartTime:   0,
		CliffTime:   0,
		EndTime:     0,
		TotalLocked: totalLocked,
	}
}

// Validate 执行基础创世状态校验（R1/R2）。
func (gs GenesisState) Validate() error {
	if gs.Denom == "" {
		return fmt.Errorf("tokenomics: denom cannot be empty")
	}
	// 总量上限必须与 Go 常量一致（双保险，Q8：不可治理修改）。
	if gs.TotalSupplyCap != TotalSupplyCap {
		return fmt.Errorf("tokenomics: total_supply_cap %d != constant %d", gs.TotalSupplyCap, TotalSupplyCap)
	}
	if uint64(len(gs.Allocations)) != 3 {
		return fmt.Errorf("tokenomics: expected 3 pool allocations, got %d", len(gs.Allocations))
	}
	var sum uint64
	var bpsSum uint32
	seen := make(map[string]bool)
	for _, a := range gs.Allocations {
		if a.Name == "" {
			return fmt.Errorf("tokenomics: allocation name cannot be empty")
		}
		if seen[a.Name] {
			return fmt.Errorf("tokenomics: duplicate allocation %q", a.Name)
		}
		seen[a.Name] = true
		if a.PercentBps == 0 {
			return fmt.Errorf("tokenomics: allocation %q percent_bps must be positive", a.Name)
		}
		sum += a.AllocatedAmount
		bpsSum += a.PercentBps
	}
	if bpsSum != 10000 {
		return fmt.Errorf("tokenomics: percent_bps sum %d != 10000", bpsSum)
	}
	if sum != gs.TotalSupplyCap {
		return fmt.Errorf("tokenomics: allocation amount sum %d != total_supply_cap %d", sum, gs.TotalSupplyCap)
	}
	if gs.Release.TeamAddress == "" {
		return fmt.Errorf("tokenomics: release team_address cannot be empty")
	}
	return nil
}
