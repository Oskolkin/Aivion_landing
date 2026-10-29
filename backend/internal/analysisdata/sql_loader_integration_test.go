//go:build integration

package analysisdata_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testDSN(t *testing.T) string {
	t.Helper()
	v := os.Getenv("TEST_DATABASE_URL")
	if v == "" {
		t.Skip("set TEST_DATABASE_URL to run analysisdata integration tests")
		return ""
	}
	testutil.RequireSafeTestDatabaseURL(t, v)
	return v
}

func openPool(t *testing.T, dsn string) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("db connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("db ping: %v", err)
	}
	return pool
}

func TestSQLLoaderLoad_RealPostgres(t *testing.T) {
	pool := openPool(t, testDSN(t))
	ctx := context.Background()
	metricDate := time.Date(2026, 7, 8, 15, 45, 0, 0, time.UTC)

	sellerA := createSeller(t, pool, "analysisdata-a")
	sellerB := createSeller(t, pool, "analysisdata-b")
	seedSellerWindow(t, pool, sellerA, metricDate, true)
	seedSellerWindow(t, pool, sellerB, metricDate, false)
	seedOutOfRangeRows(t, pool, sellerA, metricDate)

	loader := analysisdata.NewSQLLoader(pool)
	snap, err := loader.Load(ctx, sellerA, metricDate)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if snap.SellerAccountID != sellerA {
		t.Fatalf("seller=%d want %d", snap.SellerAccountID, sellerA)
	}
	if !snap.MetricDate.Valid || analysisdata.FormatDate(snap.MetricDate.Time) != "2026-07-08" {
		t.Fatalf("metric_date=%v", snap.MetricDate)
	}
	if snap.Windows.CurrentDay != "2026-07-08" ||
		snap.Windows.Last14d.From != "2026-06-25" ||
		snap.Windows.Last14d.To != "2026-07-08" {
		t.Fatalf("windows=%+v", snap.Windows)
	}
	if len(snap.AccountMetrics) == 0 {
		t.Fatal("expected account metrics")
	}
	if len(snap.SKUMetrics) == 0 {
		t.Fatal("expected sku metrics")
	}
	if len(snap.Products) == 0 {
		t.Fatal("expected products")
	}
	if len(snap.StockSnapshots) == 0 {
		t.Fatal("expected stock snapshots")
	}
	if len(snap.Campaigns) == 0 || len(snap.AdMetrics) == 0 {
		t.Fatal("expected advertising rows")
	}
	if len(snap.FunnelRows) == 0 {
		t.Fatal("expected funnel rows")
	}
	if len(snap.Categories) == 0 {
		t.Fatal("expected categories")
	}
	// constraints / api capabilities may be empty (nil or len 0) when seller has none.
	if snap.PriceConstraints == nil {
		snap.PriceConstraints = []dbgen.SkuEffectiveConstraint{}
	}
	if snap.APICapabilities == nil {
		snap.APICapabilities = []dbgen.OzonApiCapability{}
	}
	_ = snap.PriceConstraints
	_ = snap.APICapabilities
	// enrichment run is nullable by design.
	_ = snap.EnrichmentRun

	for _, row := range snap.AccountMetrics {
		if row.SellerAccountID != sellerA {
			t.Fatalf("account metric leaked seller %d", row.SellerAccountID)
		}
		d := analysisdata.FormatDate(row.MetricDate.Time)
		if d < snap.Windows.Last14d.From || d > snap.Windows.Last14d.To {
			t.Fatalf("account metric outside window: %s", d)
		}
	}
	for _, row := range snap.SKUMetrics {
		if row.SellerAccountID != sellerA {
			t.Fatalf("sku metric leaked seller %d", row.SellerAccountID)
		}
	}
	for _, row := range snap.AdMetrics {
		if row.SellerAccountID != sellerA {
			t.Fatalf("ad metric leaked seller %d", row.SellerAccountID)
		}
	}
	for _, row := range snap.FunnelRows {
		if row.SellerAccountID != sellerA {
			t.Fatalf("funnel leaked seller %d", row.SellerAccountID)
		}
		if row.ScopeType != "account" {
			t.Fatalf("unexpected funnel scope_type=%s", row.ScopeType)
		}
	}

	// Legacy Build via shared loader.
	svc := aidatapack.NewService(pool)
	pack, err := svc.Build(ctx, sellerA, metricDate)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if pack.SchemaVersion != "ai_data_pack_v1" {
		t.Fatalf("schema=%s", pack.SchemaVersion)
	}
	if pack.SellerContext.SellerAccountID != sellerA {
		t.Fatalf("pack seller=%d", pack.SellerContext.SellerAccountID)
	}
	if pack.DataWindows.CurrentDay != "2026-07-08" {
		t.Fatalf("pack windows=%+v", pack.DataWindows)
	}
	if len(pack.EntityRegistry.SKUs) == 0 {
		t.Fatal("expected entities")
	}
	if len(pack.FactRegistry) == 0 {
		t.Fatal("expected facts")
	}
	if pack.Tables.AccountRollup.DataStatus == "" {
		t.Fatal("expected account rollup")
	}
	if err := aidatapack.ValidateDataPack(pack); err != nil {
		t.Fatalf("validate: %v", err)
	}

	// LoadWithOptions with the Unified Analysis Pack's 30-day window must reach
	// further back than the legacy 14-day Load: the out-of-range row seeded at
	// metricDate-20d falls outside Load's window but inside the 30d window.
	snap30, err := loader.LoadWithOptions(ctx, analysisdata.LoadOptions{
		SellerAccountID: sellerA,
		MetricDate:      metricDate,
		HistoryDays:     analysisdata.UnifiedPackHistoryDays,
	})
	if err != nil {
		t.Fatalf("LoadWithOptions(30d): %v", err)
	}
	if snap30.LoadedHistory.Days != 30 {
		t.Fatalf("30d LoadedHistory.Days=%d want 30", snap30.LoadedHistory.Days)
	}
	if snap30.LoadedHistory.From != "2026-06-09" || snap30.LoadedHistory.To != "2026-07-08" {
		t.Fatalf("30d LoadedHistory=%+v want From=2026-06-09 To=2026-07-08", snap30.LoadedHistory)
	}
	if len(snap30.AccountMetrics) <= len(snap.AccountMetrics) {
		t.Fatalf("30d window (%d rows) should return more account metrics than 14d window (%d rows)",
			len(snap30.AccountMetrics), len(snap.AccountMetrics))
	}
	outsideDate := "2026-06-18" // metricDate (2026-07-08) minus 20 days
	var found14d, found30d bool
	for _, row := range snap.AccountMetrics {
		if analysisdata.FormatDate(row.MetricDate.Time) == outsideDate {
			found14d = true
		}
	}
	for _, row := range snap30.AccountMetrics {
		if analysisdata.FormatDate(row.MetricDate.Time) == outsideDate {
			found30d = true
		}
	}
	if found14d {
		t.Fatalf("legacy 14d Load unexpectedly included row at %s (outside its window)", outsideDate)
	}
	if !found30d {
		t.Fatalf("30d LoadWithOptions should include row at %s (inside its window)", outsideDate)
	}
	// Windows (Current7D/Previous7D rolling window helpers) stay the legacy
	// 14d-derived values regardless of HistoryDays.
	if snap30.Windows.CurrentDay != snap.Windows.CurrentDay || snap30.Windows.Last14d != snap.Windows.Last14d {
		t.Fatalf("Windows should be identical between Load and LoadWithOptions(30d): %+v vs %+v", snap.Windows, snap30.Windows)
	}
}

