package keeper

import (
	"encoding/json"
	"fmt"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	"mcchain/x/mcchain/types"
)

// ---------------------------------------------------------------------------
// 渐进治理移交（Progressive Governance Handover）
//
// 目标：把链的治理权从创始团队（多签）按「可预期、可审计、不可突袭」的方式移交给
// 新的治理主体（社区 DAO / 新多签）。核心是两阶段 + 时间锁：
//
//	InitiateHandover  -> 登记新治理地址，并把生效高度锁定在 当前高度 + TimelockBlocks
//	CompleteHandover  -> 只有越过生效高度后才允许执行，执行一次即终态（Executed）
//
// 时间锁窗口内，任何观察者都能读到「谁将接管、何时接管」，从而有充足时间退出或反对。
//
// 配置不进 proto Params（无代码生成工具），改为 JSON 持久化于模块 KVStore，
// 与 x/phonenode 的 NodeAllowanceConfig 保持同一套模式。
// ---------------------------------------------------------------------------

// GovernanceHandoverConfig 渐进治理移交配置（JSON 持久化于模块 KVStore）。
type GovernanceHandoverConfig struct {
	// Enabled 是否开启移交流程；关闭时任何发起/执行都不生效。
	Enabled bool `json:"enabled"`
	// CurrentGovernor 当前治理主体地址（bech32）。链上消息 MsgInitiateHandover /
	// MsgCompleteHandover 的签名者必须等于该地址；为空表示尚未配置治理主体，
	// 此时不接受任何链上移交消息（只能通过创世/升级由 SetGovernanceHandoverConfig 设定）。
	CurrentGovernor string `json:"current_governor"`
	// TimelockBlocks 时间锁长度（区块数），发起到可执行之间的强制冷静期。
	TimelockBlocks uint64 `json:"timelock_blocks"`
	// RequiredSigners 执行移交所需的多签阈值（与团队多签配合使用）。
	RequiredSigners uint32 `json:"required_signers"`
	// NewGovernor 待接管的新治理地址（bech32）；为空表示当前无待决移交。
	NewGovernor string `json:"new_governor"`
	// ActivationHeight 时间锁到期高度，达到该高度后方可 CompleteHandover。
	ActivationHeight int64 `json:"activation_height"`
	// Executed 移交是否已完成（终态，不可回退）。
	Executed bool `json:"executed"`
}

// DefaultGovernorAddress 返回 v1 阶段的默认治理主体：链上治理模块账户。
//
// 若 DefaultGovernanceHandoverConfig 是
//
//	Enabled=false + CurrentGovernor=""
//
// 且全仓没有任何路径写入该配置（既无 InitGenesis 播种，也无治理入口修改），
// 于是 GetGovernanceHandoverConfig 永远返回这个默认值 ——
//   - InitiateHandover 第一步就 `if !cfg.Enabled { return error }`；
//   - 即使绕过 Enabled，AssertGovernanceAuthority 也会因为 CurrentGovernor=="" 而拒绝。
//
// 结果是白皮书承诺的「渐进治理移交」在代码上 100% 不可达：CLI 存在、keeper 存在、
// 时间锁存在，但没有任何一次调用可能成功。这属于承诺与实现不一致，必须修掉。
//
// 修法：把当前治理主体锚定为**链上治理模块账户**（由模块名确定性派生，
// 全网一致、无需部署参数注入、不引入任何新的特权私钥）。于是：
//   - 发起移交 = 一条包含 MsgInitiateHandover 的治理提案通过的产物；
//   - 完成移交 = 时间锁到期后第二条治理提案通过的产物。
//   - 普通账户无法直接广播这两条消息：GetSigners() 返回 Authority，
//     ante 验签要求 gov 模块账户签名，而模块账户没有私钥，物理上签不出来；
//     提案执行路径不经过 ante，因此提案是唯一可达通道。
//
// 这比「团队多签」更严格也更透明（白皮书 v1 口径为 governance multisig enforced），
// 同时把「谁将接管、何时接管」完整暴露在链上投票与时间锁窗口内。
func DefaultGovernorAddress() string {
	return authtypes.NewModuleAddress(govtypes.ModuleName).String()
}

// 移交多签阈值的合法区间。
//
// 两端都是治理死锁，因此必须对称地拦住：
//   - 下界 1：等于取消了多签约束（任何人可发起移交）；
//   - 上界 10：超过团队多签的实际持钥数，阈值永远无法达成，移交被永久锁死。
const (
	MinGovernanceHandoverSigners uint32 = 1
	MaxGovernanceHandoverSigners uint32 = 10
)

// 默认移交参数。抽成常量（而不是从 DefaultGovernanceHandoverConfig() 里读字段）
// 是为了让 normalizeHandoverConfig 完全独立于地址前缀—— 见下方说明。
const (
	// DefaultGovernanceTimelockBlocks 默认时间锁块数。
	// 时间锁口径（与全网出块参数对齐，勿凭 1s 假设误读）：主网 timeout_commit = 4s，
	// 43200 块 = 43200 × 4s = 48 小时。即「提案通过与实际接管之间强制留出 48 小时
	// 的公开可见窗口」，窗口内任何人都能读到待接管地址并有充足时间退出或反对。
	DefaultGovernanceTimelockBlocks uint64 = 43200
	// DefaultGovernanceRequiredSigners 默认移交多签阈值。
	DefaultGovernanceRequiredSigners uint32 = 3
)

