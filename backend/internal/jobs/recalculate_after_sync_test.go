package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/alerts"
	"go.uber.org/zap"
)

type fakePostSyncAccount struct {
	lastFrom, lastTo time.Time
	n                int
	err              error
}

func (f *fakePostSyncAccount) RebuildDailyAccountMetricsForDateRange(ctx context.Context, sellerAccountID int64, dateFrom, dateTo time.Time) (int, error) {
	f.lastFrom = dateFrom
	f.lastTo = dateTo
	return f.n, f.err
}

type fakePostSyncSKU struct {
	n   int
	err error
}

func (f *fakePostSyncSKU) RebuildDailySKUMetricsForDateRange(ctx context.Context, sellerAccountID int64, dateFrom, dateTo time.Time) (int, error) {
	return f.n, f.err
}

type fakePostSyncAlerts struct {
	lastAsOf time.Time
	lastType alerts.RunType
	summary  alerts.RunForAccountSummary
	err      error
}

func (f *fakePostSyncAlerts) RunForAccountWithType(ctx context.Context, sellerAccountID int64, asOfDate time.Time, runType alerts.RunType) (alerts.RunForAccountSummary, error) {
	f.lastAsOf = asOfDate
	f.lastType = runType
	return f.summary, f.err
}

type fakePostSyncFunnel struct {
	lastSeller int64
	lastFrom   time.Time
	lastTo     time.Time
	err        error
}

func (f *fakePostSyncFunnel) SyncFunnelForSellerRange(_ context.Context, sellerAccountID int64, fromDate, toDate time.Time) error {
	f.lastSeller = sellerAccountID
	f.lastFrom = fromDate
	f.lastTo = toDate
	return f.err
}

func TestRecalculateAfterSyncHandler_runsDownstream(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	acc := &fakePostSyncAccount{n: 2}
	sku := &fakePostSyncSKU{n: 3}
	funnel := &fakePostSyncFunnel{}
	al := &fakePostSyncAlerts{
		summary: alerts.RunForAccountSummary{RunID: 99, Status: alerts.RunStatusCompleted},
	}
	h := NewRecalculateAfterSyncHandler(acc, sku, funnel, al, nil, nil, zap.NewNop(), nil)
	payload, err := json.Marshal(RecalculateAfterSyncPayload{SellerAccountID: 7, SyncJobID: 55})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Handle(ctx, payload); err != nil {
		t.Fatal(err)
	}
	if acc.lastFrom.IsZero() || acc.lastTo.IsZero() {
		t.Fatal("expected date range on account rebuild")
	}
	if inclusive := int(acc.lastTo.Sub(acc.lastFrom)/(24*time.Hour)) + 1; inclusive != postSyncMetricsLookbackDays {
		t.Fatalf("expected %d inclusive days, got %d", postSyncMetricsLookbackDays, inclusive)
	}
	if al.lastType != alerts.RunTypePostSync {
		t.Fatalf("unexpected run type: %s", al.lastType)
	}
	if !postSyncCalendarDayUTC(time.Now()).Equal(al.lastAsOf) {
		t.Fatalf("expected as_of end-of-window today UTC")
	}
	if funnel.lastSeller != 7 {
		t.Fatalf("funnel seller=%d", funnel.lastSeller)
	}
	today := postSyncCalendarDayUTC(time.Now())
	if !today.AddDate(0, 0, -1).Equal(funnel.lastFrom) || !today.Equal(funnel.lastTo) {
		t.Fatalf("unexpected funnel range from=%s to=%s", funnel.lastFrom.Format("2006-01-02"), funnel.lastTo.Format("2006-01-02"))
	}
}

func TestRecalculateAfterSyncHandler_propagatesAccountError(t *testing.T) {
	t.Parallel()
	want := errors.New("boom")
	acc := &fakePostSyncAccount{err: want}
	h := NewRecalculateAfterSyncHandler(acc, &fakePostSyncSKU{}, nil, &fakePostSyncAlerts{}, nil, nil, zap.NewNop(), nil)
	payload, _ := json.Marshal(RecalculateAfterSyncPayload{SellerAccountID: 1, SyncJobID: 2})
	if err := h.Handle(context.Background(), payload); !errors.Is(err, want) {
		t.Fatalf("expected wrapped boom, got %v", err)
	}
}

func TestRecalculateAfterSyncHandler_propagatesAlertsError(t *testing.T) {
	t.Parallel()
	want := errors.New("alerts down")
	al := &fakePostSyncAlerts{err: want}
	h := NewRecalculateAfterSyncHandler(&fakePostSyncAccount{n: 1}, &fakePostSyncSKU{n: 1}, nil, al, nil, nil, zap.NewNop(), nil)
	payload, _ := json.Marshal(RecalculateAfterSyncPayload{SellerAccountID: 1, SyncJobID: 2})
	if err := h.Handle(context.Background(), payload); !errors.Is(err, want) {
		t.Fatalf("expected alerts error, got %v", err)
	}
}

