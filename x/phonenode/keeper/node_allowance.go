package keeper

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"mcchain/internal/daywindow"
	"mcchain/x/phonenode/types"
	tokenomicstypes "mcchain/x/tokenomics/types"
)

// ErrDailyReleaseCapReached 表示设备池当日线性释放额度已用尽。
// 这不是故障，而是释放曲线的正常限流信号：分发循环据此提前收工。
var ErrDailyReleaseCapReached = errors.New("phonenode: depin daily release cap reached")

// ---------------------------------------------------------------------------
// 节点资本津贴（建设溢价）
//
// 白皮书定义：节点资本津贴 = 设备激励池 30 MC / 节点 / 日，作为「跑设备的建设溢价」，
// 区别于 DePIN 任务奖励，旨在奖励真实运行移动节点的建设性贡献，抑制纯持币空转。
//
// 实现要点（2026-08 落地，真实可运行）：
//   - 资金来源于设备激励池（depin 模块账户），不新铸、不破 1B 上限；
//   - 以 UTC「日序号」(BlockTime/86400) 为粒度，每节点每日至多发放一次（幂等）；
//   - 由 BeginBlock 在「跨天」时统一遍历活跃节点分发一次，避免每区块重复发放；
//   - 配置（Enabled / PerDay）存于模块 KVStore，可按治理调整（经后续 Msg）。
// ---------------------------------------------------------------------------

const (
	// DefaultNodeCapitalAllowancePerDay 节点建设溢价日津贴默认值（umc）。
	// 1 MC = 1e6 umc，故 30 MC = 30_000_000 umc。
	DefaultNodeCapitalAllowancePerDay uint64 = 30_000_000
	// DefaultNodeCapitalAllowanceEnabled 是否启用节点资本津贴分发。
	DefaultNodeCapitalAllowanceEnabled = true
)

// NodeAllowanceConfig 节点资本津贴配置（JSON 持久化于模块 KVStore）。
type NodeAllowanceConfig struct {
	Enabled bool   `json:"enabled"`
	PerDay  uint64 `json:"per_day_umc"` // umc / 节点 / 日
}

// GetNodeAllowanceConfig 读取配置；未设置或解析失败返回确定性默认值。
func (k Keeper) GetNodeAllowanceConfig(ctx sdk.Context) NodeAllowanceConfig {
	bz := ctx.KVStore(k.storeKey).Get(types.NodeAllowanceConfigKey)
	if bz == nil {
		return NodeAllowanceConfig{Enabled: DefaultNodeCapitalAllowanceEnabled, PerDay: DefaultNodeCapitalAllowancePerDay}
	}
	var cfg NodeAllowanceConfig
	if err := json.Unmarshal(bz, &cfg); err != nil {
		return NodeAllowanceConfig{Enabled: DefaultNodeCapitalAllowanceEnabled, PerDay: DefaultNodeCapitalAllowancePerDay}
	}
	return normalizeAllowanceConfig(cfg)
}

// normalizeAllowanceConfig 补齐节点资本津贴配置的零值。
//
// 抽成纯函数让 Get / Set 共用同一套规则（避免校验只做在写入侧），
// 同时使它可被 fuzz 直接覆盖（见 fuzz_test.go）。
//
// PerDay 为 0 会让当日分发额归零 —— 那是「静默关闭津贴」，属误配而非任何
// 有意义的治理意图，因此统一回落到默认值。
func normalizeAllowanceConfig(cfg NodeAllowanceConfig) NodeAllowanceConfig {
	if cfg.PerDay == 0 {
		cfg.PerDay = DefaultNodeCapitalAllowancePerDay
	}
	return cfg
}

// SetNodeAllowanceConfig 持久化节点资本津贴配置。
func (k Keeper) SetNodeAllowanceConfig(ctx sdk.Context, cfg NodeAllowanceConfig) {
	cfg = normalizeAllowanceConfig(cfg)
	bz, err := json.Marshal(cfg)
	if err != nil {
		ctx.Logger().Error("phonenode: marshal node allowance config", "err", err)
		return
	}
	ctx.KVStore(k.storeKey).Set(types.NodeAllowanceConfigKey, bz)
}

