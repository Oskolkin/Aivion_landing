//go:build integration

package devseed

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testAdsDSN(t *testing.T) string {
	t.Helper()
	v := os.Getenv("TEST_DATABASE_URL")
	if v == "" {
		t.Skip("set TEST_DATABASE_URL to run ads integration tests")
		return ""
	}
	testutil.RequireSafeTestDatabaseURL(t, v)
	return v
}

func openAdsTestPool(t *testing.T, dsn string) *pgxpool.Pool {
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

func TestMVPAdsIntegration(t *testing.T) {
	dsn := testAdsDSN(t)
	pool := openAdsTestPool(t, dsn)
	ctx := context.Background()

	email := fmt.Sprintf("mvp_ads_%d@test.local", time.Now().UnixNano())
	hash, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatal(err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)

	u, err := q.CreateUser(ctx, dbgen.CreateUserParams{Email: email, PasswordHash: hash, Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	sa, err := q.CreateSellerAccount(ctx, dbgen.CreateSellerAccountParams{UserID: u.ID, Name: "Ads IT", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	sellerID := sa.ID
	if err := cleanupMVPSellerData(ctx, tx, sellerID); err != nil {
		t.Fatal(err)
	}

	anchor := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	opts := MVPSeedOptions{
		AnchorDate:     anchor,
		Days:           90,
		ProductsTarget: 80,
		Seed:           20260514,
		Reset:          false,
	}
	if _, err := SeedMVPCommerce(ctx, q, sellerID, opts); err != nil {
		t.Fatal(err)
	}
	if _, err := SeedMVPAds(ctx, q, sellerID, opts); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	var cc int64
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM ad_campaigns WHERE seller_account_id = $1`, sellerID).Scan(&cc); err != nil {
		t.Fatal(err)
	}
	if cc < 8 {
		t.Fatalf("campaigns %d want >= 8", cc)
	}

	var badROAS int64
	if err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM (
  SELECT m.campaign_external_id,
    SUM(m.revenue::float8) / NULLIF(SUM(m.spend::float8), 0) AS roas
  FROM ad_metrics_daily m
  WHERE m.seller_account_id = $1
  GROUP BY m.campaign_external_id
) t WHERE t.roas < 1`, sellerID).Scan(&badROAS); err != nil {
		t.Fatal(err)
	}
	if badROAS < 1 {
		t.Fatal("expected at least one campaign with aggregate ROAS < 1")
	}

	var spendNoResult int64
	if err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM (
  SELECT campaign_external_id
  FROM ad_metrics_daily
  WHERE seller_account_id = $1
  GROUP BY campaign_external_id
  HAVING SUM(orders_count) = 0
     AND COALESCE(SUM(revenue::numeric), 0) = 0
     AND SUM(spend::numeric) > 0
     AND SUM(clicks) > 0
) x`, sellerID).Scan(&spendNoResult); err != nil {
		t.Fatal(err)
	}
	if spendNoResult < 1 {
		t.Fatal("expected spend-without-result campaign (clicks>0, orders=0, revenue=0, spend>0)")
	}

	var badClicks int64
	if err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM ad_metrics_daily
WHERE seller_account_id = $1 AND clicks > impressions`, sellerID).Scan(&badClicks); err != nil {
		t.Fatal(err)
	}
	if badClicks != 0 {
		t.Fatalf("clicks > impressions rows: %d", badClicks)
	}

	var badOrders int64
	if err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM ad_metrics_daily
WHERE seller_account_id = $1 AND orders_count > clicks`, sellerID).Scan(&badOrders); err != nil {
		t.Fatal(err)
	}
	if badOrders != 0 {
		t.Fatalf("orders > clicks rows: %d", badOrders)
	}

	var lowStockAdv int64
	if err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM ad_campaign_skus acs
JOIN products p ON p.seller_account_id = acs.seller_account_id AND p.ozon_product_id = acs.ozon_product_id
JOIN (
  SELECT product_external_id, SUM(quantity_available)::bigint AS s
  FROM stocks WHERE seller_account_id = $1
  GROUP BY product_external_id
) st ON st.product_external_id = acs.ozon_product_id::text
WHERE acs.seller_account_id = $1 AND acs.is_active = TRUE AND st.s <= 8`, sellerID).Scan(&lowStockAdv); err != nil {
		t.Fatal(err)
	}
	if lowStockAdv < 1 {
		t.Fatal("expected active ad link to low-stock aggregate (<=8 units)")
	}

	var skuMismatch int64
	if err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM ad_campaign_skus acs
JOIN products p ON p.seller_account_id = acs.seller_account_id AND p.ozon_product_id = acs.ozon_product_id
WHERE acs.seller_account_id = $1
  AND acs.sku IS DISTINCT FROM p.sku`, sellerID).Scan(&skuMismatch); err != nil {
		t.Fatal(err)
	}
	if skuMismatch != 0 {
		t.Fatalf("ad sku mismatch with products: %d", skuMismatch)
	}

	var pausedWithMetrics int64
	if err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM ad_campaigns c
WHERE c.seller_account_id = $1 AND c.status = 'paused'
  AND EXISTS (
    SELECT 1 FROM ad_metrics_daily m
    WHERE m.seller_account_id = c.seller_account_id
      AND m.campaign_external_id = c.campaign_external_id
  )`, sellerID).Scan(&pausedWithMetrics); err != nil {
		t.Fatal(err)
	}
	if pausedWithMetrics < 1 {
		t.Fatal("expected at least one paused campaign with historical ad_metrics_daily rows")
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, u.ID)
	})
}
