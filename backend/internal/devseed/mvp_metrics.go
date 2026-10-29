package devseed

import (
	"context"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RebuildMVPAnalytics runs existing analytics services over freshly inserted commerce data,
// bounded to [startDate, anchorDate] and trimming any metric rows beyond anchorDate.
func RebuildMVPAnalytics(ctx context.Context, pool *pgxpool.Pool, sellerAccountID int64, anchorDate time.Time, days int) error {
	anchor := MVPDateUTC(anchorDate)
	start := SeedStartDate(anchor, days)

	ac := analytics.NewAccountMetricsService(pool)
	if _, err := ac.RebuildDailyAccountMetricsForDateRange(ctx, sellerAccountID, start, anchor); err != nil {
		return fmt.Errorf("rebuild account metrics: %w", err)
	}
	sku := analytics.NewSKUMetricsService(pool)
	if _, err := sku.RebuildDailySKUMetricsForDateRange(ctx, sellerAccountID, start, anchor); err != nil {
		return fmt.Errorf("rebuild sku metrics: %w", err)
	}
	if err := trimMVPMetricsBeyondAnchor(ctx, pool, sellerAccountID, anchor); err != nil {
		return fmt.Errorf("trim metrics beyond anchor: %w", err)
	}
	// Demo SKU-focus scenarios (order-drop / stock-risk / no-sales / key-sku) are encoded in
	// SOURCE orders/sales/stocks by BuildMVPScenarioPlan, so they are reproduced by the
	// rebuild above. No post-rebuild UPDATE of daily_sku_metrics happens here.
	return nil
}

func trimMVPMetricsBeyondAnchor(ctx context.Context, pool *pgxpool.Pool, sellerAccountID int64, anchor time.Time) error {
	anchor = MVPDateUTC(anchor)
	_, err := pool.Exec(ctx, `
DELETE FROM daily_account_metrics
WHERE seller_account_id = $1 AND metric_date > $2::date`,
		sellerAccountID, anchor)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
DELETE FROM daily_sku_metrics
WHERE seller_account_id = $1 AND metric_date > $2::date`,
		sellerAccountID, anchor)
	return err
}

// MVPFutureBusinessDateCount returns rows in core dashboard tables with dates after anchorDate.
func MVPFutureBusinessDateCount(ctx context.Context, pool *pgxpool.Pool, sellerAccountID int64, anchorDate time.Time) (int64, error) {
	anchor := MVPDateUTC(anchorDate)
	var n int64
	err := pool.QueryRow(ctx, `
SELECT
  (SELECT COUNT(*) FROM daily_account_metrics WHERE seller_account_id = $1 AND metric_date > $2::date) +
  (SELECT COUNT(*) FROM daily_sku_metrics WHERE seller_account_id = $1 AND metric_date > $2::date) +
  (SELECT COUNT(*) FROM ad_metrics_daily WHERE seller_account_id = $1 AND metric_date > $2::date) +
  (SELECT COUNT(*) FROM product_analytics_daily WHERE seller_account_id = $1 AND metric_date > $2::date) +
  (SELECT COUNT(*) FROM stock_analytics_snapshots WHERE seller_account_id = $1 AND snapshot_date > $2::date) +
  (SELECT COUNT(*) FROM recommendation_runs WHERE seller_account_id = $1 AND as_of_date > $2::date) +
  (SELECT COUNT(*) FROM ai_daily_summaries WHERE seller_account_id = $1 AND metric_date > $2::date)`,
		sellerAccountID, anchor).Scan(&n)
	return n, err
}

// LoadMVPDateSummary fills date-range summary fields on result from the database.
func LoadMVPDateSummary(ctx context.Context, pool *pgxpool.Pool, sellerAccountID int64, opts MVPSeedOptions, into *MVPSeedResult) error {
	anchor := MVPDateUTC(opts.AnchorDate)
	into.AnchorDate = anchor.Format("2006-01-02")
	into.GeneratedDateStart = SeedStartDate(anchor, opts.Days).Format("2006-01-02")
	into.GeneratedDateEnd = anchor.Format("2006-01-02")
	into.ExpectedDays = opts.Days

	row := pool.QueryRow(ctx, `
SELECT
  (SELECT MIN(metric_date)::text FROM daily_account_metrics WHERE seller_account_id = $1),
  (SELECT MAX(metric_date)::text FROM daily_account_metrics WHERE seller_account_id = $1),
  (SELECT COUNT(DISTINCT metric_date) FROM daily_sku_metrics WHERE seller_account_id = $1)`,
		sellerAccountID)
	var minAcc, maxAcc *string
	var skuDistinct int64
	if err := row.Scan(&minAcc, &maxAcc, &skuDistinct); err != nil {
		return fmt.Errorf("load date summary: %w", err)
	}
	if minAcc != nil {
		into.AccountMetricMinDate = *minAcc
	}
	if maxAcc != nil {
		into.AccountMetricMaxDate = *maxAcc
	}
	into.SKUMetricDistinctDates = skuDistinct

	future, err := MVPFutureBusinessDateCount(ctx, pool, sellerAccountID, anchor)
	if err != nil {
		return err
	}
	into.FutureBusinessRowsFound = future

	var paRows, paDates int64
	_ = pool.QueryRow(ctx, `
SELECT COUNT(*), COUNT(DISTINCT metric_date)
FROM product_analytics_daily
WHERE seller_account_id = $1`, sellerAccountID).Scan(&paRows, &paDates)
	into.ProductAnalyticsRows = paRows
	into.ProductAnalyticsDistinctDates = paDates

	funnelOK, err := MVPAnchorFunnelAvailable(ctx, pool, sellerAccountID, anchor)
	if err != nil {
		return err
	}
	into.FunnelAvailableForAnchor = funnelOK

	_ = pool.QueryRow(ctx, `
SELECT COUNT(*) FROM products
WHERE seller_account_id = $1
  AND primary_image_url IS NOT NULL
  AND btrim(primary_image_url) <> ''`, sellerAccountID).Scan(&into.ProductsWithPrimaryImage)

	return nil
}
