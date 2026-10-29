//go:build integration

package aisummaries

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testAISummaryDSN(t *testing.T) string {
	t.Helper()
	v := os.Getenv("TEST_DATABASE_URL")
	if v == "" {
		t.Skip("set TEST_DATABASE_URL to run AI summary SQL integration tests")
		return ""
	}
	testutil.RequireSafeTestDatabaseURL(t, v)
	return v
}

func TestListDescriptionCategoryRollupIncludesStockWithoutDailySkuMetrics(t *testing.T) {
	dsn := testAISummaryDSN(t)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("db: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("ping: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := dbgen.New(tx)

	email := fmt.Sprintf("ai_sum_sql_%d@test.local", time.Now().UnixNano())
	hash, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatal(err)
	}
	u, err := q.CreateUser(ctx, dbgen.CreateUserParams{Email: email, PasswordHash: hash, Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	sa, err := q.CreateSellerAccount(ctx, dbgen.CreateSellerAccountParams{UserID: u.ID, Name: "AI Sum SQL", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	sellerID := sa.ID

	catID := int64(9_000_001 + (time.Now().UnixNano() % 1_000_000))
	if err := q.UpsertDescriptionCategory(ctx, dbgen.UpsertDescriptionCategoryParams{
		DescriptionCategoryID: catID,
		ParentID:              pgtype.Int8{},
		CategoryName:          "itest-cat",
		Path:                  pgtype.Text{String: "/itest", Valid: true},
	}); err != nil {
		t.Fatal(err)
	}

	ozonPID := int64(8_000_001 + (time.Now().UnixNano() % 1_000_000))
	skuVal := int64(7_000_001 + (time.Now().UnixNano() % 1_000_000))
	if _, err := q.UpsertProduct(ctx, dbgen.UpsertProductParams{
		SellerAccountID:       sellerID,
		OzonProductID:         ozonPID,
		OfferID:               pgtype.Text{String: "off-itest", Valid: true},
		Sku:                   pgtype.Int8{Int64: skuVal, Valid: true},
		Name:                  "itest product",
		Status:                pgtype.Text{String: "active", Valid: true},
		ReferencePrice:        pgtype.Numeric{},
		OldPrice:              pgtype.Numeric{},
		OzonMinPrice:          pgtype.Numeric{},
		DescriptionCategoryID: pgtype.Int8{Int64: catID, Valid: true},
		IsArchived:            false,
		RawAttributes:         []byte(`{}`),
		PrimaryImageUrl:       pgtype.Text{},
		ImageUrls:             []byte(`[]`),
		SourceUpdatedAt:       pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}); err != nil {
		t.Fatal(err)
	}

	metricDate := pgtype.Date{Time: time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC), Valid: true}
	_, err = tx.Exec(ctx, `INSERT INTO stocks (seller_account_id, product_external_id, warehouse_external_id, quantity_available, snapshot_at)
		VALUES ($1, $2, 'w-itest', 14, NOW())`, sellerID, fmt.Sprintf("%d", ozonPID))
	if err != nil {
		t.Fatal(err)
	}

	rows, err := q.ListDescriptionCategoryRollupForAISummary(ctx, dbgen.ListDescriptionCategoryRollupForAISummaryParams{
		SellerAccountID: sellerID,
		MetricDate:      metricDate,
	})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range rows {
		if r.DescriptionCategoryID.Valid && r.DescriptionCategoryID.Int64 == catID {
			found = true
			if r.ProductsWithStockCount < 1 {
				t.Fatalf("expected stock signal, got %+v", r)
			}
		}
	}
	if !found {
		t.Fatal("expected category in rollup when stock exists without DSM row")
	}

	skus, err := q.ListProductSKUMetricsForCategoryAndDate(ctx, dbgen.ListProductSKUMetricsForCategoryAndDateParams{
		SellerAccountID:       sellerID,
		DescriptionCategoryID: pgtype.Int8{Int64: catID, Valid: true},
		MetricDate:            metricDate,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(skus) != 1 {
		t.Fatalf("expected 1 sku row, got %d", len(skus))
	}
	if skus[0].StockAvailable < 1 || skus[0].OrdersCount != 0 {
		t.Fatalf("unexpected sku row: %+v", skus[0])
	}
}
