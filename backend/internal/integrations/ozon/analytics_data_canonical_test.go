package ozon

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestCanonicalAnalyticsParserPreservesExactNumbersAndIgnoresTotals(t *testing.T) {
	raw := []byte(`{"result":{"data":[{"dimensions":[{"id":"1"},{"id":"2026-08-01"}],"metrics":[9007199254740993.123456,10.0]}],"totals":[999,999]}}`)
	r, err := ParseCanonicalAnalyticsResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Data) != 1 || r.Data[0].Metrics[0].String() != "9007199254740993.123456" || r.Data[0].Metrics[1].String() != "10.0" {
		t.Fatalf("parsed=%+v", r)
	}
}

func TestIntervalLimiterCancellation(t *testing.T) {
	l := NewIntervalLimiter(time.Hour)
	if err := l.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := l.Wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}

func TestRetryAfterParsing(t *testing.T) {
	now := time.Now()
	if d := parseRetryAfter("12", now); d != 12*time.Second {
		t.Fatalf("seconds=%v", d)
	}
	if d := parseRetryAfter(now.Add(time.Minute).UTC().Format(http.TimeFormat), now); d < 59*time.Second || d > time.Minute {
		t.Fatalf("date=%v", d)
	}
	if parseRetryAfter("bad", now) != 0 {
		t.Fatal("invalid Retry-After accepted")
	}
}
