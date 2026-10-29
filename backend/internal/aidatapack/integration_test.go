//go:build integration

package aidatapack_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/devseed"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testAIDataPackDSN(t *testing.T) string {
	t.Helper()
	v := os.Getenv("TEST_DATABASE_URL")
	if v == "" {
		t.Skip("set TEST_DATABASE_URL to run aidatapack integration tests")
		return ""
	}
	testutil.RequireSafeTestDatabaseURL(t, v)
	return v
}

func openAIDataPackTestPool(t *testing.T, dsn string) *pgxpool.Pool {
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

func createIntegrationSellerWithProducts(t *testing.T, pool *pgxpool.Pool, productCount int) int64 {
	t.Helper()
	ctx := context.Background()
	var userID int64
	if err := pool.QueryRow(ctx, `
INSERT INTO users (email, password_hash)
VALUES ($1, 'hash')
RETURNING id`, fmt.Sprintf("aidatapack-%d@test.local", time.Now().UnixNano())).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var sellerID int64
	if err := pool.QueryRow(ctx, `
INSERT INTO seller_accounts (user_id, name, status)
VALUES ($1, 'aidatapack-integration', 'active')
RETURNING id`, userID).Scan(&sellerID); err != nil {
		t.Fatalf("insert seller: %v", err)
	}
	for i := 1; i <= productCount; i++ {
		sku := int64(901_000_100_000 + int64(i))
		productID := int64(100_000 + i)
		if _, err := pool.Exec(ctx, `
INSERT INTO products (seller_account_id, ozon_product_id, offer_id, sku, name, status)
VALUES ($1, $2, $3, $4, $5, 'active')`,
			sellerID, productID, fmt.Sprintf("offer-%d", i), sku, fmt.Sprintf("Product %d", i),
		); err != nil {
			t.Fatalf("insert product %d: %v", i, err)
		}
	}
	return sellerID
}

func TestAIDataPackIntegration_AfterOzonEnrichmentSeed(t *testing.T) {
	t.Setenv("ALLOW_DEV_SEED", "1")
	t.Setenv("APP_ENV", "local")

	pool := openAIDataPackTestPool(t, testAIDataPackDSN(t))
	ctx := context.Background()
	metricDate := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)

	// Isolated seller fixture — do not depend on mutable seller_id=1 / shared env seed.
	sellerID := createIntegrationSellerWithProducts(t, pool, 100)
	seedIntegrationDailyMetrics(t, pool, sellerID, metricDate, 100)
	seed, err := devseed.SeedOzonEnrichment(ctx, pool, devseed.OzonEnrichmentSeedOptions{
		SellerAccountID: sellerID,
		MetricDate:      &metricDate,
		Scenario:        devseed.OzonEnrichmentScenarioRich,
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	svc := aidatapack.NewService(pool)
	pack, err := svc.Build(ctx, sellerID, metricDate)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if pack.SchemaVersion != aidatapack.SchemaVersion {
		t.Fatalf("schema=%s", pack.SchemaVersion)
	}
	if len(pack.Tables.SKURollups) != seed.SKUsUsed {
		t.Fatalf("sku_rollups=%d want %d (seed.SKUsUsed)", len(pack.Tables.SKURollups), seed.SKUsUsed)
	}
	if int32(len(pack.Tables.StockPositions)) != seed.StockAnalyticsCount {
		t.Fatalf("stock_positions=%d want %d", len(pack.Tables.StockPositions), seed.StockAnalyticsCount)
	}
	if int32(len(pack.Tables.CampaignRollups)) != seed.CampaignCount {
		t.Fatalf("campaign_rollups=%d want %d", len(pack.Tables.CampaignRollups), seed.CampaignCount)
	}
	if len(pack.Tables.CampaignProductLinks) == 0 {
		t.Fatal("expected campaign_product_links > 0")
	}

	grades := map[string]bool{}
	for _, row := range pack.Tables.StockPositions {
		if row.TurnoverGrade != "" {
			grades[row.TurnoverGrade] = true
		}
	}
	for _, want := range []string{"DEFICIT", "WAITING_FOR_SUPPLY", "SURPLUS", "NO_SALES", "ACTUAL"} {
		if !grades[want] {
			t.Fatalf("missing turnover grade %s in pack", want)
		}
	}

	var spendNoOrders, highDRR, healthyROAS, zeroActivity bool
	for _, c := range pack.Tables.CampaignRollups {
		check := c.Last7d
		if check.Spend > 0 && check.Orders == 0 {
			spendNoOrders = true
		}
		if check.DRRPct != nil && *check.DRRPct > 50 {
			highDRR = true
		}
		if check.ROAS != nil && *check.ROAS >= 1 {
			healthyROAS = true
		}
		if check.Spend == 0 && check.Impressions == 0 {
			zeroActivity = true
		}
	}
	if !spendNoOrders || !highDRR || !healthyROAS {
		t.Fatalf("ad scenarios: spend_no_orders=%v high_drr=%v healthy=%v zero=%v",
			spendNoOrders, highDRR, healthyROAS, zeroActivity)
	}

	if err := aidatapack.ValidateDataPack(pack); err != nil {
		t.Fatalf("validate: %v", err)
	}

	report := aidatapack.AuditDataPack(pack)
	if report.Shape.FactRegistryType != "object" {
		t.Fatalf("fact_registry_type=%s", report.Shape.FactRegistryType)
	}
	if report.DuplicateCampaignProductLinks != 0 {
		t.Fatalf("duplicate links=%d", report.DuplicateCampaignProductLinks)
	}
	if report.CampaignScenarios.InactiveByStateCount == 0 {
		t.Fatal("expected inactive_by_state_count > 0 for rich enrichment seed campaigns")
	}
	// Zero-activity campaigns may be omitted from rollups when no daily metrics exist;
	// rich seed still marks inactive campaigns via campaign state audit above.
	_ = zeroActivity

	enrich, _ := pack.DataQuality.Sources["product_enrichment"].(map[string]any)
	if enrich == nil || enrich["source"] != "dev_seed" {
		t.Fatalf("product_enrichment source=%v", enrich)
	}
	if pack.Tables.FinanceRollup.DataStatus != "not_available" {
		t.Fatal("finance should be not_available")
	}

	raw, err := json.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("json roundtrip: %v", err)
	}
}

func seedIntegrationDailyMetrics(t *testing.T, pool *pgxpool.Pool, sellerID int64, metricDate time.Time, productCount int) {
	t.Helper()
	ctx := context.Background()
	for i := 1; i <= productCount; i++ {
		sku := int64(901_000_100_000 + int64(i))
		productID := int64(100_000 + i)
		if _, err := pool.Exec(ctx, `
INSERT INTO daily_sku_metrics (
  seller_account_id, metric_date, ozon_product_id, offer_id, sku, product_name,
  revenue, orders_count, stock_available
) VALUES ($1, $2::date, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT DO NOTHING`,
			sellerID, metricDate.Format("2006-01-02"), productID,
			fmt.Sprintf("offer-%d", i), sku, fmt.Sprintf("Product %d", i),
			float64(1000*i), i, 10*i,
		); err != nil {
			// Fallback if conflict target differs — try without ON CONFLICT.
			if _, err2 := pool.Exec(ctx, `
INSERT INTO daily_sku_metrics (
  seller_account_id, metric_date, ozon_product_id, offer_id, sku, product_name,
  revenue, orders_count, stock_available
) VALUES ($1, $2::date, $3, $4, $5, $6, $7, $8, $9)`,
				sellerID, metricDate.Format("2006-01-02"), productID,
				fmt.Sprintf("offer-%d", i), sku, fmt.Sprintf("Product %d", i),
				float64(1000*i), i, 10*i,
			); err2 != nil {
				t.Fatalf("insert daily metric %d: %v (also %v)", i, err, err2)
			}
		}
	}
}

func TestAIDataPackIntegration_EnrichmentFailedStructuredLimitation(t *testing.T) {
	t.Setenv("ALLOW_DEV_SEED", "1")
	t.Setenv("APP_ENV", "local")

	pool := openAIDataPackTestPool(t, testAIDataPackDSN(t))
	ctx := context.Background()
	metricDate := time.Date(2026, 6, 26, 0, 0, 0, 0, time.UTC)

	sellerID := createIntegrationSellerWithProducts(t, pool, 10)
	_, err := devseed.SeedOzonEnrichment(ctx, pool, devseed.OzonEnrichmentSeedOptions{
		SellerAccountID: sellerID,
		MetricDate:      &metricDate,
		Scenario:        devseed.OzonEnrichmentScenarioRich,
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	errorsJSON := `[{"source":"seller_api","message":"credentials missing"}]`
	if _, err := pool.Exec(ctx, `
UPDATE ozon_enrichment_sync_runs
SET status = 'failed',
    errors_json = $1::jsonb,
    performance_api_available = false
WHERE seller_account_id = $2
  AND metric_date = $3::date`,
		errorsJSON, sellerID, metricDate,
	); err != nil {
		t.Fatalf("mark enrichment failed: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM ozon_product_info_snapshots WHERE seller_account_id=$1`, sellerID); err != nil {
		t.Fatalf("delete product snapshots: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM stock_analytics_snapshots WHERE seller_account_id=$1`, sellerID); err != nil {
		t.Fatalf("delete stock snapshots: %v", err)
	}

	svc := aidatapack.NewService(pool)
	pack, err := svc.Build(ctx, sellerID, metricDate)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(pack.DataQuality.StructuredLimitations) != 1 {
		t.Fatalf("structured_limitations=%d want 1", len(pack.DataQuality.StructuredLimitations))
	}
	lim := pack.DataQuality.StructuredLimitations[0]
	if lim.Code != "ozon_enrichment_unavailable" || lim.Severity != "high" {
		t.Fatalf("limitation=%+v", lim)
	}
	if len(lim.Scope) == 0 {
		t.Fatal("expected non-empty scope")
	}
}
