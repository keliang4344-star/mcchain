package keeper

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"mcchain/internal/daywindow"
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
	// 改为 8 字节大端高度。原 "%d" 十进制字符串键
	// 字典序 ≠ 数值序（"10000" < "9999"），BeginBlock 的过期清理无法按序
	// 截断；大端编码后前缀迭代即数值升序，可确定性删除全部过期键。
	// （主网未上线，无历史状态迁移负担。）
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(height))
	return append(DefenseBlockSubPrefix, buf...)
}

// ---------------------------------------------------------------------------
// 防线结果
// ---------------------------------------------------------------------------

// DefenseResult 记录七层防线的单次执行结果。
type DefenseResult struct {
	Passed       bool   `json:"passed"`        // 是否通过全部防线
	RejectReason string `json:"reject_reason"` // 拒绝原因（未通过时）
	FailedLayer  int    `json:"failed_layer"`  // 失败所在层（1-7，通过时为 0）
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
	// 日界必须取自区块时间。用 time.Now 会让各节点在 UTC 跨日瞬间
	// 算出不同的 dayKey，读到不同的当日累计值，准入判定不一致 → AppHash 分叉。
	dayKey := daywindow.DayKey(ctx)

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
	// 与 defenseLayer7_EconomicConstraint 必须使用同一日界口径（区块时间），
	// 否则「写入的当日累计」与「读取校验的当日累计」会落在不同 key 上，
	// 单设备日收益上限形同虚设。
	dayKey := daywindow.DayKey(ctx)

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
// 仅用于身份/认证类失败（Layer 1/2）：设备换了硬件或认证过期，历史行为
// 与当前设备无关，重置是合理的。
//
// 若任意防线失败都调用本函数，攻击者可在计数
// 接近上限时故意触发 Layer 3（频率限制，代价仅一笔注定失败的 tx 手续费）
// 来清零计数，使 MaxConsecutiveSubmissions 形同虚设。现在频率/质量/活跃度/
// 经济类失败一律不重置计数器（Layer 5 拒绝时由 HalveConsecutiveCounter
// 惩罚性减半，保证可恢复），只保留身份类失败的重置语义。
func (k Keeper) ResetConsecutiveCounter(ctx sdk.Context, deviceAddr string) {
	store := ctx.KVStore(k.storeKey)
	store.Delete(defenseConsecKey(deviceAddr))
}

// HalveConsecutiveCounter 惩罚性减半设备的连续提交计数器。
// Layer 5（活跃度异常）拒绝时调用：设备被限流但可恢复 —— 若计数永不回收，
// 超限设备将永久无法提交；减半使其在约 log2(50) ≈ 6 轮内自然回归正常区间，
// 而高频刷量脚本每被拒绝一次就损失一半进度，成本随频率指数上升。
func (k Keeper) HalveConsecutiveCounter(ctx sdk.Context, deviceAddr string) {
	store := ctx.KVStore(k.storeKey)
	key := defenseConsecKey(deviceAddr)
	bz := store.Get(key)
	if bz == nil {
		return
	}
	var count int64
	if err := json.Unmarshal(bz, &count); err != nil || count <= 0 {
		store.Delete(key)
		return
	}
	count /= 2
	if count <= 0 {
		store.Delete(key)
		return
	}
	newBz, _ := json.Marshal(count)
	store.Set(key, newBz)
}

// ---------------------------------------------------------------------------
// 过期防线状态键的有界回收
//
// 三类键若只写不删，会随链运行线性无界增长（IAVL 树持续膨胀）：
//   - DefenseBlockSub:<height(8B BE)>   每个有贡献的区块一键；
//   - DefenseDaily:<addr>:<day>          每设备每天一键（day=YYYYMMDD，尾部定长）；
//   - ReleaseDaily:<YYYY-MM-DD>          每天一键（日期串，尾部定长）。
//
// 回收策略：depin BeginBlock 每块消费固定预算（DefenseGCPerBlock 条），
// 持久化游标轮转前缀空间，删除早于保留窗口的条目。日期串字典序=时间序，
// 键尾定长日期可直接按字符串比较判定过期。
// ---------------------------------------------------------------------------

const (
	// DefenseGCPerBlock 每区块 GC 预算（三类键合计每类各自最多这么多）。
	DefenseGCPerBlock = 256

	// DefenseBlockSubRetentionBlocks DefenseBlockSub 保留窗口（1 天）。
	DefenseBlockSubRetentionBlocks int64 = daywindow.BlocksPerDay

	// defenseRetentionDays DefenseDaily / ReleaseDaily 保留天数。
	defenseRetentionDays = 2
)

var defenseGCCursorKey = []byte("DefenseGCCursor")

// BeginBlockerGC depin 模块的过期键回收入口（由 module.go BeginBlock 调用）。
// 每区块总工作量恒定，与历史实体总量无关。
func (k Keeper) BeginBlockerGC(ctx sdk.Context) {
	k.gcDefenseBlockSub(ctx)

	// YYYYMMDD（DefenseDaily 尾部）与 YYYY-MM-DD（ReleaseDaily 尾部）都按
	// 字符串比较判过期；分别用各自的保留阈值日期字符串。
	k.gcSuffixedDateKeys(ctx, DefenseDailyPrefix, 8,
		ctx.BlockTime().UTC().AddDate(0, 0, -defenseRetentionDays).Format("20060102"))
	k.gcSuffixedDateKeys(ctx, ReleaseDailyPrefix, 10,
		ctx.BlockTime().UTC().AddDate(0, 0, -defenseRetentionDays).Format("2006-01-02"))
}

// gcDefenseBlockSub 删除早于保留窗口的区块提交集合键（大端高度，升序截断）。
func (k Keeper) gcDefenseBlockSub(ctx sdk.Context) {
	store := ctx.KVStore(k.storeKey)
	threshold := ctx.BlockHeight() - DefenseBlockSubRetentionBlocks
	if threshold <= 0 {
		return
	}
	thresholdBE := make([]byte, 8)
	binary.BigEndian.PutUint64(thresholdBE, uint64(threshold))

	it := sdk.KVStorePrefixIterator(store, DefenseBlockSubPrefix)
	var keys [][]byte
	for ; it.Valid() && len(keys) < DefenseGCPerBlock; it.Next() {
		if string(it.Key()[len(DefenseBlockSubPrefix):]) >= string(thresholdBE) {
			break // 大端序，后面的都是新高度
		}
		keys = append(keys, append([]byte(nil), it.Key()...))
	}
	it.Close()
	for _, key := range keys {
		store.Delete(key)
	}
}

// gcSuffixedDateKeys 通用游标轮转 GC：键尾部为定长日期串（格式与 cutoffDate
// 一致，字典序=时间序）。每块从持久化游标处**扫描**至多 DefenseGCPerBlock 条，
// 删除其中日期早于 cutoff 的条目；非法键（长度不足/非日期）一并删除避免滞留。
//
// 游标存的是去掉前缀的相对键（prefix.Store 迭代器的键空间不含前缀）。
//
// 预算必须按「扫描条数」计，不能按「删除条数」计。
// 删除计数版本在系统稳态（绝大多数键都未过期）下，continue 不消耗预算，循环会一路
// 扫到前缀末尾 —— 单块工作量与历史实体总量成正比，最终必然超过出块时间窗。
// BeginBlock 走 InfiniteGasMeter，不受 max_gas 约束，因此表现为**全网停块且不可
// 自愈**（每个节点都卡在同一个 BeginBlock 上）。这里严格沿用 referral/cap.go
// collectStaleKeys 的写法：scanned 先判预算再自增，未过期键同样计入扫描量。
func (k Keeper) gcSuffixedDateKeys(ctx sdk.Context, pfx []byte, dateLen int, cutoffDate string) {
	store := ctx.KVStore(k.storeKey)
	cursorKey := append(append([]byte{}, defenseGCCursorKey...), pfx...)
	ps := prefix.NewStore(store, pfx)

	it := ps.Iterator(store.Get(cursorKey), nil)
	var deleteKeys [][]byte
	var next []byte
	scanned := 0
	for ; it.Valid(); it.Next() {
		if scanned >= DefenseGCPerBlock {
			next = append([]byte(nil), it.Key()...)
			break
		}
		scanned++
		key := it.Key()
		date := string(key)
		if len(date) < dateLen {
			deleteKeys = append(deleteKeys, append([]byte(nil), key...))
			continue
		}
		date = date[len(date)-dateLen:]
		if date >= cutoffDate {
			continue // 未过期：保留，但已计入本块扫描预算
		}
		deleteKeys = append(deleteKeys, append([]byte(nil), key...))
	}
	it.Close()

	for _, key := range deleteKeys {
		ps.Delete(key)
	}
	// 游标推进到本块扫描的最后一条；扫到末尾（next==nil）则复位从头。
	if next != nil {
		store.Set(cursorKey, next)
	} else {
		store.Delete(cursorKey)
	}
}
