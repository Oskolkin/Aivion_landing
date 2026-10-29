package performance

import (
	"context"
	"errors"
	"math/rand"
	"time"
)

type RetryConfig struct {
	MaxAttempts int
	BaseDelays  []time.Duration
	JitterMax   time.Duration
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		BaseDelays: []time.Duration{
			1 * time.Second,
			3 * time.Second,
			7 * time.Second,
		},
		JitterMax: 300 * time.Millisecond,
	}
}

func shouldRetry(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrInvalidCredentials) {
		return false
	}
	if errors.Is(err, ErrNetworkError) {
		return true
	}
	var providerErr *ProviderError
	if errors.As(err, &providerErr) {
		return providerErr.StatusCode == 429 || providerErr.StatusCode >= 500
	}
	return false
}

func sleepWithJitter(ctx context.Context, base time.Duration, jitterMax time.Duration) error {
	jitter := time.Duration(rand.Int63n(int64(jitterMax + 1)))
	timer := time.NewTimer(base + jitter)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func withRetry[T any](ctx context.Context, cfg RetryConfig, fn func() (T, error)) (T, error) {
	var zero T
	var lastErr error
	attempts := cfg.MaxAttempts
	if attempts <= 0 {
		attempts = 1
	}
	for attempt := 1; attempt <= attempts; attempt++ {
		result, err := fn()
		if err == nil {
			return result, nil
		}
		lastErr = err
		if !shouldRetry(err) || attempt == attempts {
			return zero, lastErr
		}
		delay := 1 * time.Second
		if len(cfg.BaseDelays) >= attempt {
			delay = cfg.BaseDelays[attempt-1]
		} else if len(cfg.BaseDelays) > 0 {
			delay = cfg.BaseDelays[len(cfg.BaseDelays)-1]
		}
		if err := sleepWithJitter(ctx, delay, cfg.JitterMax); err != nil {
			return zero, err
		}
	}
	return zero, lastErr
}
