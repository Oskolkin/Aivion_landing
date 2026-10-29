package recommendations

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockRepository struct {
	getDailyAccountMetricByDate func(ctx context.Context, sellerAccountID int64, metricDate time.Time) (*AccountDailyMetric, error)
	listDailySKUMetricsByDate   func(ctx context.Context, sellerAccountID int64, metricDate time.Time) ([]SKUDailyMetric, error)
	listOpenAlerts              func(ctx context.Context, sellerAccountID int64, limit int, offset int) ([]AlertSignal, error)
	countOpenAlertsBySeverity   func(ctx context.Context, sellerAccountID int64) ([]NamedCount, error)
	countOpenAlertsByGroup      func(ctx context.Context, sellerAccountID int64) ([]NamedCount, error)
	getLatestAlertRun           func(ctx context.Context, sellerAccountID int64) (*RunInfo, error)
	listAdCampaigns             func(ctx context.Context, sellerAccountID int64, dateFrom time.Time, dateTo time.Time) ([]AdCampaignSummary, error)
	listEffectiveConstraints    func(ctx context.Context, sellerAccountID int64) ([]EffectiveConstraint, error)
	listOpenRecommendations     func(ctx context.Context, sellerAccountID int64, limit int, offset int) ([]RecommendationDigest, error)
	countOpenRecommendations    func(ctx context.Context, sellerAccountID int64) (int64, error)
	countRecsByPriority         func(ctx context.Context, sellerAccountID int64) ([]NamedCount, error)
	countRecsByConfidence       func(ctx context.Context, sellerAccountID int64) ([]NamedCount, error)
	getLatestRecommendationRun  func(ctx context.Context, sellerAccountID int64) (*RunInfo, error)
}

func (m mockRepository) GetDailyAccountMetricByDate(ctx context.Context, sellerAccountID int64, metricDate time.Time) (*AccountDailyMetric, error) {
	return m.getDailyAccountMetricByDate(ctx, sellerAccountID, metricDate)
}
func (m mockRepository) ListDailySKUMetricsByDate(ctx context.Context, sellerAccountID int64, metricDate time.Time) ([]SKUDailyMetric, error) {
	return m.listDailySKUMetricsByDate(ctx, sellerAccountID, metricDate)
}
func (m mockRepository) ListOpenAlerts(ctx context.Context, sellerAccountID int64, limit int, offset int) ([]AlertSignal, error) {
	return m.listOpenAlerts(ctx, sellerAccountID, limit, offset)
}
func (m mockRepository) CountOpenAlertsBySeverity(ctx context.Context, sellerAccountID int64) ([]NamedCount, error) {
	return m.countOpenAlertsBySeverity(ctx, sellerAccountID)
}
func (m mockRepository) CountOpenAlertsByGroup(ctx context.Context, sellerAccountID int64) ([]NamedCount, error) {
	return m.countOpenAlertsByGroup(ctx, sellerAccountID)
}
func (m mockRepository) GetLatestAlertRun(ctx context.Context, sellerAccountID int64) (*RunInfo, error) {
	return m.getLatestAlertRun(ctx, sellerAccountID)
}
func (m mockRepository) ListAdCampaignSummariesByDateRange(ctx context.Context, sellerAccountID int64, dateFrom time.Time, dateTo time.Time) ([]AdCampaignSummary, error) {
	return m.listAdCampaigns(ctx, sellerAccountID, dateFrom, dateTo)
}
func (m mockRepository) ListSKUEffectiveConstraintsBySellerAccountID(ctx context.Context, sellerAccountID int64) ([]EffectiveConstraint, error) {
	return m.listEffectiveConstraints(ctx, sellerAccountID)
}
func (m mockRepository) ListOpenRecommendations(ctx context.Context, sellerAccountID int64, limit int, offset int) ([]RecommendationDigest, error) {
	return m.listOpenRecommendations(ctx, sellerAccountID, limit, offset)
}
func (m mockRepository) CountOpenRecommendations(ctx context.Context, sellerAccountID int64) (int64, error) {
	return m.countOpenRecommendations(ctx, sellerAccountID)
}
func (m mockRepository) CountOpenRecommendationsByPriority(ctx context.Context, sellerAccountID int64) ([]NamedCount, error) {
	return m.countRecsByPriority(ctx, sellerAccountID)
}
func (m mockRepository) CountOpenRecommendationsByConfidence(ctx context.Context, sellerAccountID int64) ([]NamedCount, error) {
	return m.countRecsByConfidence(ctx, sellerAccountID)
}
func (m mockRepository) GetLatestRecommendationRun(ctx context.Context, sellerAccountID int64) (*RunInfo, error) {
	return m.getLatestRecommendationRun(ctx, sellerAccountID)
}

