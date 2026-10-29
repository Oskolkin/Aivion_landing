package alerts

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeRepo struct {
	createRunCalls   int
	completeRunCalls int
	failRunCalls     int
	upsertAlertCalls int
	fingerprints     []string

	errAdCampaignSummaries error

	asOf           time.Time
	currentMetric  *AccountDailyMetric
	previousMetric *AccountDailyMetric
}

func (f *fakeRepo) UpsertAlert(ctx context.Context, input UpsertAlertInput) (Alert, error) {
	f.upsertAlertCalls++
	f.fingerprints = append(f.fingerprints, input.Fingerprint)
	return Alert{
		ID:              int64(f.upsertAlertCalls),
		SellerAccountID: input.SellerAccountID,
		AlertType:       input.AlertType,
	}, nil
}

func (f *fakeRepo) GetAlertByID(ctx context.Context, sellerAccountID int64, alertID int64) (Alert, error) {
	return Alert{}, nil
}

func (f *fakeRepo) ListOpenAlerts(ctx context.Context, sellerAccountID int64, limit int, offset int) ([]Alert, error) {
	return nil, nil
}

func (f *fakeRepo) ResolveAlert(ctx context.Context, sellerAccountID int64, alertID int64) (Alert, error) {
	return Alert{}, nil
}

func (f *fakeRepo) DismissAlert(ctx context.Context, sellerAccountID int64, alertID int64) (Alert, error) {
	return Alert{}, nil
}

func (f *fakeRepo) CreateRun(ctx context.Context, sellerAccountID int64, runType RunType) (AlertRun, error) {
	f.createRunCalls++
	return AlertRun{
		ID:              42,
		SellerAccountID: sellerAccountID,
		RunType:         runType,
		Status:          RunStatusRunning,
		StartedAt:       time.Now().UTC(),
	}, nil
}

func (f *fakeRepo) CompleteRun(ctx context.Context, input CompleteRunInput) (AlertRun, error) {
	f.completeRunCalls++
	now := time.Now().UTC()
	return AlertRun{
		ID:               input.RunID,
		SellerAccountID:  input.SellerAccountID,
		Status:           RunStatusCompleted,
		SalesAlertsCount: input.SalesAlertsCount,
		StockAlertsCount: input.StockAlertsCount,
		AdAlertsCount:    input.AdAlertsCount,
		PriceAlertsCount: input.PriceAlertsCount,
		TotalAlertsCount: input.TotalAlertsCount,
		FinishedAt:       &now,
	}, nil
}

func (f *fakeRepo) FailRun(ctx context.Context, sellerAccountID int64, runID int64, errorMessage string) (AlertRun, error) {
	f.failRunCalls++
	now := time.Now().UTC()
	return AlertRun{
		ID:              runID,
		SellerAccountID: sellerAccountID,
		Status:          RunStatusFailed,
		FinishedAt:      &now,
		ErrorMessage:    &errorMessage,
	}, nil
}

func (f *fakeRepo) GetLatestRun(ctx context.Context, sellerAccountID int64) (AlertRun, error) {
	return AlertRun{}, nil
}

func (f *fakeRepo) GetDailyAccountMetricByDate(ctx context.Context, sellerAccountID int64, metricDate time.Time) (*AccountDailyMetric, error) {
	d := metricDate.UTC().Truncate(24 * time.Hour)
	if d.Equal(f.asOf) {
		return f.currentMetric, nil
	}
	if d.Equal(f.asOf.AddDate(0, 0, -1)) {
		return f.previousMetric, nil
	}
	return nil, nil
}

func (f *fakeRepo) ListDailySKUMetricsByDate(ctx context.Context, sellerAccountID int64, metricDate time.Time) ([]SKUDailyMetric, error) {
	return nil, nil
}

func (f *fakeRepo) ListDailySKUMetricsByDateRange(ctx context.Context, sellerAccountID int64, dateFrom time.Time, dateTo time.Time) ([]SKUDailyMetric, error) {
	return nil, nil
}

func (f *fakeRepo) ListAlertsFiltered(ctx context.Context, sellerAccountID int64, filter ListFilter) ([]Alert, error) {
	return nil, nil
}

func (f *fakeRepo) CountOpenAlertsBySeverity(ctx context.Context, sellerAccountID int64) ([]SeverityCount, error) {
	return nil, nil
}

func (f *fakeRepo) CountOpenAlertsByGroup(ctx context.Context, sellerAccountID int64) ([]GroupCount, error) {
	return nil, nil
}

func (f *fakeRepo) CountOpenAlerts(ctx context.Context, sellerAccountID int64) (int64, error) {
	return 0, nil
}

func (f *fakeRepo) ListAdCampaignSummariesByDateRange(ctx context.Context, sellerAccountID int64, dateFrom time.Time, dateTo time.Time) ([]AdCampaignMetricSummary, error) {
	if f.errAdCampaignSummaries != nil {
		return nil, f.errAdCampaignSummaries
	}
	return nil, nil
}

func (f *fakeRepo) ListAdCampaignSKUMappings(ctx context.Context, sellerAccountID int64) ([]AdCampaignSKUMapping, error) {
	return nil, nil
}

