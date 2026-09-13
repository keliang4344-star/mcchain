package keeper

import (
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/tokenomics/types"
)

// EmitChainHealthMetrics 每块导出链级健康指标（Prometheus gauge）。
//
// 为什么需要它：docs/ALERTS.md §1/§5 的告警规则引用了
// mcchain_latest_block_height / mcchain_tokenomics_minted_supply /
// mcchain_tokenomics_total_supply_cap / mcchain_bank_total_supply，但若全链
// 没有任何代码生产这些指标——告警规则是纸面文章，节点停块、供应异常都不会
// 触发任何告警。指标必须由链上每块刷新，tokenomics 的 BeginBlock 每块都会
// 执行（经济心跳），是成本最低的挂点。
//
// 暴露路径：app.toml [telemetry] enabled=true 后经 /metrics（默认 :26661）
// 由 deploy/prometheus-mcchain.yml 的 mcchain-app job 抓取，
// 告警规则见 deploy/mcchain_alerts.yml（与 docs/ALERTS.md 同步维护）。
//
// 精度说明：SDK telemetry 以 float32 传递。1e15 umc 量级的 float32 相对误差
// ~1e-7（约 ±100 MC），对「接近硬顶 99%」「高度停滞」这类阈值判断无影响；
// 精确对账请走 audit.go 的深度巡检（keeper 侧全精度）。
func (k Keeper) EmitChainHealthMetrics(ctx sdk.Context) {
	// §1 节点健康：高度每块推进。停块 = 该指标停止增长（BlockStalled 告警）。
	telemetry.SetGauge(float32(ctx.BlockHeight()), "mcchain", "latest_block_height")

	// §5 经济不变量：已铸造量与硬顶的比值。
	telemetry.SetGauge(gaugeFromInt(sdk.NewIntFromUint64(types.TotalSupplyCap)),
		"mcchain", "tokenomics_total_supply_cap")
	telemetry.SetGauge(gaugeFromInt(k.GetMintedSupply(ctx)),
		"mcchain", "tokenomics_minted_supply")

	// 口径的链上真实总供应（含 add-genesis-account 的凭空记账部分）。
	// 生产链该值 == minted（五池之外无注资）；若显著大于 minted，说明有人
	// 在创世/治理路径外向账本注入了余额——这正是 供应上限要拦的形态。
	telemetry.SetGauge(gaugeFromInt(k.bankKeeper.GetSupply(ctx, types.DefaultDenom).Amount),
		"mcchain", "bank_total_supply")
}

// gaugeFromInt 把 sdk.Int 安全地转为 telemetry 所需的 float32。
//
// 本链所有被观测的量（≤2e15 umc）都远小于 int63 上界，IsInt64 必然成立；
// 守卫只为防御性——万一未来观测更大的量，宁可指标失真（置 0）也不能在
// BeginBlock 里 panic（那等于全网停机）。
func gaugeFromInt(i sdk.Int) float32 {
	if !i.IsInt64() {
		return 0
	}
	return float32(i.Int64())
}