// DefaultGovernanceHandoverConfig 默认配置：开启移交流程，治理主体为链上治理模块账户，
// 时间锁 43200 块。
//
// 【必须保持为函数，绝不能退回包级 var】
//
// 若它是 `var DefaultGovernanceHandoverConfig = GovernanceHandoverConfig{CurrentGovernor: DefaultGovernorAddress, ...}`。
// 包级变量的初始化表达式在 **main() 之前**求值，此时 `cmd/mcchaind/cmd.initSDKConfig()`
// 还没跑，全局 bech32 前缀仍是 SDK 默认的 "cosmos"，于是这里固化出
// `cosmos10d07y265gmmuvt4z0w9aw880jnsr700j6zn9kn`。
//
// 更致命的是：`sdk.AccAddress.String()` 走一个**只按原始地址字节做 key、不区分前缀**
// 的全局 LRU 缓存（types/address.go 的 accAddrCache，60000 条，且 SetBech32PrefixForAccount
// 不会失效它）。这一次提前调用把「cosmos1...」永久钉进缓存，导致此后
// `app.New()` 里 11 处 `authtypes.NewModuleAddress(govtypes.ModuleName).String()`
// 全部返回 cosmos 前缀，`bankkeeper.NewBaseKeeper` 的 authority 校验直接
// panic("invalid bank authority address: invalid Bech32 prefix; expected mc, got cosmos")
// —— 整条链无法启动（且 build / vet / test 全绿也发现不了）。
//
// 改为函数后，求值被推迟到「真正使用它的时候」，那时前缀早已是 mc，问题消失。
func DefaultGovernanceHandoverConfig() GovernanceHandoverConfig {
	return GovernanceHandoverConfig{
		Enabled:          true,
		CurrentGovernor:  DefaultGovernorAddress(),
		TimelockBlocks:   DefaultGovernanceTimelockBlocks,
		RequiredSigners:  DefaultGovernanceRequiredSigners,
		NewGovernor:      "",
		ActivationHeight: 0,
		Executed:         false,
	}
}

// GetGovernanceHandoverConfig 读取移交配置；未设置或解析失败返回确定性默认值。
//
// 额外做一次「零值归一化」——当读到的配置是 Enabled=false 且 CurrentGovernor=="" 时，
// 判定为「历史遗留的未初始化状态」（该组合不可能是任何有意义的治理意图，因为它同时
// 关闭了流程且没有主体），统一回落到默认值。这样即便某条链在修复前已经导出过创世，
// 升级后移交路径也会自动变为可达，而不是继续停在死开关上。
func (k Keeper) GetGovernanceHandoverConfig(ctx sdk.Context) GovernanceHandoverConfig {
	bz := ctx.KVStore(k.storeKey).Get(types.GovernanceHandoverConfigKey)
	if bz == nil {
		return DefaultGovernanceHandoverConfig()
	}
	var cfg GovernanceHandoverConfig
	if err := json.Unmarshal(bz, &cfg); err != nil {
		return DefaultGovernanceHandoverConfig()
	}
	return normalizeHandoverConfig(cfg)
}

// HasGovernanceHandoverConfig 报告 KVStore 中是否已存在移交配置记录。
//
// 与 GetGovernanceHandoverConfig 的区别在于「键是否存在」与「读到的值」：
// Get 在键缺失时返回默认值（隐式路径，适合业务逻辑），
// Has 用于创世路径判断「是否需要播种默认值」—— 若已有记录则不覆盖，
// 否则一次导出/导入会把链上已生效的治理移交状态悄悄重置回默认值
func (k Keeper) HasGovernanceHandoverConfig(ctx sdk.Context) bool {
	return ctx.KVStore(k.storeKey).Has(types.GovernanceHandoverConfigKey)
}

// normalizeHandoverConfig 把移交配置补齐为可用状态（零值归一化）。
//
// 抽成纯函数有两个目的：
//  1. Get / Set 两条路径共用同一套规则，避免边界校验只做在写入侧而读取侧漏掉
//     （Set 校验 RequiredSigners ∈ [1,10] 而 Get 只处理 == 0，就是这种漂移）；
//  2. 它只依赖入参，可被 fuzz 直接覆盖真实代码路径（见 fuzz_test.go）。
//
// 注意 RequiredSigners 的边界现在是**读写两侧统一**的 [1,10]：为 0 会让移交阈值
// 失效（任何人可发起移交），超过 10 则阈值永不达成（移交被锁死），两者都是
// 治理死锁，必须同样拦住。
func normalizeHandoverConfig(cfg GovernanceHandoverConfig) GovernanceHandoverConfig {
	// 历史遗留的未初始化状态：同时关闭流程、且没有任何主体。该组合不可能是
	// 任何有意义的治理意图，统一回落到默认值——这样即便某条链在修复前已导出过
	// 创世，升级后移交路径也会自动变为可达，而不是继续停在死开关上。
	if !cfg.Enabled && cfg.CurrentGovernor == "" && !cfg.Executed && cfg.NewGovernor == "" {
		return DefaultGovernanceHandoverConfig()
	}
	if cfg.CurrentGovernor == "" {
		cfg.CurrentGovernor = DefaultGovernorAddress()
	}
	if cfg.TimelockBlocks == 0 {
		cfg.TimelockBlocks = DefaultGovernanceTimelockBlocks
	}
	if cfg.RequiredSigners < MinGovernanceHandoverSigners ||
		cfg.RequiredSigners > MaxGovernanceHandoverSigners {
		cfg.RequiredSigners = DefaultGovernanceRequiredSigners
	}
	return cfg
}

