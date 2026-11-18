// Package chainprofile 提供 chain-id → 部署档位（production/test/development）的显式判定。
//
// 为什么必须显式枚举（而不是子字符串推断）：
// 早期实现是「黑名单式」——只要 chain-id 不含 test/dev/local/sim 就按生产处理。
// 两个方向都会出错：
//   - 任意命名的新链（如 mcchain-qa-2）被误判为生产，触发生产闸门；
//   - 生产链若命名不规范（如 mcchain-mainnet-dev），会被子串命中而误放行，
//     等于让 SoftOracle、开发密钥后端等危险配置在「看起来像主网」的链上生效。
//
// 本包的设计（安全侧优先）：
//  1. 三个显式档位：production / test / development，各自维护**精确** chain-id 集合；
//  2. 运行期闸门 IsProduction 采用 fail-closed：只有精确命中非生产集合才放行，
//     其余（未知、混合字符串、空值）一律按生产拦截；
//  3. 启动期校验 ValidateStartup 采用 fail-closed：未知 chain-id 直接拒绝启动，
//     除非运维显式声明 MC_ALLOW_UNKNOWN_CHAIN_ID=1（仅限本地仿真/临时链）；
//  4. 生产档位额外禁止：MC_ORACLE_ALLOW_SOFT=1、缺失 MC_ORACLE_PUBKEY、
//     使用 test/memory 密钥后端。
//
// 新增主网/测试网时，必须先在本文件登记精确 chain-id 再上线——这是刻意的
// 代码变更 + 评审流程，避免任意命名的新链悄悄继承生产/非生产语义。
package chainprofile

import (
	"fmt"
	"os"
	"strings"
)

// Profile 是显式部署档位。
type Profile string

const (
	// Production 主网档位：启用全部生产闸门。
	Production Profile = "production"
	// Testnet 公共测试网档位：允许软预言机，但资金/共识逻辑与主网一致。
	Testnet Profile = "test"
	// Development 本地开发/CI/devnet 档位。
	Development Profile = "development"
)

// AllowedChainIDs 是各档位允许的精确 chain-id 集合（唯一事实源）。
//
// 必须精确匹配（区分大小写、不允许前后空格）；任何未登记的值都视为未知。
var AllowedChainIDs = map[Profile]map[string]struct{}{
	Production: {
		"mcchain-mainnet-1": {},
	},
	Testnet: {
		"mcchain-testnet-1": {},
	},
	Development: {
		"mcchain-dev-1":      {},
		"mcchain-dev-solo-1": {},
		"mcchain-local-1":    {},
		"mcchain-sim-1":      {},
	},
}

// Lookup 精确查找 chain-id 所属档位。
//
// 入参保持原样比较，不做 ToLower/TrimSpace —— 大小写或空格差异属于「未知组合」，
// 必须被上层拒绝，而不是被静默规范化成某个已知链。
func Lookup(chainID string) (Profile, bool) {
	for p, ids := range AllowedChainIDs {
		if _, ok := ids[chainID]; ok {
			return p, true
		}
	}
	return "", false
}

// Resolve 严格解析 chain-id 所属档位，未知值返回错误。
func Resolve(chainID string) (Profile, error) {
	if chainID == "" {
		return "", fmt.Errorf("chainprofile: empty chain-id is not a registered profile")
	}
	p, ok := Lookup(chainID)
	if !ok {
		return "", fmt.Errorf("chainprofile: unknown chain-id %q (registered: %s)", chainID, RegisteredIDs())
	}
	return p, nil
}

// IsProduction 判定 chain-id 是否按生产档位处理（运行期闸门）。
//
// fail-closed：只有精确命中 test/development 集合才返回 false；
// 空值、未知值、任何混合字符串一律返回 true（按生产拦截）。
// 该函数被交易校验路径调用，绝不能 panic，也绝不能因命名巧合放行。
func IsProduction(chainID string) bool {
	p, ok := Lookup(chainID)
	if !ok {
		return true
	}
	return p == Production
}

