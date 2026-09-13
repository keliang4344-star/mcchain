package oraclesvc

// 生产加固：周期性自检（liveness / 降级探测）。
//
// 不引入 HTTP server、Prometheus 等新依赖 —— 自检结果一律通过分级日志暴露，
// 由外部日志告警（journald / Loki / ELK）来触发运维动作。

import (
	"context"
	"sync"
	"time"
)

// 自检默认参数。
const (
	DefaultHealthInterval = 60 * time.Second
	DefaultStaleThreshold = 10 * time.Minute
)

// HealthMonitor 记录关键操作（链上提交 / 签名）的成败与最近一次成功时间，
// 并由 Run 启动的 goroutine 周期性判定服务是否已降级。
//
// 所有方法对 nil 接收者安全，调用方可以在未启用监控时直接传 nil。
type HealthMonitor struct {
	name      string
	interval  time.Duration
	threshold time.Duration

	mu          sync.Mutex
	startedAt   time.Time
	lastSuccess time.Time
	successes   uint64
	failures    uint64
	degraded    bool
}

// NewHealthMonitor 构造一个自检监控器。
//   - name      ：被监控操作的名称（写入日志）。
//   - interval  ：自检周期（<=0 用默认 60s）。
//   - threshold ：多久没有成功即视为可能降级（<=0 用默认 10min）。
func NewHealthMonitor(name string, interval, threshold time.Duration) *HealthMonitor {
	if interval <= 0 {
		interval = DefaultHealthInterval
	}
	if threshold <= 0 {
		threshold = DefaultStaleThreshold
	}
	return &HealthMonitor{
		name:      name,
		interval:  interval,
		threshold: threshold,
		startedAt: time.Now(),
	}
}

// MarkSuccess 记录一次成功操作，刷新「最近成功时间」。
func (h *HealthMonitor) MarkSuccess() {
	if h == nil {
		return
	}
	h.mu.Lock()
	h.lastSuccess = time.Now()
	h.successes++
	recovered := h.degraded
	h.degraded = false
	h.mu.Unlock()

	// 已从降级状态恢复，显式记一条 INFO 便于对齐告警恢复（不持锁写日志）。
	if recovered {
		Infof("health[%s]: recovered, service is healthy again", h.name)
	}
}

// MarkFailure 记录一次失败操作（仅计数，不影响最近成功时间）。
func (h *HealthMonitor) MarkFailure() {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.failures++
}

// Stats 返回当前统计快照：最近成功时间（零值表示从未成功）、成功数、失败数。
func (h *HealthMonitor) Stats() (lastSuccess time.Time, successes, failures uint64) {
	if h == nil {
		return time.Time{}, 0, 0
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lastSuccess, h.successes, h.failures
}

// Run 阻塞运行周期自检，直到 ctx 被取消。通常以 `go monitor.Run(ctx)` 启动。
//
// 判定规则（以「距最近一次成功的时长」为准）：
//   - 超过 2×threshold        → ERROR，服务大概率已不可用
//   - 超过 1×threshold        → WARN ，服务可能降级
//   - 否则                    → INFO ，心跳
//
// 启动后尚无任何成功记录时，以进程启动时间作为参照。
func (h *HealthMonitor) Run(ctx context.Context) {
	if h == nil {
		return
	}
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	Infof("health[%s]: self-check started (interval=%s, stale-threshold=%s)",
		h.name, h.interval, h.threshold)

	for {
		select {
		case <-ctx.Done():
			Infof("health[%s]: self-check stopped: %v", h.name, ctx.Err())
			return
		case <-ticker.C:
			h.check()
		}
	}
}

// check 执行一次自检并输出对应级别的日志。
func (h *HealthMonitor) check() {
	h.mu.Lock()
	last, successes, failures := h.lastSuccess, h.successes, h.failures
	ref, everSucceeded := last, !last.IsZero()
	if !everSucceeded {
		ref = h.startedAt
	}
	since := time.Since(ref).Truncate(time.Second)
	degradedNow := since > h.threshold
	h.degraded = degradedNow
	h.mu.Unlock()

	lastDesc := "never"
	if everSucceeded {
		lastDesc = last.UTC().Format(time.RFC3339)
	}

	switch {
	case since > 2*h.threshold:
		Errorf("health[%s]: DEGRADED - no successful operation for %s (last-success=%s, ok=%d, failed=%d); service likely unavailable, check chain node connectivity and oracle account balance",
			h.name, since, lastDesc, successes, failures)
	case degradedNow:
		Warnf("health[%s]: no successful operation for %s (last-success=%s, ok=%d, failed=%d); service may be degraded",
			h.name, since, lastDesc, successes, failures)
	default:
		Infof("health[%s]: ok (last-success=%s, ok=%d, failed=%d)", h.name, lastDesc, successes, failures)
	}
}
