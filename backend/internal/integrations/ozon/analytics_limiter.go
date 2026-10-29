package ozon

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"
)

type AnalyticsRequestLimiter interface{ Wait(context.Context) error }

type IntervalLimiter struct {
	mu       sync.Mutex
	next     time.Time
	interval time.Duration
}

func NewIntervalLimiter(interval time.Duration) *IntervalLimiter {
	return &IntervalLimiter{interval: interval}
}

func NewDefaultAnalyticsLimiter() *IntervalLimiter {
	d := time.Minute
	if raw := strings.TrimSpace(os.Getenv("OZON_ANALYTICS_MIN_INTERVAL")); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed >= 0 {
			d = parsed
		}
	}
	return NewIntervalLimiter(d)
}

func (l *IntervalLimiter) Wait(ctx context.Context) error {
	l.mu.Lock()
	now := time.Now()
	wait := time.Duration(0)
	if now.Before(l.next) {
		wait = l.next.Sub(now)
	}
	l.next = now.Add(wait).Add(l.interval)
	l.mu.Unlock()
	if wait <= 0 {
		return nil
	}
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