// dayIndex 由区块时间推算 UTC「日序号」。
//
// 改为走 internal/daywindow 这一唯一口径来源，不再在模块内自算 /86400。
// 全链凡是要「按天」的东西（referral 日上限、depin 日释放、dex LP 激励、
// phonenode 节点津贴）都必须落在同一个日界上，否则配额、报表与对账长期错位。
func dayIndex(ctx sdk.Context) uint64 {
	return daywindow.DayIndex(ctx)
}

func (k Keeper) getLastAllowanceDay(ctx sdk.Context, addr string) (uint64, bool) {
	bz := ctx.KVStore(k.storeKey).Get(types.NodeAllowanceDayKey(addr))
	if bz == nil {
		return 0, false
	}
	var d uint64
	if err := json.Unmarshal(bz, &d); err != nil {
		return 0, false
	}
	return d, true
}

func (k Keeper) setLastAllowanceDay(ctx sdk.Context, addr string, d uint64) {
	bz, _ := json.Marshal(d)
	ctx.KVStore(k.storeKey).Set(types.NodeAllowanceDayKey(addr), bz)
}

// PayNodeCapitalAllowance 向单个节点运营者拨付当日建设溢价。
// 幂等：同「日序号」已发则跳过。资金来自设备激励池（depin 模块账户）。
func (k Keeper) PayNodeCapitalAllowance(ctx sdk.Context, operator sdk.AccAddress, di uint64) error {
	cfg := k.GetNodeAllowanceConfig(ctx)
	if !cfg.Enabled || cfg.PerDay == 0 {
		return nil
	}
	if last, ok := k.getLastAllowanceDay(ctx, operator.String()); ok && last == di {
		return nil // 当日已发，幂等跳过
	}

	// 津贴取自设备激励池，必须与 DePIN 任务奖励共用
	// 同一个「每日线性释放额度」闸门。若这条腿绕过 depin 的日上限，节点
	// 规模上量后仅津贴一项就能把 7 亿设备池提前抽干，线性释放曲线形同虚设。
	if k.depinKeeper == nil {
		// 未接线即 fail-closed：宁可不发，也绝不允许无闸门直抽设备池。
		ctx.Logger().Error("phonenode: depin release gate not wired, node capital allowance withheld",
			"node", operator.String())
		return nil
	}
	allowed, dailyCap, remaining, err := k.depinKeeper.CheckDailyReleaseCap(ctx, cfg.PerDay)
	if err != nil {
		return fmt.Errorf("phonenode: check depin daily release cap: %w", err)
	}
	if !allowed {
		ctx.EventManager().EmitEvent(sdk.NewEvent(
			"phonenode.NodeCapitalAllowanceThrottled",
			sdk.NewAttribute("node", operator.String()),
			sdk.NewAttribute("requested", fmt.Sprintf("%d", cfg.PerDay)),
			sdk.NewAttribute("daily_cap", fmt.Sprintf("%d", dailyCap)),
			sdk.NewAttribute("remaining", fmt.Sprintf("%d", remaining)),
		))
		return ErrDailyReleaseCapReached
	}

	amt := sdk.NewCoins(sdk.NewCoin(tokenomicstypes.DefaultDenom, sdk.NewIntFromUint64(cfg.PerDay)))
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, tokenomicstypes.DepinModuleName, operator, amt); err != nil {
		return fmt.Errorf("phonenode: pay node capital allowance to %s: %w", operator.String(), err)
	}
	// 拨付成功后才记账，保证「已释放」与真实出账严格一致。
	k.depinKeeper.RecordDailyRelease(ctx, cfg.PerDay)
	k.setLastAllowanceDay(ctx, operator.String(), di)
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"phonenode.NodeCapitalAllowance",
		sdk.NewAttribute("node", operator.String()),
		sdk.NewAttribute("amount", amt.String()),
		sdk.NewAttribute("day_index", fmt.Sprintf("%d", di)),
	))
	return nil
}

