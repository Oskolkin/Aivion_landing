package analysisdata

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSQLLoaderRejectsNonPositiveSellerID(t *testing.T) {
	loader := &SQLLoader{}
	_, err := loader.Load(context.Background(), 0, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrInvalidSellerAccountID) && !strings.Contains(err.Error(), "seller_account_id must be > 0") {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = loader.Load(context.Background(), -1, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("expected error for negative id")
	}
}

func TestSQLLoaderRequiresQueries(t *testing.T) {
	loader := &SQLLoader{}
	_, err := loader.Load(context.Background(), 1, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("expected configuration error")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestComputeDataWindowsFourteenDayRange(t *testing.T) {
	w := ComputeDataWindows(time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC))
	from := ParseDate(w.Last14d.From)
	to := ParseDate(w.Last14d.To)
	days := int(to.Sub(from).Hours()/24) + 1
	if days != 14 {
		t.Fatalf("last_14d span days=%d from=%s to=%s", days, w.Last14d.From, w.Last14d.To)
	}
	if w.CurrentDay != FormatDate(DateOnlyUTC(time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC))) {
		t.Fatalf("metric date not date-only: %s", w.CurrentDay)
	}
}
