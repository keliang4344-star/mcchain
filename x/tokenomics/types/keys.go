package types

import (
	"crypto/sha256"
	"fmt"
	"sort"

	"github.com/cosmos/cosmos-sdk/codec/legacy"
	sdk "github.com/cosmos/cosmos-sdk/types"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	secp256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	multisig "github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

const (
	// ModuleName 模块名。
	ModuleName = "tokenomics"

	// StoreKey 模块主存储键。
	StoreKey = ModuleName

	// RouterKey 模块消息路由键（本模块无 Msg，仅占位）。
	RouterKey = ModuleName

	// ---- 五池分配（2026-07 定稿）：设备激励 55% / 质押安全 15% / 团队 12% / 基金会 13% / 早期开发 5% ----

	// DeviceIncentivePoolName 设备激励池（DePIN 挖矿奖励）分配名。
	// 资金托管于 depin 模块账户（DepinModuleName），创世全额注入，
	// 由 depin 按已验证任务逐笔拨付；不是独立模块账户，故无需额外注册 maccPerms。
	DeviceIncentivePoolName = "device_incentive"

	// StakingSecurityPoolName 质押安全池模块账户名。
	// 零通胀模型下预铸的质押/安全激励储备，经治理按计划注入分配。
	StakingSecurityPoolName = "staking_security"

	// TeamPoolName 团队池分配名（地址为 3-of-5 多签 vesting 账户）。
	TeamPoolName = "team"

	// FoundationPoolName 基金会池模块账户名（长期运营 / 战略储备）。
	FoundationPoolName = "foundation"

	// EarlyDevPoolName 早期开发 / 早期贡献者池模块账户名。
	EarlyDevPoolName = "early_dev"

	// DefaultDenom 链上主币 denom。
	DefaultDenom = "umc"

	// TotalSupplyCap 总量上限（umc），= 1e9 MC。
	// 链上常量 + Genesis 校验双保险；不进 params subspace、不治理可改（Q8）。
	TotalSupplyCap = uint64(1e15)

	// TeamMultisigThreshold 团队多签阈值（3-of-5，Q6）。
	TeamMultisigThreshold = 3

	// 分配占比（基点，10000 = 100%）：
	// 设备激励 55% / 质押安全 15% / 团队 12% / 基金会 13% / 早期开发 5%。
	// 五池占比之和恒为 10000（Genesis Validate 强校验）。
	DeviceIncentivePercentBps = uint32(5500)
	StakingSecurityPercentBps = uint32(1500)
	TeamPercentBps            = uint32(1200)
	FoundationPercentBps      = uint32(1300)
	EarlyDevPercentBps        = uint32(500)

	// DepinInitialPoolSlice 设备激励池整体额度（= 55% cap，含 DePIN 挖矿 + 推荐返佣两部分）。
	// 设备激励池即 DePIN 挖矿奖励金库，创世时切出推荐返佣预算后注入 depin 模块账户，
	// depin 实际接收额 = DepinInitialPoolSlice - ReferralEcosystemBudget，
	// 必须等于 x/depin/types.DefaultInitialPool。
	// 5.5e14 umc = 5.5e8 MC（占 10 亿总量的 55%）。
	DepinInitialPoolSlice = uint64(550_000_000_000_000)

	// ReferralEcosystemBudget 推荐返佣生态账户拨款（umc）。
	// 白皮书 line 398：推荐预算约 8,250 万 MC（= 设备激励 55% 的 15%），在 55% 之内；
	// 由 tokenomics InitGenesis 从设备激励池切出并拨付到 referral.EcosystemModuleAccount，
	// 供三级返佣 ClaimRewards 领取（修复"奖励领不出"P0）。
	ReferralEcosystemBudget = uint64(82_500_000_000_000)

	// DepinModuleName 生态切片拨付的目标模块名（C2：以常量替代字符串，编译期可查）。
	// tokenomics → depin 的跨模块转账统一引用此常量，避免隐式字符串耦合。
	DepinModuleName = "depin"

	// FoundationT0Unlock 基金会池 T0 即时解锁量（umc）。
	// 用户诉求：前期市场流通 1–2 亿体量。基金会池总额 13%（1.3e14 umc），
	// 其中 5000 万 MC（5e13 umc）在创世即时解锁到运营流动地址，
	// 剩余 8000 万 MC（8e13 umc）进入 2 年期线性释放 vesting 账户。
	// 5e13 umc = 5e7 MC = 5000 万 MC。
	FoundationT0Unlock = uint64(50_000_000_000_000)

	// DexInitialLiquidityMC DEX 初始流动性池的 MC 部分（umc），从基金会 T0 解锁额中划拨。
	// 计入 1B 总量上限（白皮书 §24），由 tokenomics 创世转账至 dex 模块账户，链上不新铸。
	DexInitialLiquidityMC = uint64(5_000_000_000_000) // 5,000,000 MC

	// ---- Protocol Treasury: the 6th independent on-chain address (finalized 2026-08) ----
	// Physically separated from the staking-security drip pool and logically linked:
	//   - genesis starts at ZERO (no pre-mint, no allocation);
	//   - funded only by (a) enterprise settlement fee inflow and (b) drip-pool A
	//     exhaustion renewal (see RenewalFloorAPR* below).
	// The staking-security pool (StakingSecurityPoolName) is code-unspendable;
	// the protocol treasury (ProtocolTreasuryPoolName) is governed (multisig + timelock).
	ProtocolTreasuryPoolName = "protocol_treasury"

	// ---- Staking-security drip: 12-year floor and A→B renewal (finalized 2026-08) ----
	// DripFloorYears is the guaranteed minimum drip duration (years).
	DripFloorYears = 12
	// DripRatioBps is the annualized drip rate as a fraction of total staked MC (5%).
	// Default value of the governable Params.DripRatioBps.
	DripRatioBps = uint32(500)
	// RenewalFloorAPRBps is the lower bound of the treasury renewal APR (1.00%).
	// Default value of the governable Params.RenewalFloorAPRBps.
	RenewalFloorAPRBps = uint32(100)
	// RenewalFloorAPRCeilBps is the upper bound of the treasury renewal APR (2.00%).
	// Default value of the governable Params.RenewalFloorAPRCeilBps.
	RenewalFloorAPRCeilBps = uint32(200)

	// ---- Black hole: the chain's canonical burn address (genesis-existent) ----
	// 全链唯一的销毁去向。所有通缩销毁一律打入该地址，不再调用 bank.BurnCoins：
	//   - 地址由模块名经 sha256 确定性派生，不存在对应私钥，数学上永不可花费；
	//   - 创世即存在，无需部署、无需治理开关，任何人可用标准 bank 查询实时核对余额；
	//   - 该地址余额即「累计已销毁量」的权威口径，链上可审计、不可伪造。
	// 销毁来源共 4 类（白皮书通缩表）：DePIN 任务赏金每笔 5% / DEX 手续费每笔 0.05% /
	// 推荐加成每笔 1% / 治理押金 10%。
	// 罚没（slashing）不属于销毁来源——罚没 100% 回流质押安全池，补贴诚实节点与验证人。
	BlackHolePoolName = "black_hole"

	// ---- Enterprise settlement fee (finalized 2026-08): 1.5% of settlement,
	//      40% to nodes (fee_collector) / 60% to protocol treasury ----
	// Charged on oracle data, device settlement, and EdgeAI inference settlements.
	// 企业侧付费是国库的主要活水来源：不新铸一分钱，全部来自真实业务收入的再分配。
	EnterpriseSettlementFeeBps    = uint32(150)  // 1.50%
	EnterpriseFeeNodeRatioBps     = uint32(4000) // 40.00% 分给节点（经 fee_collector 随出块分配）
	EnterpriseFeeTreasuryRatioBps = uint32(6000) // 60.00% 进国库（protocol_treasury，储备待建设）

	// ---- Double-sign permanent tombstone (finalized 2026-08) ----
	// Equivocation (signing two different blocks at the same height) is permanently
	// tombstoned: the validator can never re-enter the active set, even after
	// re-bonding. This is a hard on-chain rule and is non-negotiable.
	DoubleSignPermanentTombstone = true
)

// ---------------------------------------------------------------------------
// 团队 3-of-5 多签公钥配置（T1）
// ---------------------------------------------------------------------------
// teamPubKeyStrings（5 个真实 secp256k1 公钥，bech32 mcpub 前缀）由
// scripts/gen_team_keys 自动生成到同包文件 team_pubkeys_gen.go，本文件不再硬编码，
// 以避免「手写复制」导致 keys.go 与 team_multisig_vault.txt / team_keys_gen.json
// 三者漂移。对应私钥由各自 bip39 助记词恢复（见本地 team_multisig_vault.txt，
// 由多签持有者各自离线安全保管，绝不入库。必须恰好 5 个有效 bech32，否则回退占位公钥。
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// 运行时常量（init 派生）
// ---------------------------------------------------------------------------

var (
	// TeamMultisigPubKey 3-of-5 多签公钥（真实或占位）。
	TeamMultisigPubKey cryptotypes.PubKey
	// TeamAddress 团队多签 vesting 账户地址（bech32 前缀 mc）。
	TeamAddress sdk.AccAddress

	// ---- 早期流通拨付地址（2026-07 新增，对应"前期流通 1–2 亿"诉求）----
	// 早期开发池、基金会池不再停留于模块账户（nil 权限、不可支出），而是在创世时
	// 拨付到以下可支出地址，使前期流通量确定、可审计（参数写代码、开源可审计）。
	// 以下均为占位确定性地址，主网前必须替换为真实多签/运营地址（同 TeamAddress 规则）。

	// EarlyDevAddress 早期开发池拨付地址（T0 全额 5000 万，开发资助多签/运营地址占位）。
	EarlyDevAddress sdk.AccAddress
	// FoundationOpsAddress 基金会运营流动地址（T0 即时解锁 5000 万，用于市场运营/流动性/社区）。
	FoundationOpsAddress sdk.AccAddress
	// FoundationVestingAddress 基金会 2 年期线性释放地址（剩余 8000 万，T0 锁定、2 年后线性释放）。
	FoundationVestingAddress sdk.AccAddress

	// 对应占位公钥（仅测试/占位；主网前替换真实公钥）。
	EarlyDevPubKey          cryptotypes.PubKey
	FoundationOpsPubKey     cryptotypes.PubKey
	FoundationVestingPubKey cryptotypes.PubKey
)

func init() {
	pubKeys := buildTeamPubKeys()
	TeamMultisigPubKey = multisig.NewLegacyAminoPubKey(int(TeamMultisigThreshold), pubKeys)
	TeamAddress = sdk.AccAddress(TeamMultisigPubKey.Address())

	// 早期流通拨付地址：占位确定性派生（主网前替换真实地址）。
	EarlyDevPubKey, EarlyDevAddress = derivedPlaceholder(0x21)
	FoundationOpsPubKey, FoundationOpsAddress = derivedPlaceholder(0x22)
	FoundationVestingPubKey, FoundationVestingAddress = derivedPlaceholder(0x23)
}

// buildTeamPubKeys 若 teamPubKeyStrings 含 5 个有效 bech32 公钥则解析；
// 否则回退到固定种子派生占位公钥。
func buildTeamPubKeys() []cryptotypes.PubKey {
	if len(teamPubKeyStrings) == 5 {
		pks := make([]cryptotypes.PubKey, 0, 5)
		for _, s := range teamPubKeyStrings {
			_, bz, err := bech32.DecodeAndConvert(s)
			if err != nil {
				panic(fmt.Sprintf("tokenomics: invalid team multisig bech32 %q: %v", s, err))
			}
			pk, err := legacy.PubKeyFromBytes(bz)
			if err != nil {
				panic(fmt.Sprintf("tokenomics: invalid team multisig pubkey %q: %v", s, err))
			}
			pks = append(pks, pk)
		}
		// 必须与 mcchaind `keys add --multisig` 的默认行为一致：按地址排序。
		// 否则团队用标准 CLI 重建的多签地址会与链上 TeamAddress 不同，导致无法签名。
		sort.Slice(pks, func(i, j int) bool {
			return string(pks[i].Address().Bytes()) < string(pks[j].Address().Bytes())
		})
		return pks
	}

	// 占位：固定种子派生 5 个确定性测试 pubkey（仅测试网可用）。
	return placeholderPubKeys()
}

// placeholderPubKeys 返回 5 个由固定种子派生的 secp256k1 测试公钥。
// 仅当团队未提供真实公钥时使用；主网前必须替换。
func placeholderPubKeys() []cryptotypes.PubKey {
	pubKeys := make([]cryptotypes.PubKey, 0, 5)
	for i := 0; i < 5; i++ {
		seed := make([]byte, 32)
		seed[0] = byte(i + 1)
		hashed := sha256.Sum256(seed)
		pk := secp256k1.PrivKey{Key: hashed[:]}
		pubKeys = append(pubKeys, pk.PubKey())
	}
	return pubKeys
}

// derivedPlaceholder 由固定种子派生单个 secp256k1 占位公钥与地址。
// 确定性、可复现（同种子同地址），仅测试/占位使用；主网前替换为真实多签/运营地址。
func derivedPlaceholder(seed byte) (cryptotypes.PubKey, sdk.AccAddress) {
	seedB := make([]byte, 32)
	seedB[0] = seed
	hashed := sha256.Sum256(seedB)
	pk := secp256k1.PrivKey{Key: hashed[:]}
	pub := pk.PubKey()
	return pub, sdk.AccAddress(pub.Address())
}

// DeviceIncentivePoolAddress 返回设备激励池托管地址（= depin 模块账户）。
func DeviceIncentivePoolAddress() sdk.AccAddress {
	return authtypes.NewModuleAddress(DepinModuleName)
}

// StakingSecurityPoolAddress 返回质押安全池模块账户地址。
func StakingSecurityPoolAddress() sdk.AccAddress {
	return authtypes.NewModuleAddress(StakingSecurityPoolName)
}

// ProtocolTreasuryAddress returns the protocol treasury module account address,
// i.e. the 6th independent on-chain address. Genesis starts at zero.
func ProtocolTreasuryAddress() sdk.AccAddress {
	return authtypes.NewModuleAddress(ProtocolTreasuryPoolName)
}

// BlackHoleAddress 返回全链唯一的黑洞（销毁）地址。
// 由 BlackHolePoolName 经 sha256 确定性派生：任何人都无法构造出对应私钥，
// 打入的代币永久不可支出。创世即存在，无需初始化。
func BlackHoleAddress() sdk.AccAddress {
	return authtypes.NewModuleAddress(BlackHolePoolName)
}

// 注：基金会池与早期开发池在创世时即拨付到可支出地址（见 EarlyDevAddress /
// FoundationOpsAddress / FoundationVestingAddress），不再以模块账户作为资金托管，
// 故此处不提供 FoundationPoolAddress / EarlyDevPoolAddress 模块账户辅助函数。
