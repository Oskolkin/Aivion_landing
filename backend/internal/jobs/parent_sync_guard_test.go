package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/alerts"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

func TestMapParentSyncGuardErrorToAsynq_Retryable(t *testing.T) {
	t.Parallel()
	for _, err := range []error{
		guardRetry(ErrParentSyncPending, 1, 2, "pending", "completed"),
		guardRetry(ErrParentSyncRunning, 1, 2, "running", "completed"),
	} {
		mapped := MapParentSyncGuardErrorToAsynq(err)
		if mapped == nil {
			t.Fatal("expected error")
		}
		if errors.Is(mapped, asynq.SkipRetry) {
			t.Fatalf("retryable guard must not map to SkipRetry: %v", mapped)
		}
		if !strings.Contains(mapped.Error(), "parent sync not ready") {
			t.Fatalf("unexpected message: %v", mapped)
		}
	}
}

func TestMapParentSyncGuardErrorToAsynq_Terminal(t *testing.T) {
	t.Parallel()
	for _, err := range []error{
		guardPermanent(ErrParentSyncFailed, 1, 2, "failed", "completed"),
		guardPermanent(ErrParentSyncCancelled, 1, 2, "cancelled", "completed"),
		guardPermanent(ErrParentSyncNotFound, 1, 2, "", ""),
		guardPermanent(ErrParentSyncSellerMismatch, 1, 2, "", ""),
		guardPermanent(ErrParentSyncMetricMismatch, 1, 2, "2026-01-01", "2026-01-02"),
		guardPermanent(ErrParentSyncUnknownStatus, 1, 2, "weird", "completed"),
	} {
		mapped := MapParentSyncGuardErrorToAsynq(err)
		if !errors.Is(mapped, asynq.SkipRetry) {
			t.Fatalf("expected SkipRetry for %v, got %v", err, mapped)
		}
	}
}

func TestIsRetryableParentSyncGuardError(t *testing.T) {
	t.Parallel()
	if !IsRetryableParentSyncGuardError(guardRetry(ErrParentSyncPending, 1, 1, "", "")) {
		t.Fatal("pending should be retryable")
	}
	if !IsRetryableParentSyncGuardError(guardRetry(ErrParentSyncRunning, 1, 1, "", "")) {
		t.Fatal("running should be retryable")
	}
	if IsRetryableParentSyncGuardError(guardPermanent(ErrParentSyncFailed, 1, 1, "", "")) {
		t.Fatal("failed should not be retryable")
	}
}

func TestIsTerminalParentSyncGuardError(t *testing.T) {
	t.Parallel()
	if !IsTerminalParentSyncGuardError(guardPermanent(ErrParentSyncNotFound, 1, 1, "", "")) {
		t.Fatal("missing should be terminal")
	}
	if IsTerminalParentSyncGuardError(guardRetry(ErrParentSyncRunning, 1, 1, "", "")) {
		t.Fatal("running should not be terminal")
	}
}

func TestSameCalendarDayUTC(t *testing.T) {
	t.Parallel()
	a := time.Date(2026, 7, 7, 15, 0, 0, 0, time.FixedZone("MSK", 3*3600))
	b := time.Date(2026, 7, 7, 1, 0, 0, 0, time.UTC)
	if !sameCalendarDayUTC(a, b) {
		t.Fatal("expected same UTC calendar day")
	}
	c := time.Date(2026, 7, 6, 23, 0, 0, 0, time.UTC)
	if sameCalendarDayUTC(a, c) {
		t.Fatal("expected different days")
	}
}

func TestParseOptionalMetricDate(t *testing.T) {
	t.Parallel()
	nilDate, err := parseOptionalMetricDate("")
	if err != nil || nilDate != nil {
		t.Fatalf("empty date: %v err=%v", nilDate, err)
	}
	parsed, err := parseOptionalMetricDate("2026-05-10")
	if err != nil {
		t.Fatal(err)
	}
	if parsed == nil || !parsed.Equal(time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("parsed=%v", parsed)
	}
	if _, err := parseOptionalMetricDate("not-a-date"); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestParentSyncGuardError_Unwrap(t *testing.T) {
	t.Parallel()
	err := guardPermanent(ErrParentSyncSellerMismatch, 9, 8, "", "")
	if !errors.Is(err, ErrParentSyncSellerMismatch) {
		t.Fatalf("unwrap failed: %v", err)
	}
	var ge *ParentSyncGuardError
	if !errors.As(err, &ge) || ge.Code != "seller_mismatch" {
		t.Fatalf("as failed: %+v", ge)
	}
}

func TestGuardRunsBeforeRecalculation(t *testing.T) {
	t.Parallel()
	guard := &stubParentSyncGuard{err: guardRetry(ErrParentSyncRunning, 1, 1, "", "")}
	acc := &guardProbeAccount{}
	sku := &guardProbeSKU{}
	h := &RecalculateAfterSyncHandler{
		account:     acc,
		sku:         sku,
		alerts:      &fakePostSyncAlerts{summary: alerts.RunForAccountSummary{RunID: 1, Status: alerts.RunStatusCompleted}},
		parentGuard: guard,
		log:         zap.NewNop(),
	}
	payload, err := json.Marshal(RecalculateAfterSyncPayload{SellerAccountID: 1, SyncJobID: 1})
	if err != nil {
		t.Fatal(err)
	}
	err = h.Handle(context.Background(), payload)
	if err == nil || errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("expected retryable guard error, got %v", err)
	}
	if !guard.called {
		t.Fatal("guard was not called")
	}
	if acc.called || sku.called {
		t.Fatal("downstream must not run before guard allows")
	}
}

type stubParentSyncGuard struct {
	called bool
	err    error
	out    ValidatedParentSync
}

func (s *stubParentSyncGuard) RequireCompletedParentSync(ctx context.Context, syncJobID, sellerAccountID int64, requestedMetricDate *time.Time) (ValidatedParentSync, error) {
	s.called = true
	if s.err != nil {
		return ValidatedParentSync{}, s.err
	}
	return s.out, nil
}

type guardProbeAccount struct {
	called bool
}

func (g *guardProbeAccount) RebuildDailyAccountMetricsForDateRange(ctx context.Context, sellerAccountID int64, dateFrom, dateTo time.Time) (int, error) {
	g.called = true
	return 0, nil
}

type guardProbeSKU struct {
	called bool
}

func (g *guardProbeSKU) RebuildDailySKUMetricsForDateRange(ctx context.Context, sellerAccountID int64, dateFrom, dateTo time.Time) (int, error) {
	g.called = true
	return 0, nil
}
