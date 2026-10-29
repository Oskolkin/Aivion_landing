package dashboardfacts

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func categorizedSnapshot(t *testing.T, sellerID int64, metricDateISO string) *analysisdata.AnalysisDataSnapshot {
	t.Helper()
	snap := testSnapshot(t, sellerID, metricDateISO)
	md := mustDate(t, metricDateISO)
	prev := md.AddDate(0, 0, -1)

	snap.Products = []dbgen.Product{
		{ID: 1, SellerAccountID: sellerID, OzonProductID: 9001, Name: "Product A", DescriptionCategoryID: pgtype.Int8{Int64: 100, Valid: true}},
		{ID: 2, SellerAccountID: sellerID, OzonProductID: 9002, Name: "Product B", DescriptionCategoryID: pgtype.Int8{Int64: 100, Valid: true}},
		{ID: 3, SellerAccountID: sellerID, OzonProductID: 9003, Name: "Product C", DescriptionCategoryID: pgtype.Int8{Int64: 200, Valid: true}},
	}
	snap.Categories = []dbgen.DescriptionCategory{
		{DescriptionCategoryID: 100, CategoryName: "Электроника"},
		{DescriptionCategoryID: 200, CategoryName: "Одежда"},
	}
	// Replace metrics with a fully self-consistent set: the account totals equal
	// the SKU sums per day so the snapshot passes source reconciliation.
	//   today (md):  9001 1500/12 + 9002 500/2 + 9003 1000/4 = 3000 / 18
	//   prev:        9001 1000/10 + 9002 400/1              = 1400 / 11
	snap.SKUMetrics = []dbgen.DailySkuMetric{
		{SellerAccountID: sellerID, MetricDate: date(md), OzonProductID: 9001, Revenue: mustNumeric(t, 1500), OrdersCount: 12, StockAvailable: 5},
		{SellerAccountID: sellerID, MetricDate: date(md), OzonProductID: 9002, Revenue: mustNumeric(t, 500), OrdersCount: 2, StockAvailable: 0},
		{SellerAccountID: sellerID, MetricDate: date(md), OzonProductID: 9003, Revenue: mustNumeric(t, 1000), OrdersCount: 4, StockAvailable: 50},
		{SellerAccountID: sellerID, MetricDate: date(prev), OzonProductID: 9001, Revenue: mustNumeric(t, 1000), OrdersCount: 10, StockAvailable: 5},
		{SellerAccountID: sellerID, MetricDate: date(prev), OzonProductID: 9002, Revenue: mustNumeric(t, 400), OrdersCount: 1, StockAvailable: 0},
	}
	snap.AccountMetrics = []dbgen.DailyAccountMetric{
		{SellerAccountID: sellerID, MetricDate: date(md), Revenue: mustNumeric(t, 3000), OrdersCount: 18},
		{SellerAccountID: sellerID, MetricDate: date(prev), Revenue: mustNumeric(t, 1400), OrdersCount: 11},
	}
	return snap
}

