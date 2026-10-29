package analytics

import (
	"math"
	"testing"
	"time"
)

func TestAccountDRRPct_CabinetFormula(t *testing.T) {
	got := AccountDRRPct(23400, 120618)
	if got == nil {
		t.Fatal("expected DRR")
	}
	if math.Abs(*got-19.4) > 0.05 {
		t.Fatalf("got %v want ~19.4", *got)
	}
}

func TestAccountDRRPct_ZeroDenominatorNil(t *testing.T) {
	if AccountDRRPct(100, 0) != nil {
		t.Fatal("expected nil")
	}
}

func TestAccountDRRPeriodPct_SumOfSums(t *testing.T) {
	from := time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	spend := map[string]float64{
		"2026-03-10": 300,
		"2026-03-09": 100,
	}
	rev := map[string]float64{
		"2026-03-10": 1000,
		"2026-03-09": 1000,
	}
	got := AccountDRRPeriodPct(spend, rev, from, to)
	if got == nil || math.Abs(*got-20) > 0.01 {
		t.Fatalf("got %v want 20", got)
	}
}

func TestBuildAccountDRRDailySeries_RealDailyValues(t *testing.T) {
	from := time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	spend := map[string]float64{"2026-03-09": 100, "2026-03-10": 300}
	rev := map[string]float64{"2026-03-09": 1000, "2026-03-10": 1000}
	series := BuildAccountDRRDailySeries(from, to, spend, rev)
	if len(series) != 2 {
		t.Fatalf("len=%d", len(series))
	}
	if series[0].DRRPct == nil || math.Abs(*series[0].DRRPct-10) > 0.01 {
		t.Fatalf("day0=%v want 10", series[0].DRRPct)
	}
	if series[1].DRRPct == nil || math.Abs(*series[1].DRRPct-30) > 0.01 {
		t.Fatalf("day1=%v want 30", series[1].DRRPct)
	}
}
