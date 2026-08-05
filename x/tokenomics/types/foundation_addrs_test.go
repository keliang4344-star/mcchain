package types

import (
	"crypto/sha256"
	"testing"

	"github.com/cosmos/cosmos-sdk/codec/legacy"
	secp256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
)

// bech32PubKey 按 resolveFoundationKey 期望的格式编码一个公钥：
// amino 二进制 → bech32（hrp 不参与解析，与线上 `mcpub` 前缀保持一致仅为可读性）。
func bech32PubKey(t *testing.T, pk cryptotypes.PubKey) string {
	t.Helper()
	bz, err := legacy.Cdc.Marshal(pk)
	if err != nil {
		t.Fatalf("amino marshal pubkey: %v", err)
	}
	s, err := bech32.ConvertAndEncode("mcpub", bz)
	if err != nil {
		t.Fatalf("bech32 encode pubkey: %v", err)
	}
	return s
}

// keyFromSeed 由任意种子确定性造一把测试密钥（不同于 derivedPlaceholder 的单字节种子空间）。
func keyFromSeed(seed string) cryptotypes.PubKey {
	h := sha256.Sum256([]byte(seed))
	priv := secp256k1.PrivKey{Key: h[:]}
	return priv.PubKey()
}

// 覆写为空时，回退到源码可推导的占位地址；三个种子必须互不相同。
func TestResolveFoundationKeyFallsBackToPlaceholder(t *testing.T) {
	seeds := []byte{0x21, 0x22, 0x23}
	seen := map[string]byte{}

	for _, seed := range seeds {
		gotPub, gotAddr := resolveFoundationKey("", seed)
		wantPub, wantAddr := derivedPlaceholder(seed)

		if !gotPub.Equals(wantPub) {
			t.Fatalf("seed %#x: pubkey mismatch with derivedPlaceholder", seed)
		}
		if !gotAddr.Equals(wantAddr) {
			t.Fatalf("seed %#x: address %s != placeholder %s", seed, gotAddr, wantAddr)
		}
		if prev, dup := seen[gotAddr.String()]; dup {
			t.Fatalf("seed %#x collides with seed %#x at address %s", seed, prev, gotAddr)
		}
		seen[gotAddr.String()] = seed
	}
}

// 提供合法覆写公钥时，地址必须来自该公钥，而不是占位派生。
func TestResolveFoundationKeyHonoursOverride(t *testing.T) {
	cases := []struct {
		name string
		seed byte
	}{
		{"early_dev", 0x21},
		{"foundation_ops", 0x22},
		{"foundation_vesting", 0x23},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			real := keyFromSeed("real-" + tc.name)
			override := bech32PubKey(t, real)

			gotPub, gotAddr := resolveFoundationKey(override, tc.seed)

			if !gotPub.Equals(real) {
				t.Fatalf("override pubkey not round-tripped")
			}
			if want := sdk.AccAddress(real.Address()); !gotAddr.Equals(want) {
				t.Fatalf("address %s != %s derived from override pubkey", gotAddr, want)
			}
			if _, placeholder := derivedPlaceholder(tc.seed); gotAddr.Equals(placeholder) {
				t.Fatalf("override ignored: address still equals source-derivable placeholder %s", placeholder)
			}
		})
	}
}

// 非法覆写必须 panic —— 宁可创世起不来，也不能把资金路由到解析失败后的零值地址。
func TestResolveFoundationKeyRejectsMalformedOverride(t *testing.T) {
	valid := bech32PubKey(t, keyFromSeed("valid"))

	cases := []struct {
		name     string
		override string
	}{
		{"not_bech32", "definitely-not-bech32"},
		{"bech32_with_junk_payload", func() string {
			s, err := bech32.ConvertAndEncode("mcpub", []byte{0x01, 0x02, 0x03})
			if err != nil {
				t.Fatalf("encode junk: %v", err)
			}
			return s
		}()},
		{"truncated_valid_key", valid[:len(valid)-6]},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("expected panic for override %q", tc.override)
				}
			}()
			_, _ = resolveFoundationKey(tc.override, 0x21)
		})
	}
}

