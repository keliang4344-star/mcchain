package keeper

import (
	"testing"

	"mcchain/x/edgeai/types"
)

func TestDeterminePayout(t *testing.T) {
	params := types.DefaultParams() // MaxTaskReward = "1000000000"

	cases := []struct {
		name     string
		reward   uint64
		cap      string
		expected uint64
	}{
		{"reward under cap", 500_000_000, "1000000000", 500_000_000},
		{"reward equals cap", 1_000_000_000, "1000000000", 1_000_000_000},
		{"reward over cap", 2_000_000_000, "1000000000", 1_000_000_000},
		{"zero reward", 0, "1000000000", 0},
		{"invalid cap falls back to reward", 123, "not-a-number", 123},
		{"zero cap falls back to reward", 123, "0", 123},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := params
			p.MaxTaskReward = c.cap
			task := &Task{Reward: c.reward}
			if got := DeterminePayout(task, p); got != c.expected {
				t.Fatalf("DeterminePayout(%d,%q) = %d, want %d", c.reward, c.cap, got, c.expected)
			}
		})
	}
}
