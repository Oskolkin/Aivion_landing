package analysisdata

import (
	"testing"
	"time"
)

func TestHistoryDateRangeFourteenAndThirty(t *testing.T) {
	metric := time.Date(2026, 7, 8, 15, 0, 0, 0, time.UTC)
	from14, to14, err := HistoryDateRange(metric, 14)
	if err != nil {
		t.Fatal(err)
	}
	if FormatDate(from14) != "2026-06-25" || FormatDate(to14) != "2026-07-08" {
		t.Fatalf("14d from=%s to=%s", FormatDate(from14), FormatDate(to14))
	}
	from30, to30, err := HistoryDateRange(metric, 30)
	if err != nil {
		t.Fatal(err)
	}
	if FormatDate(from30) != "2026-06-09" || FormatDate(to30) != "2026-07-08" {
		t.Fatalf("30d from=%s to=%s", FormatDate(from30), FormatDate(to30))
	}
}

func TestHistoryDateRangeRejectsInvalid(t *testing.T) {
	_, _, err := HistoryDateRange(time.Now(), 0)
	if err == nil {
		t.Fatal("expected error for 0")
	}
	_, _, err = HistoryDateRange(time.Now(), -1)
	if err == nil {
		t.Fatal("expected error for negative")
	}
	_, _, err = HistoryDateRange(time.Now(), MaxHistoryDays+1)
	if err == nil {
		t.Fatal("expected error for too large")
	}
}
