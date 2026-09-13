package keeper

import (
	"fmt"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"mcchain/x/tokenomics/types"
)

// PoolAuditEntry 单个资金池的巡检明细。
type PoolAuditEntry struct {
	Name      string `json:"name"`
	Address   string `json:"address"`
	Allocated string `json:"allocated"` // 记账分配额
	Balance   string `json:"balance"`   // 当前链上余额
	Spent     string `json:"spent"`     // 记账额 - 余额（已拨付量）
	// OverAllocated 余额高于记账额。**不一定**是缺陷：外部向池地址直接转账
	// 也会造成这个结果，因此巡检只标记「需人工核查」，不判定为违规。
	OverAllocated bool `json:"over_allocated"`
}

// EconomyAuditReport 经济账本一致性巡检报告（只读，不停链）。
type EconomyAuditReport struct {
	Denom               string           `json:"denom"`
	MintedSupply        string           `json:"minted_supply"`
	TotalSupplyCap      string           `json:"total_supply_cap"`
	AllocatedSum        string           `json:"allocated_sum"`
	Pools               []PoolAuditEntry `json:"pools"`
	BlackHoleBalance    string           `json:"black_hole_balance"`
	FeeCollectorBalance string           `json:"fee_collector_balance"`
	Issues              []string         `json:"issues"`
}

// Consistent 报告是否无任何待核查项。
func (r EconomyAuditReport) Consistent() bool { return len(r.Issues) == 0 }

// AuditEconomy 巡检经济账本的一致性。
//
// 与 tokenomics 的 crisis 不变量（MintedSupplyInvariant / PoolSumInvariant）相比，
// 本函数刻意**更宽**：它连余额一起看，并覆盖危机不变量查不到的口径。
//
// 为什么不做成 crisis 不变量：crisis 触发即 halt 链，而这里至少有一类偏差
// （池余额高于记账额）可以被外部因素造成——任何人向池地址转账都会命中。
// 拿一个可能被无关行为触发的条件去停链，代价远大于收益。
// 因此本巡检定位为**运维工具**：定期跑、看报告、人工判断。
//
// ⚠ 复杂度 O(池数)，与设备规模无关，开销恒定；但仍不在共识路径调用。
func (k Keeper) AuditEconomy(ctx sdk.Context) EconomyAuditReport {
	denom := types.DefaultDenom
	minted := k.GetMintedSupply(ctx)
	cap := sdk.NewIntFromUint64(types.TotalSupplyCap)

	rep := EconomyAuditReport{
		Denom:          denom,
		MintedSupply:   minted.String(),
		TotalSupplyCap: cap.String(),
	}

	// 1) 发行量不越上限。
	if minted.GT(cap) {
		rep.Issues = append(rep.Issues, fmt.Sprintf(
			"已发行量 %s 超过总量上限 %s", minted, cap))
	}

	// 2) 逐池核对记账额与链上余额。
	sum := sdk.ZeroInt()
	for _, a := range k.GetAllocations(ctx) {
		alloc := sdk.NewIntFromUint64(a.AllocatedAmount)
		sum = sum.Add(alloc)
		entry := PoolAuditEntry{
			Name:      a.Name,
			Address:   a.Address,
			Allocated: alloc.String(),
			Spent:     alloc.String(),
		}
		if a.Address != "" {
			addr, err := sdk.AccAddressFromBech32(a.Address)
			if err != nil {
				rep.Issues = append(rep.Issues, fmt.Sprintf(
					"池 %s 的地址无法解析: %q", a.Name, a.Address))
			} else {
				bal := k.bankKeeper.GetBalance(ctx, addr, denom).Amount
				entry.Balance = bal.String()
				entry.Spent = alloc.Sub(bal).String()
				if bal.GT(alloc) {
					entry.OverAllocated = true
					rep.Issues = append(rep.Issues, fmt.Sprintf(
						"池 %s 余额 %s 高于记账分配额 %s —— 可能是外部转入，也可能是超发，需人工核查",
						a.Name, bal, alloc))
				}
			}
		}
		rep.Pools = append(rep.Pools, entry)
	}
	rep.AllocatedSum = sum.String()

	// 3) 五池记账合计 == 已发行量（会计口径恒等式）。
	if !sum.Equal(minted) {
		rep.Issues = append(rep.Issues, fmt.Sprintf(
			"五池记账合计 %s != 已发行量 %s", sum, minted))
	}

	// 4) 销毁与费用的可核对口径（黑洞余额 = 累计销毁量的权威来源）。
	rep.BlackHoleBalance = k.bankKeeper.GetBalance(
		ctx, types.BlackHoleAddress(), denom).Amount.String()
	rep.FeeCollectorBalance = k.bankKeeper.GetBalance(
		ctx, authtypes.NewModuleAddress(authtypes.FeeCollectorName), denom).Amount.String()

	// 输出顺序稳定，便于逐次比对。
	sort.Slice(rep.Pools, func(i, j int) bool { return rep.Pools[i].Name < rep.Pools[j].Name })
	return rep
}
