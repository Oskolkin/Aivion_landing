//go:build integration

package devseed

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testOzonEnrichmentDSN(t *testing.T) string {
	t.Helper()
	v := os.Getenv("TEST_DATABASE_URL")
	if v == "" {
		t.Skip("set TEST_DATABASE_URL to run ozon enrichment seed integration tests")
		return ""
	}
	testutil.RequireSafeTestDatabaseURL(t, v)
	return v
}

func TestOzonEnrichmentSeedIntegration_RichIdempotent(t *testing.T) {
	t.Setenv("ALLOW_DEV_SEED", "1")
	t.Setenv("APP_ENV", "local")

	pool := openAdEfficiencyTestPool(t, testOzonEnrichmentDSN(t))
	ctx := context.Background()
	sellerID := createOzonEnrichmentTestSeller(t, ctx, pool)
	metricDate := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	seedOzonEnrichmentFixtures(t, ctx, pool, sellerID, metricDate)

	opts := OzonEnrichmentSeedOptions{
		SellerAccountID: sellerID,
		MetricDate:      &metricDate,
		Scenario:        OzonEnrichmentScenarioRich,
	}
	first, err := SeedOzonEnrichment(ctx, pool, opts)
	if err != nil {
		t.Fatal(err)
	}
	if first.ProductInfoCount == 0 || first.StockAnalyticsCount == 0 {
		t.Fatalf("expected seeded rows, got %+v", first)
	}
	if first.Scope != OzonEnrichmentSeedScope {
		t.Fatalf("scope=%s", first.Scope)
	}

	second, err := SeedOzonEnrichment(ctx, pool, opts)
	if err != nil {
		t.Fatal(err)
	}
	if second.ProductInfoCount != first.ProductInfoCount {
		t.Fatalf("idempotent product info: first=%d second=%d", first.ProductInfoCount, second.ProductInfoCount)
	}
	if second.StockAnalyticsCount != first.StockAnalyticsCount {
		t.Fatalf("idempotent stock analytics: first=%d second=%d", first.StockAnalyticsCount, second.StockAnalyticsCount)
	}

	q := dbgen.New(pool)
	run, err := q.GetLatestOzonEnrichmentSyncRunBySellerAndDate(ctx, dbgen.GetLatestOzonEnrichmentSyncRunBySellerAndDateParams{
		SellerAccountID: sellerID,
		MetricDate:      pgDateUTC(metricDate),
	})
	if err != nil {
		t.Fatal(err)
	}
	if run.Scope != OzonEnrichmentSeedScope || run.Status != "completed" {
		t.Fatalf("run scope=%s status=%s", run.Scope, run.Status)
	}

	grades := map[string]int{}
	rows, err := q.ListStockAnalyticsSnapshotsBySellerAndDate(ctx, dbgen.ListStockAnalyticsSnapshotsBySellerAndDateParams{
		SellerAccountID: sellerID,
		SnapshotDate:    pgDateUTC(metricDate),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.TurnoverGrade.Valid {
			grades[row.TurnoverGrade.String]++
		}
	}
	for _, want := range []string{"DEFICIT", "WAITING_FOR_SUPPLY", "SURPLUS", "NO_SALES", "ACTUAL"} {
		if grades[want] == 0 {
			t.Fatalf("missing turnover grade %s in %v", want, grades)
		}
	}

	// available_stock_count must mirror the live stocks aggregate (not synthetic
	// stockAnalyticsValues counts). Rich seed may zero stocks for WAITING SKUs.
	stockByProduct, err := loadAvailableStockByProduct(ctx, q, sellerID)
	if err != nil {
		t.Fatal(err)
	}
	skuToProduct := map[int64]int64{}
	products, err := q.ListProductsBySellerAccountID(ctx, sellerID)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range products {
		if p.Sku.Valid {
			skuToProduct[p.Sku.Int64] = p.OzonProductID
		}
	}
	for _, row := range rows {
		productID, ok := skuToProduct[row.Sku]
		if !ok {
			t.Fatalf("sku %d missing product mapping", row.Sku)
		}
		want := stockByProduct[productID]
		if row.AvailableStockCount != want {
			t.Fatalf("sku %d available_stock_count=%d want %d (from stocks aggregate)", row.Sku, row.AvailableStockCount, want)
		}
	}

	assertRichAdMetricsScenarios(t, ctx, q, sellerID, metricDate)
	assertEnrichedAdCampaignSKUs(t, ctx, q, sellerID)
}

func TestOzonEnrichmentSeedIntegration_ReuseExistingAdvertising(t *testing.T) {
	t.Setenv("ALLOW_DEV_SEED", "1")
	t.Setenv("APP_ENV", "local")

	pool := openAdEfficiencyTestPool(t, testOzonEnrichmentDSN(t))
	ctx := context.Background()
	sellerID := createOzonEnrichmentTestSeller(t, ctx, pool)
	metricDate := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	seedOzonEnrichmentFixtures(t, ctx, pool, sellerID, metricDate)
	q := dbgen.New(pool)

	// reuse=true with no existing ad_metrics_daily must be a hard error: the
	// final seller-1 flow must never fall back to synthetic advertising.
	if _, err := SeedOzonEnrichment(ctx, pool, OzonEnrichmentSeedOptions{
		SellerAccountID:          sellerID,
		MetricDate:               &metricDate,
		Scenario:                 OzonEnrichmentScenarioRich,
		ReuseExistingAdvertising: true,
	}); err == nil {
		t.Fatal("expected error when reuse-existing-advertising is set but no ad_metrics_daily exist")
	}

	// Seed once without reuse to create real ad_metrics_daily rows.
	first, err := SeedOzonEnrichment(ctx, pool, OzonEnrichmentSeedOptions{
		SellerAccountID: sellerID,
		MetricDate:      &metricDate,
		Scenario:        OzonEnrichmentScenarioRich,
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.CampaignDailyStatsCount == 0 {
		t.Fatal("expected seeded ad_metrics_daily rows on non-reuse seed")
	}

	from := metricDate.AddDate(0, 0, -enrichmentStatsLookbackDays)
	rangeParams := dbgen.ListAdMetricsDailyBySellerAndDateRangeParams{
		SellerAccountID: sellerID,
		DateFrom:        pgDateUTC(from),
		DateTo:          pgDateUTC(metricDate),
	}
	before, err := q.ListAdMetricsDailyBySellerAndDateRange(ctx, rangeParams)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) == 0 {
		t.Fatal("expected existing ad_metrics_daily rows after non-reuse seed")
	}

	// Mutate one existing row to a sentinel value; reuse must not overwrite it.
	sentinel := before[0]
	if _, err := pool.Exec(ctx, `UPDATE ad_metrics_daily SET spend = 424242.42, impressions = 987654 WHERE id = $1`, sentinel.ID); err != nil {
		t.Fatal(err)
	}

	second, err := SeedOzonEnrichment(ctx, pool, OzonEnrichmentSeedOptions{
		SellerAccountID:          sellerID,
		MetricDate:               &metricDate,
		Scenario:                 OzonEnrichmentScenarioRich,
		ReuseExistingAdvertising: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if int(second.CampaignDailyStatsCount) != len(before) {
		t.Fatalf("reuse CampaignDailyStatsCount=%d want existing count %d", second.CampaignDailyStatsCount, len(before))
	}

	after, err := q.ListAdMetricsDailyBySellerAndDateRange(ctx, rangeParams)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("reuse changed ad_metrics_daily row count: before=%d after=%d", len(before), len(after))
	}
	var found bool
	for _, r := range after {
		if r.ID != sentinel.ID {
			continue
		}
		found = true
		if got := parseNumericFloat(r.Spend); got != 424242.42 {
			t.Fatalf("reuse overwrote ad_metrics_daily spend: got %v want sentinel 424242.42", got)
		}
		if r.Impressions != 987654 {
			t.Fatalf("reuse overwrote ad_metrics_daily impressions: got %d want 987654", r.Impressions)
		}
	}
	if !found {
		t.Fatal("sentinel ad_metrics_daily row missing after reuse seed")
	}
}

func assertEnrichedAdCampaignSKUs(t *testing.T, ctx context.Context, q *dbgen.Queries, sellerID int64) {
	t.Helper()
	links, err := q.ListAdCampaignSKUMappingsBySellerAccountID(ctx, sellerID)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) == 0 {
		t.Fatal("expected ad_campaign_skus rows after seed")
	}

	campaigns, err := q.ListAdCampaignsBySellerAccountID(ctx, sellerID)
	if err != nil {
		t.Fatal(err)
	}
	kindByCampaign := campaignKindByExternalID(campaigns)

	for _, link := range links {
		if !link.CapturedAt.Valid {
			t.Fatalf("campaign %d sku %v missing captured_at", link.CampaignExternalID, link.Sku)
		}
		var raw map[string]any
		if err := json.Unmarshal(link.RawAttributes, &raw); err != nil {
			t.Fatalf("campaign %d invalid raw_attributes: %v", link.CampaignExternalID, err)
		}
		if raw["source"] != DevOzonEnrichmentSeedSource {
			t.Fatalf("campaign %d sku %v source=%v", link.CampaignExternalID, link.Sku, raw["source"])
		}
		if link.IsActive && link.Sku.Valid {
			kind := kindByCampaign[link.CampaignExternalID]
			autoMarked := raw["bid_applicability"] == "not_applicable_auto_strategy"
			if kind != campaignKindAutoRunning && !autoMarked && !link.BidAmount.Valid {
				t.Fatalf("active manual campaign %d sku %d missing bid_amount", link.CampaignExternalID, link.Sku.Int64)
			}
		}
		if !link.ProductTitle.Valid || link.ProductTitle.String == "" {
			t.Fatalf("campaign %d sku %v missing product_title", link.CampaignExternalID, link.Sku)
		}
	}
}

func assertRichAdMetricsScenarios(t *testing.T, ctx context.Context, q *dbgen.Queries, sellerID int64, metricDate time.Time) {
	t.Helper()
	from := metricDate.AddDate(0, 0, -enrichmentStatsLookbackDays)
	rows, err := q.ListAdMetricsDailyBySellerAndDateRange(ctx, dbgen.ListAdMetricsDailyBySellerAndDateRangeParams{
		SellerAccountID: sellerID,
		DateFrom:        pgDateUTC(from),
		DateTo:          pgDateUTC(metricDate),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("expected ad_metrics_daily rows after seed")
	}

	type agg struct {
		spend   float64
		revenue float64
		orders  int64
	}
	byCampaign := map[int64]agg{}
	for _, row := range rows {
		if !row.Revenue.Valid || !row.Spend.Valid {
			t.Fatalf("campaign %d metric_date %v has NULL spend/revenue", row.CampaignExternalID, row.MetricDate)
		}
		a := byCampaign[row.CampaignExternalID]
		a.spend += parseNumericFloat(row.Spend)
		a.revenue += parseNumericFloat(row.Revenue)
		a.orders += int64(row.OrdersCount)
		byCampaign[row.CampaignExternalID] = a
	}

	var hasSpendNoOrders, hasHighDRR, hasHealthyROAS, hasZeroActivity bool
	for _, a := range byCampaign {
		switch {
		case a.spend > 0 && a.orders == 0 && a.revenue == 0:
			hasSpendNoOrders = true
		case a.spend > 0 && a.revenue > 0 && a.spend/a.revenue*100 > 100:
			hasHighDRR = true
		case a.spend > 0 && a.revenue > a.spend*3:
			hasHealthyROAS = true
		case a.spend == 0 && a.orders == 0 && a.revenue == 0:
			hasZeroActivity = true
		}
	}
	if !hasSpendNoOrders || !hasHighDRR || !hasHealthyROAS || !hasZeroActivity {
		t.Fatalf("missing ad metrics scenarios: spend_no_orders=%v high_drr=%v healthy=%v zero=%v campaigns=%v",
			hasSpendNoOrders, hasHighDRR, hasHealthyROAS, hasZeroActivity, byCampaign)
	}
}

func createOzonEnrichmentTestSeller(t *testing.T, ctx context.Context, pool *pgxpool.Pool) int64 {
	t.Helper()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	email := fmt.Sprintf("ozon_enrich_seed_%d@test.local", time.Now().UnixNano())
	hash, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatal(err)
	}
	user, err := q.CreateUser(ctx, dbgen.CreateUserParams{
		Email:        email,
		PasswordHash: hash,
		Status:       "active",
	})
	if err != nil {
		t.Fatal(err)
	}
	account, err := q.CreateSellerAccount(ctx, dbgen.CreateSellerAccountParams{
		UserID: user.ID,
		Name:   "Ozon Enrichment Seed Test",
		Status: "active",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return account.ID
}

func seedOzonEnrichmentFixtures(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sellerID int64, metricDate time.Time) {
	t.Helper()
	q := dbgen.New(pool)
	for i := 1; i <= 10; i++ {
		sku := int64(901_000_100_000 + int64(i))
		productID := int64(100_000 + i)
		if _, err := q.UpsertProduct(ctx, dbgen.UpsertProductParams{
			SellerAccountID: sellerID,
			OzonProductID:   productID,
			OfferID:         pgText("fixture-offer"),
			Sku:             pgInt8(sku),
			Name:            "Fixture Product",
			Status:          pgText("active"),
			IsArchived:      false,
			RawAttributes:   []byte(`{"source":"test_fixture"}`),
			ImageUrls:       []byte("[]"),
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := q.UpsertDailySKUMetric(ctx, dbgen.UpsertDailySKUMetricParams{
			SellerAccountID: sellerID,
			MetricDate:      pgDateUTC(metricDate),
			OzonProductID:   productID,
			OfferID:         pgText("fixture-offer"),
			Sku:             pgInt8(sku),
			ProductName:     pgText("Fixture Product"),
			Revenue:         numericFromFloat(float64(1000 * i)),
			OrdersCount:     int32(i),
			StockAvailable:  int32(10 * i),
		}); err != nil {
			t.Fatal(err)
		}
		// Two warehouses per product so stock_analytics available_stock_count is
		// sourced from the canonical stocks aggregate (sum of latest
		// quantity_available per warehouse): 5*i + 2*i = 7*i.
		for _, wh := range []struct {
			name string
			qty  int32
		}{
			{"WH-A", int32(5 * i)},
			{"WH-B", int32(2 * i)},
		} {
			if _, err := q.UpsertStock(ctx, dbgen.UpsertStockParams{
				SellerAccountID:     sellerID,
				ProductExternalID:   fmt.Sprintf("%d", productID),
				WarehouseExternalID: wh.name,
				QuantityTotal:       pgInt4(wh.qty),
				QuantityReserved:    pgInt4(0),
				QuantityAvailable:   pgInt4(wh.qty),
				SnapshotAt:          pgTimestamptzUTC(metricDate),
				RawAttributes:       []byte(`{"source":"test_fixture"}`),
			}); err != nil {
				t.Fatal(err)
			}
		}
	}
	for i, id := range []int64{990_101, 990_102, 990_103, 990_104} {
		if _, err := q.UpsertAdCampaign(ctx, dbgen.UpsertAdCampaignParams{
			SellerAccountID:    sellerID,
			CampaignExternalID: id,
			CampaignName:       fmt.Sprintf("Fixture Campaign %d", i+1),
			CampaignType:       pgText("SKU"),
			Status:             pgText("running"),
			RawAttributes:      []byte(`{"source":"test_fixture"}`),
		}); err != nil {
			t.Fatal(err)
		}
	}
	legacyRaw, _ := json.Marshal(map[string]any{
		"seeded_by": "dev-seed-mvp",
		"strategy":  "MANUAL",
	})
	for i := 1; i <= 6; i++ {
		campaignID := []int64{990_101, 990_102, 990_103, 990_104}[i%4]
		productID := int64(100_000 + i)
		sku := int64(901_000_100_000 + int64(i))
		if _, err := q.UpsertAdCampaignSKU(ctx, dbgen.UpsertAdCampaignSKUParams{
			SellerAccountID:    sellerID,
			CampaignExternalID: campaignID,
			OzonProductID:      productID,
			OfferID:            pgText(fmt.Sprintf("fixture-offer-%d", i)),
			Sku:                pgInt8(sku),
			IsActive:           true,
			Status:             pgText("active"),
			RawAttributes:      legacyRaw,
		}); err != nil {
			t.Fatal(err)
		}
	}
}
