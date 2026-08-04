package oraclesvc

// 生产加固：对链上提交 / 签名请求等易失败操作的指数退避重试。
// 全部基于标准库 context/time，不引入外部依赖。

import (
	"context"
	"fmt"
	"time"
)

// 重试默认参数：最多 4 次尝试（首次 + 3 次重试），间隔 500ms → 1s → 2s，上限 8s。
const (
	DefaultRetryAttempts = 4
	DefaultRetryBaseWait = 500 * time.Millisecond
	DefaultRetryMaxWait  = 8 * time.Second
)

// RetryPolicy 描述一次重试策略。零值不可用，请用 DefaultRetryPolicy。
type RetryPolicy struct {
	// Attempts 为总尝试次数（含首次）。小于 1 时按 1 处理。
	Attempts int
	// BaseWait 为首次退避间隔，之后每次翻倍。
	BaseWait time.Duration
	// MaxWait 为单次退避间隔上限，避免退避时间无限增长。
	MaxWait time.Duration
}

// DefaultRetryPolicy 返回生产默认的退避策略。
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		Attempts: DefaultRetryAttempts,
		BaseWait: DefaultRetryBaseWait,
		MaxWait:  DefaultRetryMaxWait,
	}
}

// normalized 修正非法字段，保证策略可用。
func (p RetryPolicy) normalized() RetryPolicy {
	if p.Attempts < 1 {
		p.Attempts = 1
	}
	if p.BaseWait <= 0 {
		p.BaseWait = DefaultRetryBaseWait
	}
	if p.MaxWait <= 0 || p.MaxWait < p.BaseWait {
		p.MaxWait = DefaultRetryMaxWait
	}
	return p
}

// Retry 以指数退避执行 fn，直到成功、尝试次数耗尽，或 ctx 被取消。
//
//   - op 仅用于日志标识（例如 "broadcast-tx"）。
//   - 每次失败记 WARN 日志（含尝试序号与下次退避间隔）；最终失败由调用方记 ERROR。
//   - 退避等待期间尊重 ctx.Done()，可被优雅取消，便于进程快速退出。
func Retry(ctx context.Context, op string, policy RetryPolicy, fn func(ctx context.Context) error) error {
	p := policy.normalized()

	var lastErr error
	wait := p.BaseWait

	for attempt := 1; attempt <= p.Attempts; attempt++ {
		// 每次尝试前先确认上下文仍然有效，避免退出过程中继续打链上请求。
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("%s canceled before attempt %d/%d: %w", op, attempt, p.Attempts, err)
		}

		lastErr = fn(ctx)
		if lastErr == nil {
			if attempt > 1 {
				Infof("%s succeeded on attempt %d/%d", op, attempt, p.Attempts)
			}
			return nil
		}

		if attempt == p.Attempts {
			break
		}

		Warnf("%s failed (attempt %d/%d): %v; retrying in %s", op, attempt, p.Attempts, lastErr, wait)
		if err := SleepCtx(ctx, wait); err != nil {
			return fmt.Errorf("%s canceled while backing off after attempt %d/%d (last error: %v): %w",
				op, attempt, p.Attempts, lastErr, err)
		}

		// 指数退避：间隔翻倍并封顶。
		if wait *= 2; wait > p.MaxWait {
			wait = p.MaxWait
		}
	}

	return fmt.Errorf("%s failed after %d attempts: %w", op, p.Attempts, lastErr)
}

// SleepCtx 等待 d，或在 ctx 取消时提前返回 ctx.Err()。
func SleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
