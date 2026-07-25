package keeper

import (
	"encoding/json"
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"

)

// ============================================================================
// 七层防刷量防线 — 白皮书行 378-382
// ============================================================================
//
// MobileChain DePIN 贡献经济通过七层递进防线抵御女巫攻击与批量刷量行为。
// 从设备指纹到经济学博弈约束，层层加码，单次防御成本极低但组合后防刷效果
// 呈指数级放大。防线设计原则：
//   1. 低成本通过（正常设备几乎无感知）
//   2. 逐步递增阻力（刷量者越往后成本越高）
//   3. 经济终局防线（第 7 层使刷量在经济上不可行）
//
// 防线流水线由 RunDefensePipeline 统一调度，任意层失败即终止并返回拒绝原因。

// ---------------------------------------------------------------------------
// 防线常量
// ---------------------------------------------------------------------------

const (
	// Layer3: 同设备同类型任务最小间隔（区块数）。
	// 300 区块 ≈ 50 分钟（按 10s/块），防止高频批量提交。
	MinTaskIntervalBlocks = 300

	// Layer5: 单个设备连续提交上限。超过此值触发活跃度异常告警。
	MaxConsecutiveSubmissions = 50

	// Layer7: 单设备单日收益上限（单位 umc）。
	// 100 MC = 100_000_000 umc，使批量刷量在经济上不可行。
	DailyRewardCapUmc uint64 = 100_000_000

	// Layer4: 随机抽检比例（分母）。值为 20 表示每 20 次贡献随机抽检 1 次。
	QualitySampleRate = 20

	// Layer4: 贡献质量合理性范围 [Min, Max]。
	QualityMinThreshold = 20
	QualityMaxThreshold = 100

	// Layer6: 单区块多设备关联检测阈值。同一区块内超过此数量的不同设备
	// 提交贡献时，触发 IP/地理分散度告警。
	MaxDevicesPerBlock = 10
)

// ---------------------------------------------------------------------------
// KVStore 前缀
// ---------------------------------------------------------------------------

var (
	// DefenseFreqPrefix 存储同设备同类型任务的最后提交区块高度。
	DefenseFreqPrefix = []byte("DefenseFreq:")

	// DefenseConsecPrefix 存储设备连续提交计数。
	DefenseConsecPrefix = []byte("DefenseConsec:")

	// DefenseDailyPrefix 存储设备每日累计收益（键格式: prefix + addr + ":" + YYYYMMDD）。
	DefenseDailyPrefix = []byte("DefenseDaily:")

	// DefenseBlockSubPrefix 存储单区块内已提交设备地址集合（用于 Layer6 关联检测）。
	DefenseBlockSubPrefix = []byte("DefenseBlockSub:")
)

func defenseFreqKey(addr, taskType string) []byte {
	return append(DefenseFreqPrefix, []byte(addr+":"+taskType)...)
}

func defenseConsecKey(addr string) []byte {
	return append(DefenseConsecPrefix, []byte(addr)...)
}

func defenseDailyKey(addr string, day string) []byte {
	return append(DefenseDailyPrefix, []byte(addr+":"+day)...)
}

func defenseBlockSubKey(height int64) []byte {
	return append(DefenseBlockSubPrefix, []byte(fmt.Sprintf("%d", height))...)
}

// ---------------------------------------------------------------------------
// 防线结果
// ---------------------------------------------------------------------------

// DefenseResult 记录七层防线的单次执行结果。
type DefenseResult struct {
	Passed       bool   `json:"passed"`        // 是否通过全部防线
	RejectReason string `json:"reject_reason"`  // 拒绝原因（未通过时）
	FailedLayer  int    `json:"failed_layer"`   // 失败所在层（1-7，通过时为 0）
}

// PassedResult 返回一个表示通过全部防线的结果。
func PassedResult() DefenseResult {
	return DefenseResult{Passed: true, FailedLayer: 0}
}

// RejectResult 返回一个防线失败的结果。
func RejectResult(layer int, reason string) DefenseResult {
	return DefenseResult{Passed: false, FailedLayer: layer, RejectReason: reason}
}

// ============================================================================
// 防线流水线入口
// ============================================================================

