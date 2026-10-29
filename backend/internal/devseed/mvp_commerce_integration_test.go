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

func testCommerceDSN(t *testing.T) string {
	t.Helper()
	v := os.Getenv("TEST_DATABASE_URL")
	if v == "" {
		t.Skip("set TEST_DATABASE_URL to run commerce integration tests")
		return ""
	}
	testutil.RequireSafeTestDatabaseURL(t, v)
	return v
}

func openCommerceTestPool(t *testing.T, dsn string) *pgxpool.Pool {
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

func TestMVPCommerceIntegration(t *testing.T) {
	dsn := testCommerceDSN(t)
	pool := openCommerceTestPool(t, dsn)
	ctx := context.Background()

	email := fmt.Sprintf("mvp_commerce_%d@test.local", time.Now().UnixNano())
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
	sa, err := q.CreateSellerAccount(ctx, dbgen.CreateSellerAccountParams{UserID: u.ID, Name: "Commerce IT", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	sellerID := sa.ID

	if err := cleanupMVPSellerData(ctx, tx, sellerID); err != nil {
		t.Fatal(err)
	}

	anchor := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
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
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	if err := RebuildMVPAnalytics(ctx, pool, sellerID, anchor, 90); err != nil {
		t.Fatal(err)
	}

	var pc int64
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM products WHERE seller_account_id = $1`, sellerID).Scan(&pc); err != nil {
		t.Fatal(err)
	}
	if pc < 80 {
		t.Fatalf("products count %d want >= 80", pc)
	}

	var segCount int64
	if err := pool.QueryRow(ctx, `
SELECT COUNT(DISTINCT raw_attributes->>'segment') FROM products WHERE seller_account_id = $1`, sellerID).Scan(&segCount); err != nil {
		t.Fatal(err)
	}
	if segCount < 8 {
		t.Fatalf("distinct segments %d want 8", segCount)
	}

	var last7Orders int64
	if err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM orders
WHERE seller_account_id = $1
  AND created_at_source::date > $2::date - interval '7 days'
  AND created_at_source::date <= $2::date`, sellerID, anchor).Scan(&last7Orders); err != nil {
		t.Fatal(err)
	}
	if last7Orders < 1 {
		t.Fatalf("last 7d orders %d want >= 1", last7Orders)
	}

	var last7Sales int64
	if err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM sales
WHERE seller_account_id = $1
  AND sale_date::date > $2::date - interval '7 days'
  AND sale_date::date <= $2::date`, sellerID, anchor).Scan(&last7Sales); err != nil {
		t.Fatal(err)
	}
	if last7Sales < 1 {
		t.Fatalf("last 7d sales %d want >= 1", last7Sales)
	}

	var lowStockWH int64
	if err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM stocks
WHERE seller_account_id = $1 AND warehouse_external_id IN ('WH-MOSCOW','WH-SPB','WH-KAZAN')
  AND quantity_available BETWEEN 0 AND 5`, sellerID).Scan(&lowStockWH); err != nil {
		t.Fatal(err)
	}
	if lowStockWH < 1 {
		t.Fatal("expected some low_stock warehouse rows")
	}

	var overstock int64
	if err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM (
  SELECT product_external_id, SUM(quantity_available)::bigint s FROM stocks
  WHERE seller_account_id = $1 AND warehouse_external_id IN ('WH-MOSCOW','WH-SPB','WH-KAZAN')
  GROUP BY product_external_id
) t WHERE t.s >= 150`, sellerID).Scan(&overstock); err != nil {
		t.Fatal(err)
	}
	if overstock < 1 {
		t.Fatal("expected at least one product with total available >= 150 (overstock)")
	}

	var leaderPos, totalPos float64
	if err := pool.QueryRow(ctx, `
SELECT
  COALESCE(SUM(CASE WHEN raw_attributes->>'segment' = 'leaders' AND amount > 0 THEN amount::float8 END), 0),
  COALESCE(SUM(CASE WHEN amount > 0 THEN amount::float8 END), 0)
FROM sales WHERE seller_account_id = $1`, sellerID).Scan(&leaderPos, &totalPos); err != nil {
		t.Fatal(err)
	}
	if totalPos <= 0 {
		t.Fatal("no positive sales")
	}
	if r := leaderPos / totalPos; r < 0.45 {
		t.Fatalf("leaders share of positive revenue %.2f want >= 0.45", r)
	}

	var dam7 int64
	if err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM daily_account_metrics
WHERE seller_account_id = $1
  AND metric_date > $2::date - interval '7 days'
  AND metric_date <= $2::date`, sellerID, anchor).Scan(&dam7); err != nil {
		t.Fatal(err)
	}
	if dam7 < 1 {
		t.Fatalf("daily_account_metrics rows in last 7d: %d", dam7)
	}

	var dsm7 int64
	if err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM daily_sku_metrics
WHERE seller_account_id = $1
  AND metric_date > $2::date - interval '7 days'
  AND metric_date <= $2::date`, sellerID, anchor).Scan(&dsm7); err != nil {
		t.Fatal(err)
	}
	if dsm7 < 1 {
		t.Fatalf("daily_sku_metrics rows in last 7d: %d", dsm7)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, u.ID)
	})
}
