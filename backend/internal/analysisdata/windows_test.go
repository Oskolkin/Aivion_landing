package analysisdata

import (
	"testing"
	"time"
)

func TestComputeDataWindowsCanonical(t *testing.T) {
	metricDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	w := ComputeDataWindows(metricDate)

	if w.CurrentDay != "2026-07-08" {
		t.Fatalf("current_day=%s", w.CurrentDay)
	}
	if w.Yesterday != "2026-07-07" {
		t.Fatalf("yesterday=%s", w.Yesterday)
	}
	if w.Last7d.From != "2026-07-02" || w.Last7d.To != "2026-07-08" {
		t.Fatalf("last_7d=%+v", w.Last7d)
	}
	if w.Previous7d.From != "2026-06-25" || w.Previous7d.To != "2026-07-01" {
		t.Fatalf("previous_7d=%+v", w.Previous7d)
	}
	if w.Last14d.From != "2026-06-25" || w.Last14d.To != "2026-07-08" {
		t.Fatalf("last_14d=%+v", w.Last14d)
	}
}

func TestComputeDataWindowsMonthYearBoundary(t *testing.T) {
	w := ComputeDataWindows(time.Date(2026, 1, 3, 12, 30, 0, 0, time.UTC))
	if w.CurrentDay != "2026-01-03" {
		t.Fatalf("current_day=%s", w.CurrentDay)
	}
	if w.Yesterday != "2026-01-02" {
		t.Fatalf("yesterday=%s", w.Yesterday)
	}
	if w.Last7d.From != "2025-12-28" || w.Last7d.To != "2026-01-03" {
		t.Fatalf("last_7d=%+v", w.Last7d)
	}
	if w.Previous7d.From != "2025-12-21" || w.Previous7d.To != "2025-12-27" {
		t.Fatalf("previous_7d=%+v", w.Previous7d)
	}
	if w.Last14d.From != "2025-12-21" || w.Last14d.To != "2026-01-03" {
		t.Fatalf("last_14d=%+v", w.Last14d)
	}
}

func TestComputeDataWindowsNonZeroTimeAndOffset(t *testing.T) {
	loc := time.FixedZone("UTC+3", 3*3600)
	w := ComputeDataWindows(time.Date(2026, 7, 8, 23, 59, 59, 0, loc))
	// 23:59 UTC+3 == 20:59 UTC same calendar day.
	if w.CurrentDay != "2026-07-08" {
		t.Fatalf("current_day=%s", w.CurrentDay)
	}

	lateUTC := time.Date(2026, 7, 8, 15, 45, 10, 0, time.UTC)
	w2 := ComputeDataWindows(lateUTC)
	if w2.CurrentDay != "2026-07-08" || w2.Yesterday != "2026-07-07" {
		t.Fatalf("windows=%+v", w2)
	}
}
