package keeper

import (
	"encoding/json"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ============================================================================
// 共振分发算法 — 白皮书行 366-376
// ============================================================================
//
// MobileChain DePIN 的共振分发算法根据设备声誉、网络负载和贡献质量三个维度
// 动态调整单次贡献的基础奖励，使奖励呈非线性共振分布。核心设计目标：
//   - 高质量贡献在网络闲时获得最高倍数奖励（激励优质设备错峰提交）
//   - 低质量贡献在网络忙时获得最低倍数（抑制刷量，降低无效负载）
//   - 倍数范围 [0.5x, 1.5x]，确保经济模型不会极端波动
//
// 共振公式（白皮书行 373）：
//   adjusted = baseReward * resonanceMultiplier
//   multiplier = 0.5 + qualityFactor * 0.5 + (1.0 - loadFactor) * 0.5
//   clamped to [0.5, 1.5]

// ---------------------------------------------------------------------------
// 共振常量
// ---------------------------------------------------------------------------

const (
	// ResonanceMultiplierMin 共振倍数下限。
	ResonanceMultiplierMin = 0.5

	// ResonanceMultiplierMax 共振倍数上限。
	ResonanceMultiplierMax = 1.5

	// NetworkLoadWindowBlocks 网络负载估算窗口（区块数）。
	// 统计最近 N 个区块内的贡献提交数，与理论最大值对比得出负载因子。
	NetworkLoadWindowBlocks = 100

	// NetworkLoadMaxPerBlock 单区块理论最大贡献提交数（用于归一化）。
	NetworkLoadMaxPerBlock = 50

	// ReputationWeight 设备声誉在共振计算中的权重。
	// 声誉 = min(1.0, taskCount / ReputationNormalizer)
	ReputationNormalizer = 1000
)

// ---------------------------------------------------------------------------
// 共振分发核心函数
// ============================================================================

// ComputeResonanceReward 根据设备声誉、网络负载和贡献质量计算调整后奖励。
//
// 参数：
//   - baseReward:          基础奖励（由 ComputeReward 计算的原始金额，单位 umc）
//   - deviceTaskCount:     设备历史贡献任务总数（用于衡量声誉）
//   - networkLoad:         网络负载因子 [0.0, 1.0]，0=完全空闲，1=完全饱和
//   - contributionQuality: 贡献质量归一化因子 [0.0, 1.0]，由 score/100 得出
//
// 返回值：调整后的奖励金额（int，单位 umc），范围 [0.5 * baseReward, 1.5 * baseReward]。
//
// 算法（白皮书行 373）：
//
//	qualityFactor = 0.3 * contributionQuality + 0.2 * reputationFactor
//	multiplier   = 0.5 + qualityFactor * 0.5 + (1.0 - networkLoad) * 0.5
//	multiplier   = clamp(multiplier, 0.5, 1.5)
//	adjusted     = int(float64(baseReward) * multiplier)
func ComputeResonanceReward(baseReward int, deviceTaskCount int, networkLoad float64, contributionQuality float64) int {
	if baseReward <= 0 {
		return 0
	}

	// 计算声誉因子：任务数越多声誉越高，上限 1.0
	reputationFactor := float64(deviceTaskCount) / float64(ReputationNormalizer)
	if reputationFactor > 1.0 {
		reputationFactor = 1.0
	}

	// 综合质量因子：贡献质量占 60%，设备声誉占 40%
	qualityFactor := 0.6*contributionQuality + 0.4*reputationFactor

	// clamp 输入范围
	if contributionQuality < 0.0 {
		contributionQuality = 0.0
	}
	if contributionQuality > 1.0 {
		contributionQuality = 1.0
	}
	if networkLoad < 0.0 {
		networkLoad = 0.0
	}
	if networkLoad > 1.0 {
		networkLoad = 1.0
	}

	// 共振倍数公式
	multiplier := ResonanceMultiplierMin +
		qualityFactor*0.5 +
		(1.0-networkLoad)*0.5

	// 钳制范围
	if multiplier < ResonanceMultiplierMin {
		multiplier = ResonanceMultiplierMin
	}
	if multiplier > ResonanceMultiplierMax {
		multiplier = ResonanceMultiplierMax
	}

	adjusted := int(float64(baseReward) * multiplier)
	if adjusted < 0 {
		adjusted = 0
	}

	return adjusted
}

// ============================================================================
// Keeper 方法：网络负载估算
// ============================================================================

// EstimateNetworkLoad 估算当前网络负载因子 [0.0, 1.0]。
//
// 实现：读取最近 NetworkLoadWindowBlocks 个区块的设备提交数存储记录，
// 取平均值后除以单区块理论最大值，得到归一化负载因子。
//
// 若无历史数据（网络刚启动），返回 0.0（最低负载，奖励最大化）。
func (k Keeper) EstimateNetworkLoad(ctx sdk.Context) float64 {
	store := ctx.KVStore(k.storeKey)
	currentHeight := ctx.BlockHeight()

	startHeight := currentHeight - NetworkLoadWindowBlocks
	if startHeight < 1 {
		startHeight = 1
	}

	totalSubmissions := 0
	validBlocks := 0

	for h := startHeight; h <= currentHeight; h++ {
		key := defenseBlockSubKey(h)
		bz := store.Get(key)
		if bz == nil {
			continue
		}
		var devices []string
		if err := json.Unmarshal(bz, &devices); err == nil {
			totalSubmissions += len(devices)
			validBlocks++
		}
	}

	if validBlocks == 0 {
		return 0.0 // 空网络，最低负载
	}

	avgPerBlock := float64(totalSubmissions) / float64(validBlocks)
	load := avgPerBlock / float64(NetworkLoadMaxPerBlock)

	if load > 1.0 {
		load = 1.0
	}

	return load
}

// ============================================================================
// Keeper 方法：端到端共振奖励计算
// ============================================================================

// ComputeResonanceRewardWithContext 是 ComputeResonanceReward 的上下文包装。
//
// 自动从链上状态提取设备声誉（任务数）和网络负载（区块提交统计），
// 结合贡献分数计算最终的共振调整奖励。
//
// 调用方只需传入基础奖励和设备地址，无需手动计算各因子。
func (k Keeper) ComputeResonanceRewardWithContext(ctx sdk.Context, baseReward int, deviceAddr string, score int) int {
	if baseReward <= 0 {
		return 0
	}

	// 设备声誉 = 历史任务数
	taskCount := 0
	st, err := k.GetDevice(ctx, deviceAddr)
	if err == nil && st != nil {
		taskCount = st.TaskCount
	}

	// 贡献质量 = score / 100
	contributionQuality := float64(score) / 100.0
	if contributionQuality > 1.0 {
		contributionQuality = 1.0
	}

	// 网络负载 = 区块提交密度
	networkLoad := k.EstimateNetworkLoad(ctx)

	// 共振计算
	adjusted := ComputeResonanceReward(baseReward, taskCount, networkLoad, contributionQuality)

	// 记录审计日志
	k.Logger(ctx).Debug("resonance reward computed",
		"device", deviceAddr,
		"base_reward", baseReward,
		"task_count", taskCount,
		"score", score,
		"quality_factor", contributionQuality,
		"network_load", networkLoad,
		"adjusted_reward", adjusted,
		"multiplier", float64(adjusted)/float64(baseReward),
	)

	return adjusted
}
