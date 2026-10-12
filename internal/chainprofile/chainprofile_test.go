package chainprofile

import "testing"

func TestIsProduction(t *testing.T) {
	cases := []struct {
		name    string
		chainID string
		want    bool
	}{
		{"mainnet 精确名", "mcchain-mainnet-1", true},
		{"mainnet 别名", "mcchain-mainnet", true},
		{"mainnet 大小写", "MCCHAIN-MAINNET-1", true},
		{"空 chain-id fail-closed", "", true},
		{"未知链名 fail-closed", "mcchain-qa-2", true},
		{"testnet 标记", "mcchain-testnet-1", false},
		{"devnet", "mcchain-dev-1", false},
		{"local", "mcchain-local", false},
		{"sim", "mcchain-sim", false},
	}
	for _, tc := range cases {
		if got := IsProduction(tc.chainID); got != tc.want {
			t.Errorf("%s: IsProduction(%q) = %v, want %v", tc.name, tc.chainID, got, tc.want)
		}
	}
}
