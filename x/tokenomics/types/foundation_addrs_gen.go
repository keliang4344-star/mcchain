package types

import "os"

// 主网创世前覆写文件（与 team_pubkeys_gen.go 同模式）。
//
// 安全修复（A2）：此前 EarlyDev / FoundationOps / FoundationVesting 三个拨付地址
// 在 init() 中无条件由 derivedPlaceholder 确定性派生，其私钥 = sha256(单字节种子)，
// 源码可读即还原，1.8 亿 MC（18% 总量）等于「放在源码里的私钥」控制。
//
// 现提供覆写机制：主网创世前将下列三个公钥替换为真实密钥（bech32 mcpub 前缀），
// init() 会优先使用覆写公钥派生地址，使资金路由到真实多签/冷钱包。
// 默认空字符串 → 仍使用占位地址（私钥写在源码中，仅测试/占位用，绝不接收主网资金）。
//
// 生产加固（KEY-2）：除直接编辑本文件外，也可通过环境变量注入真实公钥，避免在源码/
// 镜像中硬编码任何密钥材料：
//   MC_FOUNDATION_EARLY_DEV_PUBKEY / MC_FOUNDATION_OPS_PUBKEY / MC_FOUNDATION_VESTING_PUBKEY
// 任一变量非空时优先生效（覆盖本文件默认值；本文件默认留空=占位派生）。
var (
	earlyDevPubKeyOverride          = ""
	foundationOpsPubKeyOverride     = ""
	foundationVestingPubKeyOverride = ""
)

func init() {
	if v := os.Getenv("MC_FOUNDATION_EARLY_DEV_PUBKEY"); v != "" {
		earlyDevPubKeyOverride = v
	}
	if v := os.Getenv("MC_FOUNDATION_OPS_PUBKEY"); v != "" {
		foundationOpsPubKeyOverride = v
	}
	if v := os.Getenv("MC_FOUNDATION_VESTING_PUBKEY"); v != "" {
		foundationVestingPubKeyOverride = v
	}
}
