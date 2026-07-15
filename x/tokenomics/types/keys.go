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

	// CommunityPoolName 社区池模块账户名（Q5：独立社区账户）。
	CommunityPoolName = "community"

	// EcosystemPoolName 生态池模块账户名。
	EcosystemPoolName = "ecosystem"

	// TeamPoolName 团队池分配名（地址为 3-of-5 多签 vesting 账户）。
	TeamPoolName = "team"

	// DefaultDenom 链上主币 denom。
	DefaultDenom = "umc"

	// TotalSupplyCap 总量上限（umc），= 1e9 MC。
	// 链上常量 + Genesis 校验双保险；不进 params subspace、不治理可改（Q8）。
	TotalSupplyCap = uint64(1e15)

	// TeamMultisigThreshold 团队多签阈值（3-of-5，Q6）。
	TeamMultisigThreshold = 3

	// 分配占比（基点，10000 = 100%）：团队 15% / 社区 35% / 生态 50%（Q2）。
	TeamPercentBps      = uint32(1500)
	CommunityPercentBps = uint32(3500)
	EcosystemPercentBps = uint32(5000)

	// DepinInitialPoolSlice 生态池转给 depin 的切片（InitialPool，Q4/Q7）。
	// 必须等于 x/depin/types.DefaultInitialPool（1e14 umc = 1e8 MC）。
	DepinInitialPoolSlice = uint64(1e14)

	// DepinModuleName 生态切片拨付的目标模块名（C2：以常量替代字符串，编译期可查）。
	// tokenomics → depin 的跨模块转账统一引用此常量，避免隐式字符串耦合。
	DepinModuleName = "depin"
)

// ---------------------------------------------------------------------------
// 团队 3-of-5 多签公钥配置（T1）
// ---------------------------------------------------------------------------
// teamPubKeyStrings（5 个真实 secp256k1 公钥，bech32 mcpub 前缀）由
// scripts/gen_team_keys 自动生成到同包文件 team_pubkeys_gen.go，本文件不再硬编码，
// 以避免「手写复制」导致 keys.go 与 team_multisig_vault.txt / team_keys_gen.json
// 三者漂移。对应私钥由各自 bip39 助记词恢复（见本地 team_multisig_vault.txt，
// 仅你本机保管，绝不入库）。必须恰好 5 个有效 bech32，否则回退占位公钥。
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// 运行时常量（init 派生）
// ---------------------------------------------------------------------------

var (
	// TeamMultisigPubKey 3-of-5 多签公钥（真实或占位）。
	TeamMultisigPubKey cryptotypes.PubKey
	// TeamAddress 团队多签 vesting 账户地址（bech32 前缀 mc）。
	TeamAddress sdk.AccAddress
)

func init() {
	pubKeys := buildTeamPubKeys()
	TeamMultisigPubKey = multisig.NewLegacyAminoPubKey(int(TeamMultisigThreshold), pubKeys)
	TeamAddress = sdk.AccAddress(TeamMultisigPubKey.Address())
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

// CommunityPoolAddress 返回社区池模块账户地址。
func CommunityPoolAddress() sdk.AccAddress {
	return authtypes.NewModuleAddress(CommunityPoolName)
}

// EcosystemPoolAddress 返回生态池模块账户地址。
func EcosystemPoolAddress() sdk.AccAddress {
	return authtypes.NewModuleAddress(EcosystemPoolName)
}
