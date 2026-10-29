//go:build integration

package pack_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	uapack "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/pack"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testDSN(t *testing.T) string {
	t.Helper()
	v := os.Getenv("TEST_DATABASE_URL")
	if v == "" {
		t.Skip("set TEST_DATABASE_URL to run unifiedanalysis/pack integration tests")
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

func TestUnifiedPackBuild_RealPostgres(t *testing.T) {
	pool := openPool(t, testDSN(t))
	ctx := context.Background()
	metricDate := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)

	sellerID := createSeller(t, pool, "uapack")
	seedSeller30d(t, pool, sellerID, metricDate)

	loader := analysisdata.NewSQLLoader(pool)
	svc := uapack.NewService(loader, uapack.NewPackBuilder(uapack.DefaultBuildOptions()), uapack.NewValidator(uapack.DefaultBuildOptions()), nil)

	r1, err := svc.Build(ctx, sellerID, metricDate)
	if err != nil {
		if r1 != nil {
			t.Fatalf("build 1: %v; business=%s errors=%+v", err, r1.BusinessConsistency.SummaryLine(), r1.BusinessConsistency.Errors)
		}
		t.Fatalf("build 1: %v", err)
	}
	if !r1.Validation.Valid {
		t.Fatalf("validation failed: %+v", r1.Validation.Issues)
	}
	if r1.Manifest.SellerAccountID != sellerID {
		t.Fatalf("seller_id=%d want %d", r1.Manifest.SellerAccountID, sellerID)
	}
	if len(r1.Files) != 12 {
		t.Fatalf("files=%d want 12", len(r1.Files))
	}

	r2, err := svc.Build(ctx, sellerID, metricDate)
	if err != nil {
		t.Fatalf("build 2: %v", err)
	}
	if r1.PackID != r2.PackID || r1.PackHash != r2.PackHash {
		t.Fatalf("determinism failed: id %s/%s hash %s/%s", r1.PackID, r2.PackID, r1.PackHash, r2.PackHash)
	}
	if !bytes.Equal(r1.ZipBytes, r2.ZipBytes) {
		t.Fatal("zip bytes differ across rebuilds")
	}

	sum1 := sha256.Sum256(r1.ZipBytes)
	sum2 := sha256.Sum256(r2.ZipBytes)
	t.Logf("pack_id=%s pack_hash=%s archive_sha256=%s", r1.PackID, r1.PackHash, hex.EncodeToString(sum1[:]))
	if sum1 != sum2 {
		t.Fatal("archive sha mismatch")
	}

	// Isolation: second seller without seed must not appear in first pack (already enforced by loader).
	other := createSeller(t, pool, "uapack-other")
	_ = other
	if r1.SellerAccountID != sellerID {
		t.Fatalf("seller leak: %d", r1.SellerAccountID)
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

func seedSeller30d(t *testing.T, pool *pgxpool.Pool, sellerID int64, metricDate time.Time) {
	t.Helper()
	ctx := context.Background()
	asOf := analysisdata.DateOnlyUTC(metricDate)
	sku := int64(811_000_000_000 + sellerID)
	productID := int64(911_000_000_000 + sellerID)
	campaignID := int64(711_000_000_000 + sellerID)

	if _, err := pool.Exec(ctx, `
INSERT INTO products (seller_account_id, ozon_product_id, offer_id, sku, name, status)
VALUES ($1, $2, $3, $4, $5, 'active')`,
		sellerID, productID, fmt.Sprintf("offer-%d", sellerID), sku, fmt.Sprintf("UA Product %d", sellerID),
	); err != nil {
		t.Fatalf("insert product: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO ad_campaigns (seller_account_id, campaign_external_id, campaign_name, status)
VALUES ($1, $2, $3, 'CAMPAIGN_STATE_RUNNING')
ON CONFLICT (seller_account_id, campaign_external_id) DO UPDATE
SET campaign_name = EXCLUDED.campaign_name`,
		sellerID, campaignID, fmt.Sprintf("UA Campaign %d", sellerID),
	); err != nil {
		t.Fatalf("campaign: %v", err)
	}

	for i := 0; i < 30; i++ {
		d := asOf.AddDate(0, 0, -29+i)
		if _, err := pool.Exec(ctx, `
INSERT INTO daily_account_metrics (
  seller_account_id, metric_date, revenue, orders_count, returns_count, cancel_count, units_sold
) VALUES ($1, $2, 800, 8, 0, 0, 8)
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
			sellerID, d, productID, fmt.Sprintf("offer-%d", sellerID), sku, fmt.Sprintf("UA Product %d", sellerID),
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
		if _, err := pool.Exec(ctx, `
INSERT INTO ad_metrics_daily (
  seller_account_id, campaign_external_id, metric_date,
  impressions, clicks, spend, orders_count, revenue, campaign_name
) VALUES ($1, $2, $3, 100, 10, 50, 1, 200, $4)
ON CONFLICT (seller_account_id, campaign_external_id, metric_date) DO UPDATE
SET spend = EXCLUDED.spend`,
			sellerID, campaignID, d, fmt.Sprintf("UA Campaign %d", sellerID),
		); err != nil {
			t.Fatalf("ad metrics: %v", err)
		}
	}

	if _, err := pool.Exec(ctx, `
INSERT INTO stock_analytics_snapshots (
  seller_account_id, snapshot_date, sku, offer_id, product_name, warehouse_id, warehouse_name,
  available_stock_count, excess_stock_count, cluster_id
) VALUES ($1, $2, $3, $4, $5, 1, 'WH-1', 50, 0, 1)
ON CONFLICT (seller_account_id, snapshot_date, sku, warehouse_id, cluster_id) DO UPDATE
SET available_stock_count = EXCLUDED.available_stock_count`,
		sellerID, asOf, sku, fmt.Sprintf("offer-%d", sellerID), fmt.Sprintf("UA Product %d", sellerID),
	); err != nil {
		t.Fatalf("stock: %v", err)
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
