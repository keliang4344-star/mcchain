package keeper

import (
	"encoding/json"
	"testing"

	"mcchain/x/phonenode/types"
)

// FuzzValidateAttestationInputs 覆盖 attestation 输入闸门。
//
// nonce 与 device_id_hash 会**原样拼进 KVStore 键**，且来自不受信的客户端消息，
// 是链上最主要的外部输入攻击面之一。这里断言的不是「某个具体输入被拒绝」，
// 而是**闸门的完备性**：凡是被接受的输入，必须同时满足全部约束——
// 任何「接受却不满足约束」的输入都意味着一处可被利用的漏网。
func FuzzValidateAttestationInputs(f *testing.F) {
	f.Add("cm9vdA==", "abcdefgh", "0123456789abcdef0123456789abcdef")
	f.Add("", "", "")
	f.Add("root", "short", "not-hex")
	f.Add("root", "abcdefgh", "ABCDEF0123456789ABCDEF0123456789")
	f.Add("root", "abcdefgh", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")

	f.Fuzz(func(t *testing.T, rootHash, nonce, deviceIDHash string) {
		if err := validateAttestationInputs(rootHash, nonce, deviceIDHash); err != nil {
			return // 拒绝是预期结果之一
		}
		// 一旦接受，必须同时满足全部约束。
		if len(rootHash) > types.MaxRootHashLength {
			t.Fatalf("接受超长 root_hash: len=%d > %d", len(rootHash), types.MaxRootHashLength)
		}
		if !allInRange(rootHash, 0x21, 0x7E) {
			t.Fatalf("接受含非可打印字节的 root_hash: %q", rootHash)
		}
		if len(nonce) < types.MinNonceLength || len(nonce) > types.MaxNonceLength {
			t.Fatalf("接受越界 nonce: len=%d（应在 %d..%d）",
				len(nonce), types.MinNonceLength, types.MaxNonceLength)
		}
		if !allInRange(nonce, 0x21, 0x7E) {
			t.Fatalf("接受含非可打印字节的 nonce: %q", nonce)
		}
		if len(deviceIDHash) < 32 || len(deviceIDHash) > types.MaxDeviceIDHashLength ||
			len(deviceIDHash)%2 != 0 {
			t.Fatalf("接受越界 device_id_hash: len=%d", len(deviceIDHash))
		}
		for i := 0; i < len(deviceIDHash); i++ {
			c := deviceIDHash[i]
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Fatalf("接受非小写 hex 的 device_id_hash: %q", deviceIDHash)
			}
		}
	})
}

// FuzzNormalizeAllowanceConfig 覆盖节点资本津贴配置的零值归一化：
// 任何输入经归一化后 PerDay 都不应为 0 —— 0 等于「静默关闭津贴」，
// 属误配而非治理意图，必须被归一化挡住。
func FuzzNormalizeAllowanceConfig(f *testing.F) {
	f.Add([]byte(`{"enabled":true,"per_day_umc":30000000}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"per_day_umc":0}`))
	f.Add([]byte(`{"enabled":false,"per_day_umc":18446744073709551615}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var cfg NodeAllowanceConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			return
		}
		got := normalizeAllowanceConfig(cfg)
		if got.PerDay == 0 {
			t.Fatalf("归一化后 PerDay 仍为 0，会静默关闭津贴")
		}
		// Enabled 是治理意图，归一化只允许补零值，不得擅自改写开关。
		if got.Enabled != cfg.Enabled {
			t.Fatalf("归一化改写了 Enabled（%v → %v）", cfg.Enabled, got.Enabled)
		}
	})
}

// allInRange 报告 s 的每个字节是否都落在 [lo, hi]。
func allInRange(s string, lo, hi byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < lo || s[i] > hi {
			return false
		}
	}
	return true
}