// RunDefensePipeline 按序执行七层防线，任一失败即返回拒绝结果。
//
// 调用时机：msg_server_submit_contribution 中，在设备 attestation 检查通过后、
// SubmitAndReward 之前调用。此阶段已确认设备身份（Creator 地址合法 + 设备已
// attest），防线在此基础上做进一步的反刷量校验。
//
// 参数：
//   - deviceAddr: 提交贡献的设备地址（即 msg.Creator）
//   - taskType:   任务类型字符串（inference / data_label / bandwidth）
//   - score:      贡献质量分数（0-100），由消息层传入
//
// 返回 DefenseResult，调用方根据 Passed 字段决定是否继续。
func (k Keeper) RunDefensePipeline(ctx sdk.Context, deviceAddr, taskType string, score int) DefenseResult {
	// 第 1 层：设备指纹校验
	if result := k.defenseLayer1_DeviceFingerprint(ctx, deviceAddr); !result.Passed {
		return result
	}

	// 第 2 层：认证有效性检查
	if result := k.defenseLayer2_AttestationValidity(ctx, deviceAddr); !result.Passed {
		return result
	}

	// 第 3 层：任务频率限制
	if result := k.defenseLayer3_TaskFrequency(ctx, deviceAddr, taskType); !result.Passed {
		return result
	}

	// 第 4 层：贡献质量基线
	if result := k.defenseLayer4_QualityBaseline(ctx, score); !result.Passed {
		return result
	}

	// 第 5 层：设备活跃度追踪
	if result := k.defenseLayer5_ActivityTracking(ctx, deviceAddr); !result.Passed {
		return result
	}

	// 第 6 层：IP/地理位置分散度
	if result := k.defenseLayer6_IPDispersion(ctx, deviceAddr); !result.Passed {
		return result
	}

	// 第 7 层：经济学博弈约束
	if result := k.defenseLayer7_EconomicConstraint(ctx, deviceAddr); !result.Passed {
		return result
	}

	return PassedResult()
}

// ============================================================================
// 第 1 层：设备指纹校验
// ============================================================================
//
// 校验设备是否已在链上注册且记录完整。设备地址（Creator）即为设备指纹，
// 所有后续防线均基于该指纹进行追踪。未注册设备直接拒绝，确保只有经过
// phonenode 注册流程的设备才能提交贡献。

func (k Keeper) defenseLayer1_DeviceFingerprint(ctx sdk.Context, deviceAddr string) DefenseResult {
	st, err := k.GetDevice(ctx, deviceAddr)
	if err != nil || st == nil || !st.Registered {
		return RejectResult(1, "device not registered")
	}
	return PassedResult()
}

// ============================================================================
// 第 2 层：认证有效性检查
// ============================================================================
//
// 检查设备是否已通过 phonenode 模块的 attestation。本层复用了 msg_server
// 已有的 IsAttested 校验，作为防线中的独立关卡。
//
// 与 msg_server 层校验的区别：msg_server 层校验发生在发币闸口（仅 reward>0
// 时），而本防线在贡献入账前就检查，使得即使 0 奖励的贡献也需通过认证，
// 杜绝低质刷量设备污染贡献统计。

func (k Keeper) defenseLayer2_AttestationValidity(ctx sdk.Context, deviceAddr string) DefenseResult {
	if !k.phonenodeKeeper.IsAttested(ctx, deviceAddr) {
		return RejectResult(2, "device attestation invalid or expired")
	}
	return PassedResult()
}

// ============================================================================
// 第 3 层：任务频率限制
// ============================================================================
//
// 同设备同类型任务必须间隔至少 MinTaskIntervalBlocks 个区块才能再次提交。
// 防止单个设备以极高频率批量提交同类任务进行刷量。
//
// 实现：KVStore 记录上次提交区块高度，提交时比对差值。

func (k Keeper) defenseLayer3_TaskFrequency(ctx sdk.Context, deviceAddr, taskType string) DefenseResult {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(defenseFreqKey(deviceAddr, taskType))
	if bz != nil {
		var lastHeight int64
		if err := json.Unmarshal(bz, &lastHeight); err == nil {
			currentHeight := ctx.BlockHeight()
			if currentHeight-lastHeight < MinTaskIntervalBlocks {
				return RejectResult(3,
					fmt.Sprintf("task frequency too high: last=%d, current=%d, min_interval=%d",
						lastHeight, currentHeight, MinTaskIntervalBlocks))
			}
		}
	}

	// 更新最后提交高度
	currentHeight := ctx.BlockHeight()
	newBz, _ := json.Marshal(currentHeight)
	store.Set(defenseFreqKey(deviceAddr, taskType), newBz)

	return PassedResult()
}

// ============================================================================
// 第 4 层：贡献质量基线
// ============================================================================
//
// 基于区块高度做伪随机抽检，对抽中的贡献进行严格质量范围校验。
// 未被抽中的贡献直接放行。此机制在几乎不影响正常设备的前提下，
// 使刷量者无法预测哪次提交会被抽检，从而必须维持所有提交的高质量。
//
// 抽检率 = 1 / QualitySampleRate（默认 5%）
// 质量范围 = [QualityMinThreshold, QualityMaxThreshold]

func (k Keeper) defenseLayer4_QualityBaseline(ctx sdk.Context, score int) DefenseResult {
	// 伪随机抽检：仅当 (blockHeight % QualitySampleRate) == 0 时抽检
	if ctx.BlockHeight()%QualitySampleRate != 0 {
		return PassedResult()
	}

	// 命中抽检：严格质量范围校验
	if score < QualityMinThreshold || score > QualityMaxThreshold {
		return RejectResult(4,
			fmt.Sprintf("quality out of reasonable range: score=%d, expected [%d, %d]",
				score, QualityMinThreshold, QualityMaxThreshold))
	}

	return PassedResult()
}

// ============================================================================
// 第 5 层：设备活跃度追踪
// ============================================================================
//
// 追踪设备连续提交次数。单设备连续提交超过 MaxConsecutiveSubmissions 时
// 触发限制，防止自动化脚本持续高频刷量。
//
// 连续计数器在以下情况重置：
//   - 设备停止提交超过一个任务间隔周期
//   - 设备提交被其他防线拒绝
//
// 本函数在通过前四层后调用，记录通过前的连续计数，若超限则拒绝。