func TestContextBuilderBuildForAccount(t *testing.T) {
	now := time.Date(2026, 4, 30, 12, 34, 56, 0, time.UTC)
	repo := mockRepository{
		getDailyAccountMetricByDate: func(_ context.Context, _ int64, metricDate time.Time) (*AccountDailyMetric, error) {
			if metricDate.Equal(time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC)) {
				return &AccountDailyMetric{MetricDate: "2026-04-29", Revenue: 500, OrdersCount: 25}, nil
			}
			return &AccountDailyMetric{MetricDate: "2026-04-30", Revenue: 600, OrdersCount: 30}, nil
		},
		listDailySKUMetricsByDate: func(_ context.Context, _ int64, _ time.Time) ([]SKUDailyMetric, error) {
			doc := 5.0
			return []SKUDailyMetric{
				{OzonProductID: 2, Revenue: 90, StockAvailable: 10},
				{OzonProductID: 1, Revenue: 120, StockAvailable: 0, DaysOfCover: &doc},
			}, nil
		},
		listOpenAlerts: func(_ context.Context, _ int64, _ int, _ int) ([]AlertSignal, error) {
			evidence := map[string]any{"stock_available": 0}
			return []AlertSignal{
				{ID: 11, AlertType: "ad_spend_without_result", Severity: "medium", Evidence: evidence},
				{ID: 10, AlertType: "stock_oos_risk", Severity: "critical", Evidence: evidence},
			}, nil
		},
		countOpenAlertsBySeverity: func(_ context.Context, _ int64) ([]NamedCount, error) {
			return []NamedCount{{Name: "high", Count: 2}, {Name: "critical", Count: 1}}, nil
		},
		countOpenAlertsByGroup: func(_ context.Context, _ int64) ([]NamedCount, error) {
			return []NamedCount{{Name: "stock", Count: 3}}, nil
		},
		getLatestAlertRun: func(_ context.Context, _ int64) (*RunInfo, error) {
			return &RunInfo{ID: 1, RunType: "scheduled", Status: "completed", StartedAt: now}, nil
		},
		listAdCampaigns: func(_ context.Context, _ int64, _ time.Time, _ time.Time) ([]AdCampaignSummary, error) {
			return []AdCampaignSummary{{CampaignExternalID: 2, SpendTotal: 100}, {CampaignExternalID: 1, SpendTotal: 150}}, nil
		},
		listEffectiveConstraints: func(_ context.Context, _ int64) ([]EffectiveConstraint, error) {
			return []EffectiveConstraint{{OzonProductID: 2}, {OzonProductID: 1}}, nil
		},
		listOpenRecommendations: func(_ context.Context, _ int64, _ int, _ int) ([]RecommendationDigest, error) {
			return []RecommendationDigest{{ID: 100, Title: "Do X"}}, nil
		},
		countOpenRecommendations: func(_ context.Context, _ int64) (int64, error) {
			return 1, nil
		},
		countRecsByPriority: func(_ context.Context, _ int64) ([]NamedCount, error) {
			return []NamedCount{{Name: "high", Count: 1}}, nil
		},
		countRecsByConfidence: func(_ context.Context, _ int64) ([]NamedCount, error) {
			return []NamedCount{{Name: "medium", Count: 1}}, nil
		},
		getLatestRecommendationRun: func(_ context.Context, _ int64) (*RunInfo, error) {
			return &RunInfo{ID: 2, RunType: "manual", Status: "completed", StartedAt: now}, nil
		},
	}

	builder := NewContextBuilder(repo)
	got, err := builder.BuildForAccount(context.Background(), 77, now)
	if err != nil {
		t.Fatalf("BuildForAccount returned error: %v", err)
	}
	if got.SellerAccountID != 77 {
		t.Fatalf("unexpected seller account id: %d", got.SellerAccountID)
	}
	if got.AsOfDate != "2026-04-30" {
		t.Fatalf("unexpected as_of_date: %s", got.AsOfDate)
	}
	if got.Windows.PreviousDate != "2026-04-29" {
		t.Fatalf("unexpected previous date: %s", got.Windows.PreviousDate)
	}
	if got.Alerts.OpenTotal != 1 {
		t.Fatalf("unexpected active alerts open total: %d", got.Alerts.OpenTotal)
	}
	if got.Alerts.OpenCriticalHighCount != 1 {
		t.Fatalf("unexpected critical/high count: %d", got.Alerts.OpenCriticalHighCount)
	}
	if len(got.Alerts.TopOpen) != 1 || got.Alerts.TopOpen[0].AlertType != "stock_oos_risk" {
		t.Fatalf("legacy ad alert must be excluded from top_open: %+v", got.Alerts.TopOpen)
	}
	if got.Alerts.TopOpen[0].Severity != "critical" {
		t.Fatalf("expected critical alert first in top_open: %+v", got.Alerts.TopOpen)
	}
	if len(got.Alerts.TopOpen[0].Evidence) == 0 {
		t.Fatalf("expected evidence_payload on alerts in context")
	}
	if got.Recommendations.OpenTotal != 1 {
		t.Fatalf("unexpected recommendations open total: %d", got.Recommendations.OpenTotal)
	}
	if len(got.Merchandising.TopRevenueSKUs) != 2 || got.Merchandising.TopRevenueSKUs[0].OzonProductID != 1 {
		t.Fatalf("top revenue skus not sorted as expected: %+v", got.Merchandising.TopRevenueSKUs)
	}
	if len(got.Merchandising.LowStockSKUs) != 1 || got.Merchandising.LowStockSKUs[0].OzonProductID != 1 {
		t.Fatalf("low stock skus mismatch: %+v", got.Merchandising.LowStockSKUs)
	}
	if len(got.Advertising.TopCampaigns) != 2 || got.Advertising.TopCampaigns[0].CampaignExternalID != 1 {
		t.Fatalf("top campaigns not sorted by spend: %+v", got.Advertising.TopCampaigns)
	}
	if got.Account.RevenueDeltaPct == nil || *got.Account.RevenueDeltaPct != 20 {
		t.Fatalf("unexpected revenue delta: %+v", got.Account.RevenueDeltaPct)
	}
}