// 包级地址必须与 init() 时的覆写状态一致（防止有人改了覆写却忘了重建地址）。
func TestPackageAddressesTrackOverrides(t *testing.T) {
	cases := []struct {
		name     string
		override string
		seed     byte
		gotAddr  sdk.AccAddress
		gotPub   cryptotypes.PubKey
	}{
		{"EarlyDev", earlyDevPubKeyOverride, 0x21, EarlyDevAddress, EarlyDevPubKey},
		{"FoundationOps", foundationOpsPubKeyOverride, 0x22, FoundationOpsAddress, FoundationOpsPubKey},
		{"FoundationVesting", foundationVestingPubKeyOverride, 0x23, FoundationVestingAddress, FoundationVestingPubKey},
	}

	for _, tc := range cases {
		wantPub, wantAddr := resolveFoundationKey(tc.override, tc.seed)
		if !tc.gotPub.Equals(wantPub) {
			t.Fatalf("%s: package pubkey does not match resolver output", tc.name)
		}
		if !tc.gotAddr.Equals(wantAddr) {
			t.Fatalf("%s: package address %s != resolver output %s", tc.name, tc.gotAddr, wantAddr)
		}
		if tc.gotAddr.Empty() {
			t.Fatalf("%s: address is empty", tc.name)
		}
	}
}

// 三个拨付地址彼此不同，也不与团队多签地址相同。
func TestPayoutAddressesAreDistinct(t *testing.T) {
	addrs := map[string]sdk.AccAddress{
		"EarlyDev":          EarlyDevAddress,
		"FoundationOps":     FoundationOpsAddress,
		"FoundationVesting": FoundationVestingAddress,
		"Team":              TeamAddress,
	}

	seen := map[string]string{}
	for name, addr := range addrs {
		key := addr.String()
		if prev, dup := seen[key]; dup {
			t.Fatalf("%s and %s share address %s", name, prev, key)
		}
		seen[key] = name
	}
}

// FoundationOverridesConfigured 必须如实反映三个覆写是否齐备。
func TestFoundationOverridesConfigured(t *testing.T) {
	origEarly, origOps, origVest := earlyDevPubKeyOverride, foundationOpsPubKeyOverride, foundationVestingPubKeyOverride
	t.Cleanup(func() {
		earlyDevPubKeyOverride, foundationOpsPubKeyOverride, foundationVestingPubKeyOverride = origEarly, origOps, origVest
	})

	a := bech32PubKey(t, keyFromSeed("a"))
	b := bech32PubKey(t, keyFromSeed("b"))
	c := bech32PubKey(t, keyFromSeed("c"))

	cases := []struct {
		name             string
		early, ops, vest string
		wantConfigured   bool
	}{
		{"all_empty", "", "", "", false},
		{"early_only", a, "", "", false},
		{"missing_vesting", a, b, "", false},
		{"missing_ops", a, "", c, false},
		{"all_present", a, b, c, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			earlyDevPubKeyOverride, foundationOpsPubKeyOverride, foundationVestingPubKeyOverride = tc.early, tc.ops, tc.vest
			if got := FoundationOverridesConfigured(); got != tc.wantConfigured {
				t.Fatalf("FoundationOverridesConfigured() = %v, want %v", got, tc.wantConfigured)
			}
		})
	}
}

// 团队多签当前由 5 个真实公钥构成；一旦有人清空或截断该列表，本测试立刻失败。
func TestTeamPubKeysConfigured(t *testing.T) {
	if !TeamPubKeysConfigured() {
		t.Fatalf("team multisig fell back to source-derivable placeholder keys")
	}
	if len(teamPubKeyStrings) != 5 {
		t.Fatalf("team pubkey count = %d, want 5 (3-of-5 multisig)", len(teamPubKeyStrings))
	}

	// 真实公钥派生出的多签地址，不得等于占位公钥派生出的多签地址。
	placeholder := placeholderPubKeys()
	if len(placeholder) != len(teamPubKeyStrings) {
		t.Fatalf("placeholder set size %d != real set size %d", len(placeholder), len(teamPubKeyStrings))
	}
	for _, pk := range placeholder {
		if sdk.AccAddress(pk.Address()).Equals(TeamAddress) {
			t.Fatalf("team address collides with a placeholder key address")
		}
	}
}