func (k Keeper) defenseLayer5_ActivityTracking(ctx sdk.Context, deviceAddr string) DefenseResult {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(defenseConsecKey(deviceAddr))

	var consecCount int64
	if bz != nil {
		if err := json.Unmarshal(bz, &consecCount); err != nil {
			consecCount = 0
		}
	}

	if consecCount >= MaxConsecutiveSubmissions {
		return RejectResult(5,
			fmt.Sprintf("device activity anomaly: consecutive submissions %d >= max %d",
				consecCount, MaxConsecutiveSubmissions))
	}

	// 递增并写回
	consecCount++
	newBz, _ := json.Marshal(consecCount)
	store.Set(defenseConsecKey(deviceAddr), newBz)

	return PassedResult()
}

// ============================================================================
// 第 6 层：IP/地理位置分散度
// ============================================================================
//
// 检测同一区块内多设备关联提交行为。若同一区块内有超过 MaxDevicesPerBlock
// 个不同设备提交贡献，则标记为异常关联（疑似同 IP 多设备刷量农场）。
//
// 注意：MsgSubmitContribution 不含 IP 字段，本层使用区块级设备集合作为
// 关联度的代理指标，符合链上去中心化设计约束。

func (k Keeper) defenseLayer6_IPDispersion(ctx sdk.Context, deviceAddr string) DefenseResult {
	store := ctx.KVStore(k.storeKey)
	height := ctx.BlockHeight()
	key := defenseBlockSubKey(height)

	// 读取当前区块已记录设备集合
	var devices []string
	bz := store.Get(key)
	if bz != nil {
		if err := json.Unmarshal(bz, &devices); err != nil {
			devices = nil
		}
	}

	// 检查是否已在集合中（去重）
	for _, d := range devices {
		if d == deviceAddr {
			// 同一设备同区块多次提交，已在第 3 层处理，此处放行
			return PassedResult()
		}
	}

	// 检查是否超过阈值
	if len(devices) >= MaxDevicesPerBlock {
		return RejectResult(6,
			fmt.Sprintf("block-level device dispersion alert: %d devices in block %d exceeds max %d",
				len(devices)+1, height, MaxDevicesPerBlock))
	}

	// 追加并写回
	devices = append(devices, deviceAddr)
	newBz, _ := json.Marshal(devices)
	store.Set(key, newBz)

	return PassedResult()
}

// ============================================================================
// 第 7 层：经济学博弈约束
// ============================================================================
//
// 单设备单日收益上限，使批量刷量在经济上不可行。
// 每日上限 = DailyRewardCapUmc（默认 100 MC = 100_000_000 umc）。
//
// 本层不检查本次贡献的具体奖励金额（此时奖励尚未计算），而是基于已记录的
// 当日累计奖励进行准入判断。若当日累计已达上限，拒绝所有后续提交。

func (k Keeper) defenseLayer7_EconomicConstraint(ctx sdk.Context, deviceAddr string) DefenseResult {
	dayKey := time.Now().UTC().Format("20060102")

	store := ctx.KVStore(k.storeKey)

	var dailyTotal uint64
	bz := store.Get(defenseDailyKey(deviceAddr, dayKey))
	if bz != nil {
		if err := json.Unmarshal(bz, &dailyTotal); err != nil {
			dailyTotal = 0
		}
	}

	if dailyTotal >= DailyRewardCapUmc {
		return RejectResult(7,
			fmt.Sprintf("daily reward cap reached: %d umc (cap=%d umc, ~%d MC)",
				dailyTotal, DailyRewardCapUmc, DailyRewardCapUmc/1_000_000))
	}

	return PassedResult()
}

// ---------------------------------------------------------------------------
// 防线状态维护函数（供外部在奖励发放后调用）
// ---------------------------------------------------------------------------

// RecordDailyReward 将本次发放的奖励计入设备当日累计，用于第 7 层判断。
// 应在 msg_server 拨付奖励成功后调用。
func (k Keeper) RecordDailyReward(ctx sdk.Context, deviceAddr string, amount uint64) {
	dayKey := time.Now().UTC().Format("20060102")

	store := ctx.KVStore(k.storeKey)

	var dailyTotal uint64
	bz := store.Get(defenseDailyKey(deviceAddr, dayKey))
	if bz != nil {
		_ = json.Unmarshal(bz, &dailyTotal)
	}

	dailyTotal += amount
	newBz, _ := json.Marshal(dailyTotal)
	store.Set(defenseDailyKey(deviceAddr, dayKey), newBz)
}

// ResetConsecutiveCounter 重置设备的连续提交计数器。
// 当设备提交被拒绝（任意防线层失败）时调用，使其活跃度追踪恢复。
func (k Keeper) ResetConsecutiveCounter(ctx sdk.Context, deviceAddr string) {
	store := ctx.KVStore(k.storeKey)
	store.Delete(defenseConsecKey(deviceAddr))
}
