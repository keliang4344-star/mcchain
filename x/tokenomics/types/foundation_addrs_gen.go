package types

// 主网创世前覆写文件（与 team_pubkeys_gen.go 同模式）。
//
// 安全修复（A2）：此前 EarlyDev / FoundationOps / FoundationVesting 三个拨付地址
// 在 init() 中无条件由 derivedPlaceholder 确定性派生，其私钥 = sha256(单字节种子)，
// 源码可读即还原，1.8 亿 MC（18% 总量）等于「放在源码里的私钥」控制。
//
// 现提供覆写机制：主网创世前将下列三个公钥替换为真实密钥（bech32 mcpub 前缀），
// init() 会优先使用覆写公钥派生地址，使资金路由到真实多签/冷钱包。
// 默认空字符串 → 仍使用占位地址（私钥写在源码中，仅测试/占位用，绝不接收主网资金）。
var (
	earlyDevPubKeyOverride          = ""
	foundationOpsPubKeyOverride     = ""
	foundationVestingPubKeyOverride = ""
)
