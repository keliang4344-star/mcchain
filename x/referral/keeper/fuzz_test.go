package keeper

import (
	"encoding/json"
	"testing"
)

// FuzzReleaseScheduleNormalize 覆盖推荐返佣释放节奏配置的归一化与解析。
//
// ReleaseSchedule 是治理可调的 JSON 配置，直接决定「生态预算按什么节奏释放」。
// 它的三个字段之间存在硬约束，任何一个被治理提案写成退化值都可能让配速失效：
//
//	TotalDays == 0           → 配速分母为零（除零 / 无上限释放）
//	Phase1MultiplierBps < 100 → 冷启动倍数低于 1 倍，会把上限压到治理参数之下
//	Phase1Days > TotalDays    → 冷启动期越过整个释放窗口
//
// 这里断言的是**归一化后的完备性**：无论输入如何，输出都必须同时满足上述三条。
func FuzzReleaseScheduleNormalize(f *testing.F) {
	f.Add([]byte(`{"start_time_unix":1700000000,"total_days":4015,"phase1_days":365,"phase1_multiplier_bps":200}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"total_days":0,"phase1_days":0,"phase1_multiplier_bps":0}`))
	f.Add([]byte(`{"total_days":4294967295,"phase1_days":4294967295,"phase1_multiplier_bps":4294967295}`))
	f.Add([]byte(`{"total_days":1,"phase1_days":4294967295,"phase1_multiplier_bps":99}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var s ReleaseSchedule
		if err := json.Unmarshal(data, &s); err != nil {
			return // 解析失败时上层返回确定性默认值，不进入归一化
		}
		got := s.normalize()

		if got.TotalDays == 0 {
			t.Fatalf("归一化后 TotalDays 仍为 0：配速分母为零，释放节奏失去上界")
		}
		if got.Phase1MultiplierBps < 100 {
			t.Fatalf("归一化后 Phase1MultiplierBps=%d < 100：会把日上限压到治理参数之下",
				got.Phase1MultiplierBps)
		}
		if got.Phase1Days > got.TotalDays {
			t.Fatalf("归一化后 Phase1Days=%d > TotalDays=%d：冷启动期越过释放窗口",
				got.Phase1Days, got.TotalDays)
		}
		// StartTimeUnix 是「是否已初始化」的标记，归一化不得擅自改写它，
		// 否则重启后释放窗口会被静默重置。
		if got.StartTimeUnix != s.StartTimeUnix {
			t.Fatalf("归一化改写了 StartTimeUnix（%d → %d）", s.StartTimeUnix, got.StartTimeUnix)
		}
	})
}
