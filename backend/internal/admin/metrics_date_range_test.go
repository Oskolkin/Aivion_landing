package admin

import (
	"testing"
	"time"
)

func TestResolveRerunMetricsDateRange_defaultsLast30UTC(t *testing.T) {
	t.Parallel()
	in := RerunMetricsInput{}
	from, to, err := resolveRerunMetricsDateRange(in)
	if err != nil {
		t.Fatal(err)
	}
	if inclusiveDaySpan(from, to) != metricsRerunDefaultInclusiveDays {
		t.Fatalf("want %d inclusive days, got %d (%s..%s)", metricsRerunDefaultInclusiveDays, inclusiveDaySpan(from, to), from.Format("2006-01-02"), to.Format("2006-01-02"))
	}
	if !from.Before(to) && !from.Equal(to) {
		t.Fatalf("from after to: %v %v", from, to)
	}
	today := metricsCalendarDayUTC(time.Now())
	if !to.Equal(today) {
		t.Fatalf("expected default end=today UTC, got %v", to)
	}
}

func TestResolveRerunMetricsDateRange_explicit(t *testing.T) {
	t.Parallel()
	// Local instant that maps to the previous calendar day in UTC.
	from := time.Date(2026, 1, 1, 2, 0, 0, 0, time.FixedZone("L", 14*3600))
	to := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	gotFrom, gotTo, err := resolveRerunMetricsDateRange(RerunMetricsInput{DateFrom: from, DateTo: to})
	if err != nil {
		t.Fatal(err)
	}
	if gotFrom.Format("2006-01-02") != "2025-12-31" || gotTo.Format("2006-01-02") != "2026-01-05" {
		t.Fatalf("unexpected normalization: %s..%s", gotFrom.Format("2006-01-02"), gotTo.Format("2006-01-02"))
	}
}

func TestResolveRerunMetricsDateRange_rejectsFromAfterTo(t *testing.T) {
	t.Parallel()
	_, _, err := resolveRerunMetricsDateRange(RerunMetricsInput{
		DateFrom: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		DateTo:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveRerunMetricsDateRange_rejectsPartial(t *testing.T) {
	t.Parallel()
	_, _, err := resolveRerunMetricsDateRange(RerunMetricsInput{
		DateFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveRerunMetricsDateRange_rejectsTooLarge(t *testing.T) {
	t.Parallel()
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 366)
	_, _, err := resolveRerunMetricsDateRange(RerunMetricsInput{DateFrom: from, DateTo: to})
	if err == nil {
		t.Fatal("expected error")
	}
}
