package admin

import (
	"context"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
)

// AnalyticsMetricsRerunner runs account- and SKU-level daily metric rebuilds for a date range.
type AnalyticsMetricsRerunner struct {
	Account *analytics.AccountMetricsService
	SKU     *analytics.SKUMetricsService
}

func NewAnalyticsMetricsRerunner(account *analytics.AccountMetricsService, sku *analytics.SKUMetricsService) *AnalyticsMetricsRerunner {
	return &AnalyticsMetricsRerunner{Account: account, SKU: sku}
}

func (r *AnalyticsMetricsRerunner) Rerun(ctx context.Context, sellerAccountID int64, dateFrom, dateTo time.Time) (map[string]any, error) {
	nAcc, err := r.Account.RebuildDailyAccountMetricsForDateRange(ctx, sellerAccountID, dateFrom, dateTo)
	if err != nil {
		return nil, err
	}
	nSku, err := r.SKU.RebuildDailySKUMetricsForDateRange(ctx, sellerAccountID, dateFrom, dateTo)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"status":                     "completed",
		"account_daily_metrics_rows": nAcc,
		"sku_daily_metrics_rows":     nSku,
	}, nil
}