// SetGovernanceHandoverConfig 持久化移交配置。
func (k Keeper) SetGovernanceHandoverConfig(ctx sdk.Context, cfg GovernanceHandoverConfig) {
	cfg = normalizeHandoverConfig(cfg)
	bz, err := json.Marshal(cfg)
	if err != nil {
		ctx.Logger().Error("mcchain: marshal governance handover config", "err", err)
		return
	}
	ctx.KVStore(k.storeKey).Set(types.GovernanceHandoverConfigKey, bz)
}

// InitiateHandover 发起治理移交：登记新治理地址并启动时间锁。
// 生效高度 = 当前高度 + TimelockBlocks，在此之前 CompleteHandover 一律失败。
func (k Keeper) InitiateHandover(ctx sdk.Context, newGovAddr string) error {
	cfg := k.GetGovernanceHandoverConfig(ctx)
	if !cfg.Enabled {
		return fmt.Errorf("mcchain: governance handover is disabled")
	}
	if cfg.Executed {
		return fmt.Errorf("mcchain: governance handover already executed")
	}
	if _, err := sdk.AccAddressFromBech32(newGovAddr); err != nil {
		return fmt.Errorf("mcchain: invalid new governor address %q: %w", newGovAddr, err)
	}

	cfg.NewGovernor = newGovAddr
	cfg.ActivationHeight = ctx.BlockHeight() + int64(cfg.TimelockBlocks)
	k.SetGovernanceHandoverConfig(ctx, cfg)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"mcchain.GovernanceHandoverInitiated",
		sdk.NewAttribute("new_governor", cfg.NewGovernor),
		sdk.NewAttribute("activation_height", fmt.Sprintf("%d", cfg.ActivationHeight)),
	))
	return nil
}

// CompleteHandover 完成治理移交：仅在时间锁到期后允许，执行一次即终态。
// 未启用或已执行时为幂等空操作。
func (k Keeper) CompleteHandover(ctx sdk.Context) error {
	cfg := k.GetGovernanceHandoverConfig(ctx)
	if !cfg.Enabled || cfg.Executed {
		return nil
	}
	if cfg.NewGovernor == "" {
		return fmt.Errorf("mcchain: no pending governance handover")
	}
	if ctx.BlockHeight() < cfg.ActivationHeight {
		return fmt.Errorf("mcchain: governance handover timelock not elapsed: current height %d < activation height %d",
			ctx.BlockHeight(), cfg.ActivationHeight)
	}

	cfg.Executed = true
	// 移交生效：新治理主体正式接管，后续链上治理消息以其地址为准。
	cfg.CurrentGovernor = cfg.NewGovernor
	k.SetGovernanceHandoverConfig(ctx, cfg)
	telemetry.IncrCounter(1, "mcchain", "governance_handover_completed")

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"mcchain.GovernanceHandoverCompleted",
		sdk.NewAttribute("new_governor", cfg.NewGovernor),
	))
	return nil
}

// AssertGovernanceAuthority 校验链上治理消息的签名者是否为当前治理主体。
// 未配置治理主体时一律拒绝，避免任意地址发起移交。
func (k Keeper) AssertGovernanceAuthority(ctx sdk.Context, authority string) error {
	if _, err := sdk.AccAddressFromBech32(authority); err != nil {
		return fmt.Errorf("mcchain: invalid authority address %q: %w", authority, err)
	}
	cfg := k.GetGovernanceHandoverConfig(ctx)
	if cfg.CurrentGovernor == "" {
		return fmt.Errorf("mcchain: current governor is not configured; on-chain handover messages are rejected")
	}
	if cfg.CurrentGovernor != authority {
		return fmt.Errorf("mcchain: unauthorized: expected governance authority %s, got %s", cfg.CurrentGovernor, authority)
	}
	return nil
}

// IsHandoverPending 是否存在「已发起但尚未执行」的移交。
func (k Keeper) IsHandoverPending(ctx sdk.Context) bool {
	cfg := k.GetGovernanceHandoverConfig(ctx)
	return cfg.Enabled && cfg.NewGovernor != "" && !cfg.Executed
}

// IsHandoverComplete 移交是否已完成。
func (k Keeper) IsHandoverComplete(ctx sdk.Context) bool {
	return k.GetGovernanceHandoverConfig(ctx).Executed
}