func TestContextBuilderExcludesLegacyAlertsFromContext(t *testing.T) {
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	repo := mockRepository{
		getDailyAccountMetricByDate: func(_ context.Context, _ int64, _ time.Time) (*AccountDailyMetric, error) {
			return &AccountDailyMetric{MetricDate: "2026-05-01", Revenue: 100, OrdersCount: 5}, nil
		},
		listDailySKUMetricsByDate: func(_ context.Context, _ int64, _ time.Time) ([]SKUDailyMetric, error) {
			return nil, nil
		},
		listOpenAlerts: func(_ context.Context, _ int64, _ int, _ int) ([]AlertSignal, error) {
			return []AlertSignal{
				{ID: 1, AlertType: "ad_spend_without_result", AlertGroup: "advertising", Severity: "high"},
				{ID: 2, AlertType: "price_below_min_constraint", AlertGroup: "price_economics", Severity: "high"},
				{ID: 3, AlertType: "ad_weak_campaign_efficiency", AlertGroup: "advertising", Severity: "medium"},
			}, nil
		},
		countOpenAlertsBySeverity: func(_ context.Context, _ int64) ([]NamedCount, error) {
			return []NamedCount{{Name: "high", Count: 99}}, nil
		},
		countOpenAlertsByGroup: func(_ context.Context, _ int64) ([]NamedCount, error) {
			return []NamedCount{{Name: "price_economics", Count: 99}}, nil
		},
		getLatestAlertRun: func(_ context.Context, _ int64) (*RunInfo, error) {
			return &RunInfo{ID: 1, Status: "completed"}, nil
		},
		listAdCampaigns: func(_ context.Context, _ int64, _ time.Time, _ time.Time) ([]AdCampaignSummary, error) {
			return nil, nil
		},
		listEffectiveConstraints:   func(_ context.Context, _ int64) ([]EffectiveConstraint, error) { return nil, nil },
		listOpenRecommendations:    func(_ context.Context, _ int64, _ int, _ int) ([]RecommendationDigest, error) { return nil, nil },
		countOpenRecommendations:   func(_ context.Context, _ int64) (int64, error) { return 0, nil },
		countRecsByPriority:        func(_ context.Context, _ int64) ([]NamedCount, error) { return nil, nil },
		countRecsByConfidence:      func(_ context.Context, _ int64) ([]NamedCount, error) { return nil, nil },
		getLatestRecommendationRun: func(_ context.Context, _ int64) (*RunInfo, error) { return nil, nil },
	}
	ctx, err := NewContextBuilder(repo).BuildForAccount(context.Background(), 1, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(ctx.Alerts.TopOpen) != 1 || ctx.Alerts.TopOpen[0].AlertType != "ad_weak_campaign_efficiency" {
		t.Fatalf("top_open: %+v", ctx.Alerts.TopOpen)
	}
	if ctx.Alerts.OpenTotal != 1 {
		t.Fatalf("open_total=%d want 1", ctx.Alerts.OpenTotal)
	}
	for _, g := range ctx.Alerts.ByGroup {
		if g.Name == "price_economics" {
			t.Fatalf("legacy group in context: %+v", ctx.Alerts.ByGroup)
		}
	}
}

func TestContextBuilderBuildForAccountRepoError(t *testing.T) {
	expectedErr := errors.New("boom")
	repo := mockRepository{
		getDailyAccountMetricByDate: func(_ context.Context, _ int64, _ time.Time) (*AccountDailyMetric, error) {
			return nil, expectedErr
		},
		listDailySKUMetricsByDate: func(_ context.Context, _ int64, _ time.Time) ([]SKUDailyMetric, error) {
			return nil, nil
		},
		listOpenAlerts: func(_ context.Context, _ int64, _ int, _ int) ([]AlertSignal, error) {
			return nil, nil
		},
		countOpenAlertsBySeverity: func(_ context.Context, _ int64) ([]NamedCount, error) {
			return nil, nil
		},
		countOpenAlertsByGroup: func(_ context.Context, _ int64) ([]NamedCount, error) {
			return nil, nil
		},
		getLatestAlertRun: func(_ context.Context, _ int64) (*RunInfo, error) {
			return nil, nil
		},
		listAdCampaigns: func(_ context.Context, _ int64, _ time.Time, _ time.Time) ([]AdCampaignSummary, error) {
			return nil, nil
		},
		listEffectiveConstraints: func(_ context.Context, _ int64) ([]EffectiveConstraint, error) {
			return nil, nil
		},
		listOpenRecommendations: func(_ context.Context, _ int64, _ int, _ int) ([]RecommendationDigest, error) {
			return nil, nil
		},
		countOpenRecommendations: func(_ context.Context, _ int64) (int64, error) {
			return 0, nil
		},
		countRecsByPriority: func(_ context.Context, _ int64) ([]NamedCount, error) {
			return nil, nil
		},
		countRecsByConfidence: func(_ context.Context, _ int64) ([]NamedCount, error) {
			return nil, nil
		},
		getLatestRecommendationRun: func(_ context.Context, _ int64) (*RunInfo, error) {
			return nil, nil
		},
	}

	builder := NewContextBuilder(repo)
	_, err := builder.BuildForAccount(context.Background(), 77, time.Now().UTC())
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}
