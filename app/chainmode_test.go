package app

import (
	"testing"

	tokenomicsmoduletypes "mcchain/x/tokenomics/types"
)

// TestProductionGenesisRequired ——
// 「是否按生产口径硬校验创世」的判据必须与 tokenomics 的创世密钥闸门同口径：
// chain-id 嗅探（黑名单式，默认 fail-closed）**或**显式声明 MC_REQUIRE_REAL_GENESIS_KEYS=1。
//
// 为什么值得单独钉住：
//   - 如果只靠 chain-id，把链改名叫 mcchain-dev-1 就能让「总量超顶即拒绝启动」
//     退化成一行日志，而总量恒定是唯一不可回滚的不变量；
//   - isProductionChainID 是黑名单（只在含 test/dev/local/sim 时放行），
//     所以默认方向是 fail-closed：起个随意名字的链也按生产拦。
func TestProductionGenesisRequired(t *testing.T) {
	const env = tokenomicsmoduletypes.RequireRealGenesisKeysEnv

	cases := []struct {
		name    string
		chainID string
		envVal  string
		want    bool
	}{
		{"mainnet 链名 → 生产口径", "mcchain-mainnet-1", "", true},
		{"无名链 → 默认按生产口径（黑名单式 fail-closed）", "mc-1", "", true},
		{"dev 链名 → 放宽（允许 add-genesis-account 的本地链）", "mcchain-dev-1", "", false},
		{"local 链名 → 放宽", "mcchain-local", "", false},
		{"sim 链名 → 放宽", "mcchain-sim", "", false},
		{"test 链名 → 放宽", "mcchain-test-1", "", false},
		{"dev 链名 + 显式生产开关 → 收紧（生产误命名场景）", "mcchain-dev-1", "1", true},
		{"local 链名 + 显式生产开关 → 收紧", "mcchain-local", "1", true},
		{"mainnet 链名 + 开关非 1（如 0/true）→ 仍按生产", "mcchain-mainnet-1", "0", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(env, tc.envVal)
			if got := productionGenesisRequired(tc.chainID); got != tc.want {
				t.Fatalf("productionGenesisRequired(%q) with %s=%q = %v, want %v",
					tc.chainID, env, tc.envVal, got, tc.want)
			}
		})
	}
}
