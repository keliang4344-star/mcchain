package types

import "fmt"

// Params 承载可治理更新的运行期经济参数（质押安全滴灌 + 12 年续期）。
//
// 存储为 KVStore 中的 JSON（与 MintedSupply/Allocations 一致），不依赖 proto
// 生成；主网上线后可由治理提案更新（post-mainnet），默认值与创世常量一致，
// 保证「不改行为」的向后兼容。
//
// 注意：总量上限（TotalSupplyCap）与五池占比属创世固化设计，刻意不纳入
// Params（Q8：不可治理修改）。
type Params struct {
	// DripRatioBps 质押安全滴灌年化率（基点，默认 500 = 5% of staked）。
	DripRatioBps uint32 `json:"drip_ratio_bps"`
	// RenewalFloorAPRBps 质押池 A 耗尽后协议金库续期 APR 下限（基点，默认 100 = 1.00%）。
	RenewalFloorAPRBps uint32 `json:"renewal_floor_apr_bps"`
	// RenewalFloorAPRCeilBps 协议金库续期 APR 上限（基点，默认 200 = 2.00%）。
	RenewalFloorAPRCeilBps uint32 `json:"renewal_floor_apr_ceil_bps"`
	// DripFloorYears 质押安全滴灌保证的最短年限（默认 12）。
	DripFloorYears uint32 `json:"drip_floor_years"`
}

// DefaultParams 返回默认参数（与创世常量一致，保持既有链行为不变）。
func DefaultParams() Params {
	return Params{
		DripRatioBps:          DripRatioBps,
		RenewalFloorAPRBps:    RenewalFloorAPRBps,
		RenewalFloorAPRCeilBps: RenewalFloorAPRCeilBps,
		DripFloorYears:        uint32(DripFloorYears),
	}
}

// Validate 校验参数取值范围，非法值拒绝写入（fail-closed）。
func (p Params) Validate() error {
	if p.DripRatioBps > 10_000 {
		return fmt.Errorf("tokenomics: drip_ratio_bps %d exceeds 10000", p.DripRatioBps)
	}
	if p.RenewalFloorAPRBps > p.RenewalFloorAPRCeilBps {
		return fmt.Errorf("tokenomics: renewal_floor_apr_bps %d > ceil %d",
			p.RenewalFloorAPRBps, p.RenewalFloorAPRCeilBps)
	}
	if p.RenewalFloorAPRCeilBps > 10_000 {
		return fmt.Errorf("tokenomics: renewal_floor_apr_ceil_bps %d exceeds 10000", p.RenewalFloorAPRCeilBps)
	}
	if p.DripFloorYears == 0 || p.DripFloorYears > 100 {
		return fmt.Errorf("tokenomics: drip_floor_years %d out of range (1..100)", p.DripFloorYears)
	}
	return nil
}
