package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// 本文件内联 MobileChain DePIN + 边缘 AI 的核心经济逻辑（纯函数层）。
//
// 来源：Desktop/MC公链/mcchain_staging/depin/reward.go（A 线提炼的生产级奖励引擎）。
// 按 03b-cometbft-scaffold 的「直接内联」方案并入 B 线模块，使 x/depin 自包含、
// 可在无外部 replace 的情况下编译。逻辑与 staging 完全一致，改 staging 时同步此处。
//
// 金额语义与 Cosmos math.Int 对齐（E2）：奖励乘法关键路径统一经 math.Int 计算，
// 杜绝任意中间溢出；返回维持 int（当前量级 ≤ MaxRewardPerTask，与链上存储一致）。

// 任务类型
const (
	TaskTypeInference = "inference" // AI 推理：高价值
	TaskTypeDataLabel = "data_label" // 数据标注：中价值
	TaskTypeBandwidth = "bandwidth" // 带宽共享：低价值
)

// 经济常量（与 A 线 depin.go / staging 一致）
const (
	ContributionThreshold = 30  // 质量分数低于此值 → 贡献被拒，不发币
	MaxRewardPerTask      = 500 // 单任务奖励上限（防单次刷量）
)

// RewardRate 返回任务类型的奖励系数。
// inference 5x、data_label 3x、bandwidth 1x；未知类型按 1x（安全默认）。
func RewardRate(taskType string) int {
	switch taskType {
	case TaskTypeInference:
		return 5
	case TaskTypeDataLabel:
		return 3
	case TaskTypeBandwidth:
		return 1
	default:
		return 1
	}
}

// ComputeReward 计算一条已验证贡献的代币奖励。
//
// 规则（与 A 线 verifyTask / staging 等价）：
//   - score 不在 [0,100] → 0（非法打分）
//   - score < ContributionThreshold → 0（贡献被拒，不发币）
//   - 否则 reward = score * RewardRate(taskType)，并封顶 MaxRewardPerTask
//
// 返回值恒为非负且 <= MaxRewardPerTask，调用方无需再次夹紧。
func ComputeReward(score int, taskType string) int {
	if score < 0 || score > 100 {
		return 0
	}
	if score < ContributionThreshold {
		return 0
	}
	// E2：关键乘法经 sdk.Int（cosmos v0.47 的 math.Int 等价类型），杜绝中间溢出（即便当前量级极小）。
	scoreInt := sdk.NewInt(int64(score))
	rateInt := sdk.NewInt(int64(RewardRate(taskType)))
	r := scoreInt.Mul(rateInt)
	capInt := sdk.NewInt(int64(MaxRewardPerTask))
	if r.GT(capInt) {
		r = capInt
	}
	if r.IsNegative() {
		r = sdk.ZeroInt()
	}
	return int(r.Int64())
}

// IsValidTaskType 校验任务类型是否受支持（防止非法类型绕过量级）。
func IsValidTaskType(taskType string) bool {
	switch taskType {
	case TaskTypeInference, TaskTypeDataLabel, TaskTypeBandwidth:
		return true
	default:
		return false
	}
}
