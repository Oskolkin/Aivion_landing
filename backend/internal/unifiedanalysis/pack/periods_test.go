package pack

import (
	"testing"
	"time"
)

func TestComputePackPeriodsCanonical(t *testing.T) {
	metricDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	p := ComputePackPeriods(metricDate)

	if p.Current7D.From != "2026-07-02" || p.Current7D.To != "2026-07-08" {
		t.Fatalf("current_7d=%+v", p.Current7D)
	}
	if p.Previous7D.From != "2026-06-25" || p.Previous7D.To != "2026-07-01" {
		t.Fatalf("previous_7d=%+v", p.Previous7D)
	}
	if p.Trend.From != "2026-06-09" || p.Trend.To != "2026-07-08" {
		t.Fatalf("trend=%+v", p.Trend)
	}
}

func TestComputePackPeriodsTrendSpanMatchesHistoryDays(t *testing.T) {
	metricDate := time.Date(2026, 1, 3, 12, 30, 0, 0, time.UTC)
	p := ComputePackPeriods(metricDate)
	days := len(dateRangeInclusive(p.Trend.From, p.Trend.To))
	if days != 30 {
		t.Fatalf("trend span days=%d want=30", days)
	}
	current := len(dateRangeInclusive(p.Current7D.From, p.Current7D.To))
	if current != 7 {
		t.Fatalf("current_7d span=%d want=7", current)
	}
	previous := len(dateRangeInclusive(p.Previous7D.From, p.Previous7D.To))
	if previous != 7 {
		t.Fatalf("previous_7d span=%d want=7", previous)
	}
}

func TestComputePackPeriodsNonOverlapping(t *testing.T) {
	metricDate := time.Date(2026, 7, 8, 23, 59, 59, 0, time.UTC)
	p := ComputePackPeriods(metricDate)
	if p.Previous7D.To >= p.Current7D.From {
		t.Fatalf("previous_7d.to=%s should be < current_7d.from=%s", p.Previous7D.To, p.Current7D.From)
	}
}

func TestComputePackPeriodsToContract(t *testing.T) {
	metricDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	p := ComputePackPeriods(metricDate)
	c := p.ToContract()
	if c.Current7D.From != p.Current7D.From || c.Current7D.To != p.Current7D.To {
		t.Fatalf("current_7d mismatch: %+v vs %+v", c.Current7D, p.Current7D)
	}
	if c.Trend.From != p.Trend.From || c.Trend.To != p.Trend.To {
		t.Fatalf("trend mismatch: %+v vs %+v", c.Trend, p.Trend)
	}
}

func TestDateRangeInclusive(t *testing.T) {
	dates := dateRangeInclusive("2026-01-30", "2026-02-02")
	want := []string{"2026-01-30", "2026-01-31", "2026-02-01", "2026-02-02"}
	if len(dates) != len(want) {
		t.Fatalf("dates=%v want=%v", dates, want)
	}
	for i := range want {
		if dates[i] != want[i] {
			t.Fatalf("dates=%v want=%v", dates, want)
		}
	}
	if got := dateRangeInclusive("2026-02-02", "2026-01-30"); got != nil {
		t.Fatalf("reversed range should be nil, got=%v", got)
	}
}