func (f *fakeRepo) ListProductsBySellerAccountID(ctx context.Context, sellerAccountID int64) ([]dbgen.Product, error) {
	return nil, nil
}

func (f *fakeRepo) ListSKUEffectiveConstraintsBySellerAccountID(ctx context.Context, sellerAccountID int64) ([]dbgen.SkuEffectiveConstraint, error) {
	return nil, nil
}

func (f *fakeRepo) ListStockAnalyticsSnapshotsBySellerAndDate(ctx context.Context, sellerAccountID int64, snapshotDate time.Time) ([]dbgen.StockAnalyticsSnapshot, error) {
	return nil, nil
}

func TestRunForAccountSuccessCreatesAndCompletesRun(t *testing.T) {
	asOf := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)
	repo := &fakeRepo{
		asOf: asOf,
		currentMetric: &AccountDailyMetric{
			SellerAccountID: 1,
			MetricDate:      asOf,
			Revenue:         60,
			OrdersCount:     60,
		},
		previousMetric: &AccountDailyMetric{
			SellerAccountID: 1,
			MetricDate:      asOf.AddDate(0, 0, -1),
			Revenue:         100,
			OrdersCount:     100,
		},
	}
	svc := NewService(repo)

	summary, err := svc.RunForAccount(context.Background(), 1, asOf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.createRunCalls != 1 {
		t.Fatalf("expected create run call, got %d", repo.createRunCalls)
	}
	if repo.completeRunCalls != 1 {
		t.Fatalf("expected complete run call, got %d", repo.completeRunCalls)
	}
	if repo.failRunCalls != 0 {
		t.Fatalf("expected no fail run call, got %d", repo.failRunCalls)
	}
	if summary.Status != RunStatusCompleted {
		t.Fatalf("unexpected summary status: %s", summary.Status)
	}
	if summary.TotalUpsertedAlerts != summary.Sales.UpsertedAlerts+summary.Stock.UpsertedAlerts+summary.Advertising.UpsertedAlerts+summary.PriceEconomics.UpsertedAlerts {
		t.Fatalf("total upserted alerts mismatch")
	}
}

func TestRunForAccountFailsRunOnGroupError(t *testing.T) {
	asOf := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)
	repo := &fakeRepo{
		asOf:                   asOf,
		currentMetric:          &AccountDailyMetric{SellerAccountID: 1, MetricDate: asOf, Revenue: 100, OrdersCount: 100},
		previousMetric:         &AccountDailyMetric{SellerAccountID: 1, MetricDate: asOf.AddDate(0, 0, -1), Revenue: 100, OrdersCount: 100},
		errAdCampaignSummaries: errors.New("ad source unavailable"),
	}
	svc := NewService(repo)

	_, err := svc.RunForAccount(context.Background(), 1, asOf)
	if err == nil {
		t.Fatal("expected orchestration error")
	}
	if repo.createRunCalls != 1 {
		t.Fatalf("expected create run call, got %d", repo.createRunCalls)
	}
	if repo.completeRunCalls != 0 {
		t.Fatalf("expected no complete run call, got %d", repo.completeRunCalls)
	}
	if repo.failRunCalls != 1 {
		t.Fatalf("expected fail run call, got %d", repo.failRunCalls)
	}
}

func TestRunForAccountRepeatedRunKeepsStableFingerprints(t *testing.T) {
	asOf := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)
	repo := &fakeRepo{
		asOf: asOf,
		currentMetric: &AccountDailyMetric{
			SellerAccountID: 1,
			MetricDate:      asOf,
			Revenue:         60,
			OrdersCount:     60,
		},
		previousMetric: &AccountDailyMetric{
			SellerAccountID: 1,
			MetricDate:      asOf.AddDate(0, 0, -1),
			Revenue:         100,
			OrdersCount:     100,
		},
	}
	svc := NewService(repo)

	if _, err := svc.RunForAccount(context.Background(), 1, asOf); err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	firstRunFingerprints := append([]string(nil), repo.fingerprints...)
	if len(firstRunFingerprints) == 0 {
		t.Fatal("expected at least one upserted fingerprint on first run")
	}

	if _, err := svc.RunForAccount(context.Background(), 1, asOf); err != nil {
		t.Fatalf("second run failed: %v", err)
	}
	secondRunFingerprints := repo.fingerprints[len(firstRunFingerprints):]
	if len(secondRunFingerprints) != len(firstRunFingerprints) {
		t.Fatalf("fingerprint count mismatch between runs: first=%d second=%d", len(firstRunFingerprints), len(secondRunFingerprints))
	}
	for i := range firstRunFingerprints {
		if firstRunFingerprints[i] != secondRunFingerprints[i] {
			t.Fatalf("fingerprint mismatch at index %d: first=%s second=%s", i, firstRunFingerprints[i], secondRunFingerprints[i])
		}
	}
}

var _ Repository = (*fakeRepo)(nil)

// Keep pgtype imported in this test file for interface drift visibility.
var _ = pgtype.Text{}