func TestRecalculateAfterSyncHandler_idempotentRetries(t *testing.T) {
	t.Parallel()
	acc := &fakePostSyncAccount{n: 1}
	sku := &fakePostSyncSKU{n: 1}
	al := &fakePostSyncAlerts{summary: alerts.RunForAccountSummary{RunID: 1, Status: alerts.RunStatusCompleted}}
	h := NewRecalculateAfterSyncHandler(acc, sku, nil, al, nil, nil, zap.NewNop(), nil)
	payload, _ := json.Marshal(RecalculateAfterSyncPayload{SellerAccountID: 1, SyncJobID: 2})
	ctx := context.Background()
	if err := h.Handle(ctx, payload); err != nil {
		t.Fatal(err)
	}
	if err := h.Handle(ctx, payload); err != nil {
		t.Fatal(err)
	}
}

func TestRecalculateAfterSyncHandler_funnelErrorDoesNotFailPostSync(t *testing.T) {
	t.Parallel()
	funnel := &fakePostSyncFunnel{err: errors.New("funnel db down")}
	h := NewRecalculateAfterSyncHandler(
		&fakePostSyncAccount{n: 1},
		&fakePostSyncSKU{n: 1},
		funnel,
		&fakePostSyncAlerts{summary: alerts.RunForAccountSummary{RunID: 1, Status: alerts.RunStatusCompleted}},
		nil,
		nil,
		zap.NewNop(),
		nil,
	)
	payload, _ := json.Marshal(RecalculateAfterSyncPayload{SellerAccountID: 1, SyncJobID: 2})
	if err := h.Handle(context.Background(), payload); err != nil {
		t.Fatalf("funnel error should not fail post-sync: %v", err)
	}
}

func TestRecalculateAfterSyncHandler_invokesUnifiedAutoRunTrigger(t *testing.T) {
	t.Parallel()
	called := 0
	var gotSeller int64
	h := NewRecalculateAfterSyncHandler(
		&fakePostSyncAccount{n: 1},
		&fakePostSyncSKU{n: 1},
		nil,
		&fakePostSyncAlerts{summary: alerts.RunForAccountSummary{RunID: 1, Status: alerts.RunStatusCompleted}},
		nil,
		nil,
		zap.NewNop(),
		nil,
	)
	h.SetUnifiedAutoRunTrigger(func(ctx context.Context, sellerAccountID int64) error {
		called++
		gotSeller = sellerAccountID
		return nil
	})
	payload, _ := json.Marshal(RecalculateAfterSyncPayload{SellerAccountID: 42, SyncJobID: 7})
	if err := h.Handle(context.Background(), payload); err != nil {
		t.Fatal(err)
	}
	if called != 1 || gotSeller != 42 {
		t.Fatalf("unified auto-run trigger called=%d seller=%d", called, gotSeller)
	}
}

func TestRecalculateAfterSyncHandler_unifiedAutoRunErrorDoesNotFailPostSync(t *testing.T) {
	t.Parallel()
	h := NewRecalculateAfterSyncHandler(
		&fakePostSyncAccount{n: 1},
		&fakePostSyncSKU{n: 1},
		nil,
		&fakePostSyncAlerts{summary: alerts.RunForAccountSummary{RunID: 1, Status: alerts.RunStatusCompleted}},
		nil,
		nil,
		zap.NewNop(),
		nil,
	)
	h.SetUnifiedAutoRunTrigger(func(ctx context.Context, sellerAccountID int64) error {
		return errors.New("unified create failed")
	})
	payload, _ := json.Marshal(RecalculateAfterSyncPayload{SellerAccountID: 1, SyncJobID: 2})
	if err := h.Handle(context.Background(), payload); err != nil {
		t.Fatalf("unified auto-run error must not fail post-sync: %v", err)
	}
}

func TestNewRecalculateAfterSyncTask_roundtrip(t *testing.T) {
	t.Parallel()
	task, err := NewRecalculateAfterSyncTask(10, 20)
	if err != nil {
		t.Fatal(err)
	}
	if task.Type() != TaskTypePostSyncRecalculation {
		t.Fatalf("type %q", task.Type())
	}
	var got RecalculateAfterSyncPayload
	if err := json.Unmarshal(task.Payload(), &got); err != nil {
		t.Fatal(err)
	}
	if got.SellerAccountID != 10 || got.SyncJobID != 20 {
		t.Fatalf("payload %+v", got)
	}
}
