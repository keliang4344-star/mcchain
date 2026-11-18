package chainprofile

import (
	"strings"
	"testing"
)

func TestLookupAndResolve(t *testing.T) {
	cases := []struct {
		name    string
		chainID string
		want    Profile
		wantOK  bool
	}{
		{"production 精确名", "mcchain-mainnet-1", Production, true},
		{"testnet 精确名", "mcchain-testnet-1", Testnet, true},
		{"devnet 精确名", "mcchain-dev-1", Development, true},
		{"混合字符串未知", "mcchain-mainnet-dev", "", false},
		{"大写未知", "MCCHAIN-MAINNET-1", "", false},
		{"前后空格未知", " mcchain-mainnet-1 ", "", false},
		{"空值未知", "", "", false},
		{"未登记新链未知", "mcchain-qa-2", "", false},
	}
	for _, tc := range cases {
		got, ok := Lookup(tc.chainID)
		if ok != tc.wantOK || (ok && got != tc.want) {
			t.Errorf("%s: Lookup(%q) = (%q,%v), want (%q,%v)", tc.name, tc.chainID, got, ok, tc.want, tc.wantOK)
		}
		_, err := Resolve(tc.chainID)
		if tc.wantOK && err != nil {
			t.Errorf("%s: Resolve(%q) unexpected error: %v", tc.name, tc.chainID, err)
		}
		if !tc.wantOK && err == nil {
			t.Errorf("%s: Resolve(%q) should return error", tc.name, tc.chainID)
		}
	}
}

// IsProduction 必须 fail-closed：只有精确命中 test/development 才放行。
func TestIsProductionFailClosed(t *testing.T) {
	cases := []struct {
		chainID string
		want    bool
	}{
		{"mcchain-mainnet-1", true},
		{"MCCHAIN-MAINNET-1", true},   // 大写不是精确匹配 → 生产
		{"", true},                    // 空 → 生产
		{"mcchain-mainnet-dev", true}, // 混合字符串 → 生产
		{"mcchain-testnet-1", false},
		{"mcchain-dev-1", false},
		{"mcchain-local-1", false},
		{"mcchain-sim-1", false},
	}
	for _, tc := range cases {
		if got := IsProduction(tc.chainID); got != tc.want {
			t.Errorf("IsProduction(%q) = %v, want %v", tc.chainID, got, tc.want)
		}
	}
}

func TestValidateStartup(t *testing.T) {
	t.Run("已知非生产链通过", func(t *testing.T) {
		t.Setenv("MC_PROFILE", "")
		if err := ValidateStartup("mcchain-dev-1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("未知链默认拒绝", func(t *testing.T) {
		t.Setenv("MC_PROFILE", "")
		t.Setenv("MC_ALLOW_UNKNOWN_CHAIN_ID", "")
		err := ValidateStartup("chain-id-0-0")
		if err == nil {
			t.Fatal("unknown chain-id must be rejected")
		}
		if !strings.Contains(err.Error(), "unknown chain-id") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("未知链显式豁免", func(t *testing.T) {
		t.Setenv("MC_ALLOW_UNKNOWN_CHAIN_ID", "1")
		t.Setenv("MC_ORACLE_PUBKEY", "test-pubkey")
		if err := ValidateStartup("chain-id-0-0"); err != nil {
			t.Fatalf("explicit opt-in should pass: %v", err)
		}
	})

	t.Run("profile 与 chain-id 不一致拒绝", func(t *testing.T) {
		t.Setenv("MC_ALLOW_UNKNOWN_CHAIN_ID", "")
		t.Setenv("MC_PROFILE", "test")
		if err := ValidateStartup("mcchain-dev-1"); err == nil {
			t.Fatal("profile mismatch must be rejected")
		}
	})

	t.Run("空值/空格/大写拒绝", func(t *testing.T) {
		t.Setenv("MC_PROFILE", "")
		for _, id := range []string{"", " mcchain-dev-1", "mcchain-dev-1 ", "MCCHAIN-DEV-1"} {
			if err := ValidateStartup(id); err == nil {
				t.Errorf("ValidateStartup(%q) must fail", id)
			}
		}
	})

	t.Run("生产链禁止软预言机", func(t *testing.T) {
		t.Setenv("MC_PROFILE", "")
		t.Setenv("MC_ORACLE_PUBKEY", "test-pubkey")
		t.Setenv("MC_ORACLE_ALLOW_SOFT", "1")
		err := ValidateStartup("mcchain-mainnet-1")
		if err == nil || !strings.Contains(err.Error(), "MC_ORACLE_ALLOW_SOFT") {
			t.Fatalf("soft oracle must be rejected on production, got: %v", err)
		}
	})

	t.Run("生产链要求预言机公钥", func(t *testing.T) {
		t.Setenv("MC_ORACLE_ALLOW_SOFT", "")
		t.Setenv("MC_ORACLE_PUBKEY", "")
		err := ValidateStartup("mcchain-mainnet-1")
		if err == nil || !strings.Contains(err.Error(), "MC_ORACLE_PUBKEY") {
			t.Fatalf("missing oracle pubkey must be rejected on production, got: %v", err)
		}
	})

	t.Run("生产链禁止开发密钥后端", func(t *testing.T) {
		t.Setenv("MC_ORACLE_PUBKEY", "test-pubkey")
		t.Setenv("MC_KEYRING_BACKEND", "test")
		defer t.Setenv("MC_KEYRING_BACKEND", "")
		err := ValidateStartup("mcchain-mainnet-1")
		if err == nil || !strings.Contains(err.Error(), "development key backend") {
			t.Fatalf("dev key backend must be rejected on production, got: %v", err)
		}
	})
}
