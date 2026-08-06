package keeper

import (
	"encoding/json"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/internal/safemath"
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
//   adjusted   = baseReward * resonanceMultiplier
//   multiplier = 0.5 + qualityFactor * 0.5 + (1.0 - loadFactor) * 0.5
//   clamped to [0.5, 1.5]
//
// ----------------------------------------------------------------------------
// FLOAT-1 修复（分叉级）：本文件此前全部使用 float64 计算发币金额。
//
// Go 语言规范明确允许实现将多个浮点运算融合为单条指令（FMA），
// 且允许跨语句融合：arm64 / ppc64 / s390x / riscv64 上编译器会把
// `a*0.5 + b*0.5` 编译成 FMA，而 amd64 默认不融合。两者结果可相差 1 ULP。
// 该差值经 `int(float64(baseReward) * multiplier)` 截断后会放大为 ±1 umc，
// 导致 x86 验证人与 ARM 验证人向同一设备转账的金额不同 →
// AppHash 不一致 → 全网停链。
//
// 现全部改用 sdk.Dec（18 位定点十进制，纯整数大数实现），
// 在所有架构、所有 Go 版本上逐位可复现，是 Cosmos 生态处理链上比率的标准做法。
// ============================================================================

// ---------------------------------------------------------------------------
// 共振常量
// ---------------------------------------------------------------------------

const (
	// NetworkLoadWindowBlocks 网络负载估算窗口（区块数）。
	// 统计最近 N 个区块内的贡献提交数，与理论最大值对比得出负载因子。
	NetworkLoadWindowBlocks = 100

	// NetworkLoadMaxPerBlock 单区块理论最大贡献提交数（用于归一化）。
	NetworkLoadMaxPerBlock = 50

	// ReputationNormalizer 设备声誉归一化分母。
	// 声誉 = min(1.0, taskCount / ReputationNormalizer)
	ReputationNormalizer = 1000

	// ContributionScoreMax 贡献分数满分，用于归一化 quality = score / 100。
	ContributionScoreMax = 100
)

// 共振倍数边界与权重（定点常量，避免任何浮点字面量参与共识计算）。
var (
	// ResonanceMultiplierMin 共振倍数下限 0.5。
	ResonanceMultiplierMin = sdk.NewDecWithPrec(5, 1)

	// ResonanceMultiplierMax 共振倍数上限 1.5。
	ResonanceMultiplierMax = sdk.NewDecWithPrec(15, 1)

	// qualityWeight 贡献质量在综合质量因子中的权重 0.6。
	qualityWeight = sdk.NewDecWithPrec(6, 1)

	// reputationWeight 设备声誉在综合质量因子中的权重 0.4。
	reputationWeight = sdk.NewDecWithPrec(4, 1)

	// halfDec 常量 0.5，用于共振公式中的两个 0.5 系数。
	halfDec = sdk.NewDecWithPrec(5, 1)
)

// clampUnit 将 d 钳制到 [0, 1]。
func clampUnit(d sdk.Dec) sdk.Dec {
	if d.IsNegative() {
		return sdk.ZeroDec()
	}
	if d.GT(sdk.OneDec()) {
		return sdk.OneDec()
	}
	return d
}

// ---------------------------------------------------------------------------
// 共振分发核心函数
// ---------------------------------------------------------------------------

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
//	reputationFactor = min(1, deviceTaskCount / 1000)
//	qualityFactor    = 0.6 * contributionQuality + 0.4 * reputationFactor
//	multiplier       = 0.5 + qualityFactor * 0.5 + (1.0 - networkLoad) * 0.5
//	multiplier       = clamp(multiplier, 0.5, 1.5)
//	adjusted         = truncate(baseReward * multiplier)
//
// 修复说明：入参 clamp 必须先于 qualityFactor 计算执行。
// 旧实现先用未钳制的 contributionQuality 算出 qualityFactor，之后才 clamp，
// 使钳制完全失效——传入 quality=5.0 时倍数会突破上限（虽有最终 clamp 兜底，
// 但语义与白皮书不符且掩盖了上游异常输入）。
func ComputeResonanceReward(baseReward int, deviceTaskCount int, networkLoad sdk.Dec, contributionQuality sdk.Dec) int {
	if baseReward <= 0 {
		return 0
	}

	// 先钳制全部输入到 [0, 1]，再参与运算。
	quality := clampUnit(contributionQuality)
	load := clampUnit(networkLoad)

	// 声誉因子：任务数越多声誉越高，上限 1.0。
	reputation := sdk.ZeroDec()
	if deviceTaskCount > 0 {
		reputation = sdk.NewDec(int64(deviceTaskCount)).QuoInt64(ReputationNormalizer)
		reputation = clampUnit(reputation)
	}

	// 综合质量因子：贡献质量占 60%，设备声誉占 40%。
	qualityFactor := qualityWeight.Mul(quality).Add(reputationWeight.Mul(reputation))

	// 共振倍数公式：0.5 + qualityFactor*0.5 + (1 - load)*0.5
	multiplier := ResonanceMultiplierMin.
		Add(qualityFactor.Mul(halfDec)).
		Add(sdk.OneDec().Sub(load).Mul(halfDec))

	// 钳制范围 [0.5, 1.5]
	if multiplier.LT(ResonanceMultiplierMin) {
		multiplier = ResonanceMultiplierMin
	}
	if multiplier.GT(ResonanceMultiplierMax) {
		multiplier = ResonanceMultiplierMax
	}

	// TruncateInt 向零取整，与旧实现的 int() 截断语义一致，且完全确定。
	adjusted := sdk.NewDec(int64(baseReward)).Mul(multiplier).TruncateInt()
	if adjusted.IsNegative() {
		return 0
	}

	// OVF-1：baseReward 为导出函数入参，理论上可接近 MaxInt64；乘以 1.5 倍后
	// Int64() 会 panic。用饱和钳制取代裸转换，并夹到平台 int 值域（32 位平台安全）。
	return safemath.ClampToInt(adjusted)
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
//
// FLOAT-1：返回 sdk.Dec 而非 float64，保证负载因子逐位可复现。
func (k Keeper) EstimateNetworkLoad(ctx sdk.Context) sdk.Dec {
	store := ctx.KVStore(k.storeKey)
	currentHeight := ctx.BlockHeight()

	startHeight := currentHeight - NetworkLoadWindowBlocks
	if startHeight < 1 {
		startHeight = 1
	}

	totalSubmissions := int64(0)
	validBlocks := int64(0)

	for h := startHeight; h <= currentHeight; h++ {
		key := defenseBlockSubKey(h)
		bz := store.Get(key)
		if bz == nil {
			continue
		}
		var devices []string
		if err := json.Unmarshal(bz, &devices); err == nil {
			totalSubmissions += int64(len(devices))
			validBlocks++
		}
	}

	if validBlocks == 0 {
		return sdk.ZeroDec() // 空网络，最低负载
	}

	// load = (totalSubmissions / validBlocks) / NetworkLoadMaxPerBlock
	//      = totalSubmissions / (validBlocks * NetworkLoadMaxPerBlock)
	// 合并为一次除法，避免两次定点除法各自截断带来的精度损失。
	load := sdk.NewDec(totalSubmissions).QuoInt64(validBlocks * NetworkLoadMaxPerBlock)

	return clampUnit(load)
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

	// 贡献质量 = score / 100，钳制到 [0, 1]
	contributionQuality := clampUnit(sdk.NewDec(int64(score)).QuoInt64(ContributionScoreMax))

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
		"quality_factor", contributionQuality.String(),
		"network_load", networkLoad.String(),
		"adjusted_reward", adjusted,
	)

	return adjusted
}

// ResonanceMultiplierOf 返回 adjusted/base 的定点倍数，供事件与日志展示。
// 仅用于可观测性，不参与状态转换；但同样使用 sdk.Dec 以免日志与实际发放口径不一致。
func ResonanceMultiplierOf(baseReward, adjustedReward int) sdk.Dec {
	if baseReward <= 0 {
		return sdk.OneDec()
	}
	return sdk.NewDec(int64(adjustedReward)).QuoInt64(int64(baseReward))
}