// DistributeNodeCapitalAllowances 每日（按 UTC 天）向活跃移动节点分发建设溢价。
//
// 不得在「跨天」的那一个区块里一次性
// 遍历全量节点并逐个转账。节点规模上量后，这一个区块要做数以百万计的转账，
// 必然超时停块——把工作量堆在单块上，本身就是不可上线的设计。
//
// 现改为有界轮转：每区块从持久化游标处取至多 MaxAllowancePerBlock 个节点处理，
// 扫到末尾自动回到开头。发放的正确性由「节点级日幂等」保证
// （PayNodeCapitalAllowance 内按日序号去重），因此同一节点在一天内被轮转命中
// 多次也只会领到一份；轮转在一天内跑完全网即可，不要求集中在某一个区块完成。
// 游标是链上状态的一部分，全网批次一致，不引入非确定性。
func (k Keeper) DistributeNodeCapitalAllowances(ctx sdk.Context) error {
	cfg := k.GetNodeAllowanceConfig(ctx)
	if !cfg.Enabled || cfg.PerDay == 0 {
		return nil
	}
	di := dayIndex(ctx)

	root := ctx.KVStore(k.storeKey)
	ps := prefix.NewStore(root, NodeKeyPrefix)

	// 先取批次再关闭迭代器，避免转账写入与迭代交错。
	it := ps.Iterator(root.Get(types.AllowanceScanCursorKey), nil)
	batch := make([]NodeState, 0, types.MaxAllowancePerBlock)
	scanned := 0
	for ; it.Valid() && scanned < types.MaxAllowancePerBlock; it.Next() {
		scanned++
		var st NodeState
		if err := json.Unmarshal(it.Value(), &st); err != nil {
			continue
		}
		batch = append(batch, st)
	}
	var next []byte
	if it.Valid() {
		next = append([]byte(nil), it.Key()...)
	}
	it.Close()

	if next != nil {
		root.Set(types.AllowanceScanCursorKey, next)
	} else {
		root.Delete(types.AllowanceScanCursorKey) // 一轮走完，下区块从头继续
	}

	for _, node := range batch {
		if !node.Registered {
			continue
		}
		// 已 jail / inactive 的节点不发放（仅奖励真实在线建设贡献）。
		if node.VerifierStatus == types.VerifierStatusJailed || node.VerifierStatus == types.VerifierStatusInactive {
			continue
		}
		// 认证必须有效（白皮书：已注册 + 认证有效 + 未被 jail 的节点才可领）。
		//
		// 身份闸门必须由 IsAttested 提升到
		// IsVerifiedAttested（TierOracle）。
		//
		// 原闸门只查「设备自签 attestation 是否有效」。自签材料任何人都能自己
		// 生成一对 secp256k1 密钥造出来，女巫成本为零。津贴是 30 MC / 节点 / 日
		// 的**无条件**现金流，且已接 depin 日释放闸门 —— 于是攻击者只要批量
		// 造节点，就能把当日设备池排放额度**全部**占满：真实设备的 DePIN 贡献
		// 拨付会被 CheckDailyReleaseCap 限流到 0，主网上线首日即出现「真实矿工
		// 零收益、女巫节点躺着领钱」。按当前日排放额度，约 3881 个女巫节点即可
		// 吃满，门槛低到不可接受。
		//
		// 提到 TierOracle 后，领取津贴需要预言机对同一 challenge 的背书签名，
		// 女巫成本从「零」变成「必须拿到一台通过 TEE 真机校验的设备」，与上述
		// 分级信任的设计口径一致。代价：预言机未上线时津贴事实上停发（资金留在
		// 设备池，方向安全），而 TEE 硬件证明本来就是主网四件套之一。
		if !k.IsVerifiedAttested(ctx, node.Address) {
			continue
		}
		opAddr, err := sdk.AccAddressFromBech32(node.Address)
		if err != nil {
			continue
		}
		if err := k.PayNodeCapitalAllowance(ctx, opAddr, di); err != nil {
			if errors.Is(err, ErrDailyReleaseCapReached) {
				// 当日设备池额度已用尽，本区块提前收工。游标已前移，下一轮从
				// 后续节点继续，长期看轮转是公平的，不会永远卡在同一批节点上。
				ctx.Logger().Info("phonenode: node capital allowance throttled by depin daily release cap",
					"day_index", di)
				break
			}
			ctx.Logger().Error("phonenode: distribute node capital allowance failed",
				"node", node.Address, "err", err)
			continue
		}
	}
	return nil
}
