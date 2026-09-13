package keeper

import (
	"encoding/json"
	"testing"
)

// FuzzNormalizeHandoverConfig 覆盖渐进治理移交配置的归一化。
//
// 这是治理最敏感的一段配置：它决定「谁能接管链、以及多久之后能接管」。
// 归一化的职责是把任何读到的（可能来自历史创世、也可能来自旧版本格式的）
// 配置收敛到可用状态，且**绝不产生治理死锁**：
//
//	RequiredSigners ∉ [1,10] → 0 等于取消多签约束；>10 则阈值永不达成、移交锁死
//	TimelockBlocks == 0      → 取消冷静期，观察者来不及反应
//	CurrentGovernor == ""    → 没有任何主体能发起移交，流程整体不可达
//
// 这里断言「归一化后的配置在三条上全部可用」，即输出永远是可达状态。
func FuzzNormalizeHandoverConfig(f *testing.F) {
	f.Add([]byte(`{"enabled":true,"current_governor":"mc1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqhq4d2z","timelock_blocks":43200,"required_signers":3}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"enabled":true,"required_signers":0}`))
	f.Add([]byte(`{"enabled":true,"required_signers":4294967295,"timelock_blocks":0}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var cfg GovernanceHandoverConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			return // 解析失败时上层回落到默认配置
		}
		got := normalizeHandoverConfig(cfg)

		if got.RequiredSigners < MinGovernanceHandoverSigners ||
			got.RequiredSigners > MaxGovernanceHandoverSigners {
			t.Fatalf("归一化后 RequiredSigners=%d 越界 [%d,%d]：移交阈值失效或永不达成",
				got.RequiredSigners, MinGovernanceHandoverSigners, MaxGovernanceHandoverSigners)
		}
		if got.TimelockBlocks == 0 {
			t.Fatalf("归一化后 TimelockBlocks 仍为 0：冷静期消失，移交可被突袭")
		}
		if got.CurrentGovernor == "" {
			t.Fatalf("归一化后 CurrentGovernor 仍为空：没有任何主体能发起移交")
		}
		// 终态标记与时间锁到期高度属流程状态，归一化不得篡改。
		if got.Executed != cfg.Executed && !isUninitialized(cfg) {
			t.Fatalf("归一化改写了 Executed（%v → %v）", cfg.Executed, got.Executed)
		}
		if got.ActivationHeight != cfg.ActivationHeight && !isUninitialized(cfg) {
			t.Fatalf("归一化改写了 ActivationHeight（%d → %d）",
				cfg.ActivationHeight, got.ActivationHeight)
		}
	})
}

// isUninitialized 复述「历史遗留未初始化状态」的判定：该组合会被整体替换为
// 默认配置，因此所有字段都允许被改写。
func isUninitialized(cfg GovernanceHandoverConfig) bool {
	return !cfg.Enabled && cfg.CurrentGovernor == "" && !cfg.Executed && cfg.NewGovernor == ""
}
