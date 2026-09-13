package keeper

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// FuzzValidateSettlementAuthority 覆盖 DEX 结算运营地址的校验闸门。
//
// 这个地址一旦配错，DEX 结算通道就是死锁的：
//   - 配成模块账户 → 模块账户没有私钥，永远签不出授权交易；
//   - 配成非法 bech32 → 后续所有鉴权必然失败。
//
// 因此断言的是**接受即安全**：凡是通过校验的地址，必须①能被解析成账户地址，
// ②不是已知的模块账户。任何「接受了一个模块账户」的输入都是必须修的漏洞。
func FuzzValidateSettlementAuthority(f *testing.F) {
	f.Add("")                                                // 空 → 必须拒绝
	f.Add(authtypes.NewModuleAddress("gov").String())        // gov 模块账户 → 必须拒绝
	f.Add(authtypes.NewModuleAddress("tokenomics").String()) // 其它模块账户 → 必须拒绝
	f.Add("mc1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqhq4d2z")       // 普通地址 → 应接受

	f.Fuzz(func(t *testing.T, addr string) {
		if err := ValidateSettlementAuthority(addr); err != nil {
			return // 拒绝是预期结果
		}
		if addr == "" {
			t.Fatalf("接受了空地址：结算通道将不可用")
		}
		if _, err := sdk.AccAddressFromBech32(addr); err != nil {
			t.Fatalf("接受了非法 bech32 地址 %q: %v", addr, err)
		}
		for _, m := range moduleAccountNames {
			if authtypes.NewModuleAddress(m).String() == addr {
				t.Fatalf("接受了模块账户 %q 作为结算运营地址：该账户无私钥，结算必然死锁", m)
			}
		}
	})
}