func TestBuildDashboardFacts_CategoryFactsAggregateRevenueAndDeltas(t *testing.T) {
	sellerID := int64(42)
	snap := categorizedSnapshot(t, sellerID, "2025-03-10")

	facts, report, err := BuildDashboardFacts(snap, PackIdentity{PackID: "pack:42:2025-03-10:abc", PackHash: "abc"})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Valid {
		t.Fatalf("validation failed: %+v", report)
	}
	if len(facts.Categories) != 2 {
		t.Fatalf("expected 2 categories, got %d: %+v", len(facts.Categories), facts.Categories)
	}

	var electronics, apparel *CategoryFacts
	for i := range facts.Categories {
		switch facts.Categories[i].CategoryID {
		case 100:
			electronics = &facts.Categories[i]
		case 200:
			apparel = &facts.Categories[i]
		}
	}
	if electronics == nil || apparel == nil {
		t.Fatalf("expected both categories present, got %+v", facts.Categories)
	}

	// Category 100 (Электроника): product 9001 (1500 today, from
	// testSnapshot) + product 9002 (500 today) = 2000 revenue, 12+2=14 orders.
	if electronics.CategoryName != "Электроника" {
		t.Fatalf("unexpected category name: %q", electronics.CategoryName)
	}
	if electronics.SKUCount != 2 {
		t.Fatalf("expected 2 skus in category 100, got %d", electronics.SKUCount)
	}
	if electronics.Revenue != 2000 {
		t.Fatalf("expected revenue 2000, got %v", electronics.Revenue)
	}
	if electronics.Orders != 14 {
		t.Fatalf("expected orders 14, got %d", electronics.Orders)
	}
	// Previous day: 9001=1000 + 9002=400 = 1400.
	if electronics.PreviousRevenue != 1400 {
		t.Fatalf("expected previous_revenue 1400, got %v", electronics.PreviousRevenue)
	}
	if electronics.RevenueDeltaDayToDay != 600 {
		t.Fatalf("expected revenue_delta_day_to_day 600, got %v", electronics.RevenueDeltaDayToDay)
	}
	if electronics.StockRiskSKUCount != 1 {
		t.Fatalf("expected 1 stock-risk sku (9002 has 0 stock), got %d", electronics.StockRiskSKUCount)
	}

	// Category 200 (Одежда): product 9003, 1000 revenue, 4 orders, no
	// previous-day row => 0 previous revenue/orders.
	if apparel.Revenue != 1000 || apparel.Orders != 4 {
		t.Fatalf("unexpected apparel aggregation: %+v", apparel)
	}
	if apparel.PreviousRevenue != 0 || apparel.PreviousOrders != 0 {
		t.Fatalf("expected no previous-day data for apparel, got %+v", apparel)
	}

	// Shares sum to (approximately) 100% across categories.
	totalShare := 0.0
	for _, c := range facts.Categories {
		if c.RevenueShare != nil {
			totalShare += *c.RevenueShare
		}
	}
	if totalShare < 99.9 || totalShare > 100.1 {
		t.Fatalf("expected revenue shares to sum to ~100%%, got %v", totalShare)
	}

	if facts.DataCoverage.CategoryCount != 2 {
		t.Fatalf("expected data_coverage.category_count=2, got %d", facts.DataCoverage.CategoryCount)
	}
	if facts.ContentCounts.CategoryCount != 2 {
		t.Fatalf("expected content_counts.category_count=2, got %d", facts.ContentCounts.CategoryCount)
	}
}

func TestBuildDashboardFacts_NoCategoriesWhenCatalogUncategorized(t *testing.T) {
	snap := testSnapshot(t, 42, "2025-03-10")
	facts, report, err := BuildDashboardFacts(snap, PackIdentity{PackID: "pack:42:2025-03-10:abc", PackHash: "abc"})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Valid {
		t.Fatalf("validation failed: %+v", report)
	}
	if len(facts.Categories) != 0 {
		t.Fatalf("expected no categories for an uncategorized catalog, got %+v", facts.Categories)
	}
	if facts.DataCoverage.CategoryCount != 0 {
		t.Fatalf("expected data_coverage.category_count=0, got %d", facts.DataCoverage.CategoryCount)
	}
}

// TestValidateDashboardFacts_ErrorsWhenCatalogCategorizedButFactsEmpty
// covers the acceptance-completeness rule: a seller catalog that IS
// categorized (DataCoverage.CategoryCount > 0) must never publish an
// entirely empty Categories facts slice.
func TestValidateDashboardFacts_ErrorsWhenCatalogCategorizedButFactsEmpty(t *testing.T) {
	facts := DashboardFactsV1{
		SchemaVersion:   SchemaVersion,
		SellerAccountID: 42,
		MetricDate:      "2025-03-10",
		PackID:          "pack:42:2025-03-10:abc",
		PackHash:        "abc",
		FactsHash:       "deadbeef",
		DataCoverage:    DataCoverage{CategoryCount: 3},
	}
	report := validateDashboardFacts(facts)
	if report.Valid {
		t.Fatal("expected validation to fail when catalog has categories but facts.Categories is empty")
	}
	found := false
	for _, e := range report.Errors {
		if e.Code == "missing_category_facts" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected missing_category_facts error, got %+v", report.Errors)
	}
}