// RegisteredIDs 返回全部已登记的 chain-id（用于错误提示与文档生成）。
func RegisteredIDs() string {
	var out []string
	for _, p := range []Profile{Production, Testnet, Development} {
		for id := range AllowedChainIDs[p] {
			out = append(out, fmt.Sprintf("%s(%s)", id, p))
		}
	}
	return strings.Join(out, ", ")
}

// DeclaredProfile 读取运维显式声明的档位（MC_PROFILE），空表示未声明。
func DeclaredProfile() (Profile, error) {
	raw := strings.TrimSpace(os.Getenv("MC_PROFILE"))
	if raw == "" {
		return "", nil
	}
	switch Profile(strings.ToLower(raw)) {
	case Production:
		return Production, nil
	case Testnet:
		return Testnet, nil
	case Development:
		return Development, nil
	default:
		return "", fmt.Errorf("MC_PROFILE must be one of production|test|development, got %q", raw)
	}
}

// ValidateStartup 校验启动期 chain-id 与档位配置的一致性。
//
// 校验项（任何一条失败都拒绝启动）：
//  1. chain-id 非空、无前后空格、全小写（防止 "mcchain-mainnet-1 " / 大写绕过精确匹配）；
//  2. chain-id 已登记；未登记时必须显式设置 MC_ALLOW_UNKNOWN_CHAIN_ID=1
//     （仅用于本地仿真/临时链，默认拒绝）；
//  3. 若声明了 MC_PROFILE，必须与 chain-id 所属档位完全一致；
//  4. 生产档位禁止 MC_ORACLE_ALLOW_SOFT=1、要求 MC_ORACLE_PUBKEY 非空、
//     禁止 test/memory 密钥后端。
func ValidateStartup(chainID string) error {
	if chainID == "" {
		return fmt.Errorf("chainprofile: chain-id is empty; a registered chain-id is required")
	}
	if strings.TrimSpace(chainID) != chainID {
		return fmt.Errorf("chainprofile: chain-id %q has leading/trailing whitespace", chainID)
	}
	if chainID != strings.ToLower(chainID) {
		return fmt.Errorf("chainprofile: chain-id %q must be lowercase", chainID)
	}

	profile, ok := Lookup(chainID)
	if !ok {
		if os.Getenv("MC_ALLOW_UNKNOWN_CHAIN_ID") == "1" {
			// 显式豁免：按生产档位处理（fail-closed），仅跳过「必须已登记」这一条。
			profile = Production
		} else {
			return fmt.Errorf(
				"chainprofile: unknown chain-id %q; register it in internal/chainprofile or "+
					"set MC_ALLOW_UNKNOWN_CHAIN_ID=1 for throwaway local chains (registered: %s)",
				chainID, RegisteredIDs())
		}
	}

	declared, err := DeclaredProfile()
	if err != nil {
		return err
	}
	if declared != "" && declared != profile {
		return fmt.Errorf(
			"chainprofile: MC_PROFILE=%s does not match chain-id %q (profile %s)",
			declared, chainID, profile)
	}

	if profile == Production {
		if os.Getenv("MC_ORACLE_ALLOW_SOFT") == "1" {
			return fmt.Errorf(
				"chainprofile: MC_ORACLE_ALLOW_SOFT=1 is forbidden on production chain %q", chainID)
		}
		if strings.TrimSpace(os.Getenv("MC_ORACLE_PUBKEY")) == "" {
			return fmt.Errorf(
				"chainprofile: MC_ORACLE_PUBKEY is required on production chain %q", chainID)
		}
		for _, key := range []string{"MC_KEYRING_BACKEND", "MC_VALIDATOR_KEYRING_BACKEND"} {
			backend := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
			if backend == "test" || backend == "memory" {
				return fmt.Errorf(
					"chainprofile: %s=%q is a development key backend and is forbidden on production chain %q",
					key, backend, chainID)
			}
		}
	}
	return nil
}