func createSeller(t *testing.T, pool *pgxpool.Pool, label string) int64 {
	t.Helper()
	ctx := context.Background()
	var userID int64
	email := fmt.Sprintf("%s-%d@test.local", label, time.Now().UnixNano())
	if err := pool.QueryRow(ctx, `
INSERT INTO users (email, password_hash)
VALUES ($1, 'hash')
RETURNING id`, email).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var sellerID int64
	if err := pool.QueryRow(ctx, `
INSERT INTO seller_accounts (user_id, name, status)
VALUES ($1, $2, 'active')
RETURNING id`, userID, label).Scan(&sellerID); err != nil {
		t.Fatalf("insert seller: %v", err)
	}
	return sellerID
}

func seedSellerWindow(t *testing.T, pool *pgxpool.Pool, sellerID int64, metricDate time.Time, withAds bool) {
	t.Helper()
	ctx := context.Background()
	asOf := analysisdata.DateOnlyUTC(metricDate)
	sku := int64(801_000_000_000 + sellerID)
	productID := int64(900_000_000_000 + sellerID)
	campaignID := int64(700_000_000_000 + sellerID)

	if _, err := pool.Exec(ctx, `
INSERT INTO products (seller_account_id, ozon_product_id, offer_id, sku, name, status)
VALUES ($1, $2, $3, $4, $5, 'active')`,
		sellerID, productID, fmt.Sprintf("offer-%d", sellerID), sku, fmt.Sprintf("Product %d", sellerID),
	); err != nil {
		t.Fatalf("insert product: %v", err)
	}

	for i := 0; i < 14; i++ {
		d := asOf.AddDate(0, 0, -13+i)
		if _, err := pool.Exec(ctx, `
INSERT INTO daily_account_metrics (
  seller_account_id, metric_date, revenue, orders_count, returns_count, cancel_count, units_sold
) VALUES ($1, $2, 1000, 10, 0, 0, 10)
ON CONFLICT (seller_account_id, metric_date) DO UPDATE
SET revenue = EXCLUDED.revenue, orders_count = EXCLUDED.orders_count`,
			sellerID, d,
		); err != nil {
			t.Fatalf("account metrics: %v", err)
		}
		if _, err := pool.Exec(ctx, `
INSERT INTO daily_sku_metrics (
  seller_account_id, metric_date, ozon_product_id, offer_id, sku, product_name,
  revenue, orders_count, stock_available, units_sold
) VALUES ($1, $2, $3, $4, $5, $6, 800, 8, 50, 8)
ON CONFLICT (seller_account_id, metric_date, ozon_product_id) DO UPDATE
SET revenue = EXCLUDED.revenue, orders_count = EXCLUDED.orders_count`,
			sellerID, d, productID, fmt.Sprintf("offer-%d", sellerID), sku, fmt.Sprintf("Product %d", sellerID),
		); err != nil {
			t.Fatalf("sku metrics: %v", err)
		}
		if _, err := pool.Exec(ctx, `
INSERT INTO product_analytics_daily (
  seller_account_id, metric_date, scope_type, scope_id,
  impressions, product_views, add_to_cart, orders_count, completed_orders, source_status
) VALUES ($1, $2, 'account', '', 1000, 200, 20, 8, 7, 'ok')
ON CONFLICT (seller_account_id, metric_date, scope_type, scope_id) DO UPDATE
SET impressions = EXCLUDED.impressions`,
			sellerID, d,
		); err != nil {
			t.Fatalf("funnel: %v", err)
		}
	}

	if _, err := pool.Exec(ctx, `
INSERT INTO stock_analytics_snapshots (
  seller_account_id, snapshot_date, sku, offer_id, product_name, warehouse_id, warehouse_name,
  available_stock_count, excess_stock_count, cluster_id
) VALUES ($1, $2, $3, $4, $5, 1, 'WH-1', 50, 0, 1)
ON CONFLICT (seller_account_id, snapshot_date, sku, warehouse_id, cluster_id) DO UPDATE
SET available_stock_count = EXCLUDED.available_stock_count`,
		sellerID, asOf, sku, fmt.Sprintf("offer-%d", sellerID), fmt.Sprintf("Product %d", sellerID),
	); err != nil {
		t.Fatalf("stock: %v", err)
	}

	if _, err := pool.Exec(ctx, `
INSERT INTO description_categories (description_category_id, category_name)
VALUES (991001, 'AnalysisData Category')
ON CONFLICT (description_category_id) DO NOTHING`); err != nil {
		t.Fatalf("category: %v", err)
	}

	if !withAds {
		return
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO ad_campaigns (seller_account_id, campaign_external_id, campaign_name, status)
VALUES ($1, $2, $3, 'CAMPAIGN_STATE_RUNNING')
ON CONFLICT (seller_account_id, campaign_external_id) DO UPDATE
SET campaign_name = EXCLUDED.campaign_name`,
		sellerID, campaignID, fmt.Sprintf("Campaign %d", sellerID),
	); err != nil {
		t.Fatalf("campaign: %v", err)
	}
	for i := 0; i < 14; i++ {
		d := asOf.AddDate(0, 0, -13+i)
		if _, err := pool.Exec(ctx, `
INSERT INTO ad_metrics_daily (
  seller_account_id, campaign_external_id, metric_date,
  impressions, clicks, spend, orders_count, revenue, campaign_name
) VALUES ($1, $2, $3, 100, 10, 50, 1, 200, $4)
ON CONFLICT (seller_account_id, campaign_external_id, metric_date) DO UPDATE
SET spend = EXCLUDED.spend`,
			sellerID, campaignID, d, fmt.Sprintf("Campaign %d", sellerID),
		); err != nil {
			t.Fatalf("ad metrics: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO ad_campaign_skus (
  seller_account_id, campaign_external_id, ozon_product_id, offer_id, sku, is_active
) VALUES ($1, $2, $3, $4, $5, TRUE)`,
		sellerID, campaignID, productID, fmt.Sprintf("offer-%d", sellerID), sku,
	); err != nil {
		t.Logf("campaign sku seed skipped: %v", err)
	}
}

func seedOutOfRangeRows(t *testing.T, pool *pgxpool.Pool, sellerID int64, metricDate time.Time) {
	t.Helper()
	ctx := context.Background()
	outside := analysisdata.DateOnlyUTC(metricDate).AddDate(0, 0, -20)
	if _, err := pool.Exec(ctx, `
INSERT INTO daily_account_metrics (
  seller_account_id, metric_date, revenue, orders_count, returns_count, cancel_count, units_sold
) VALUES ($1, $2, 99999, 999, 0, 0, 999)
ON CONFLICT (seller_account_id, metric_date) DO UPDATE
SET revenue = EXCLUDED.revenue, orders_count = EXCLUDED.orders_count`,
		sellerID, outside,
	); err != nil {
		t.Fatalf("out-of-range seed: %v", err)
	}
	_ = pgtype.Date{}
}
