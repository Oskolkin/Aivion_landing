package dashboardhome

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
)

func TestQualifiesForStockFocusTab(t *testing.T) {
	doc := 2.5
	if !qualifiesForStockFocusTab(5, &doc) {
		t.Fatal("expected low coverage in-stock SKU to qualify")
	}
	if qualifiesForStockFocusTab(0, &doc) {
		t.Fatal("zero stock must not qualify")
	}
	if qualifiesForStockFocusTab(5, nil) {
		t.Fatal("nil coverage must not qualify")
	}
	zeroDoc := 0.0
	if qualifiesForStockFocusTab(5, &zeroDoc) {
		t.Fatal("zero coverage must not qualify")
	}
	highDoc := 15.0
	if qualifiesForStockFocusTab(5, &highDoc) {
		t.Fatal("high coverage must not qualify")
	}
}

func TestCalcRevenueAtRisk7d(t *testing.T) {
	got := calcRevenueAtRisk7d(3000, 2)
	if got != 15000 {
		t.Fatalf("revenue at risk got %.2f want 15000", got)
	}
	if calcRevenueAtRisk7d(3000, 8) != 0 {
		t.Fatal("coverage above 7 days should have zero revenue at risk")
	}
	if calcRevenueAtRisk7d(0, 2) != 0 {
		t.Fatal("zero avg revenue should have zero revenue at risk")
	}
}

func TestSortStockFocusCandidates(t *testing.T) {
	candidates := []stockFocusCandidate{
		{ozonProductID: 3, productName: "C", coverageDays: 2.2, revenueAtRisk7d: 1000},
		{ozonProductID: 1, productName: "A", coverageDays: 0.6, revenueAtRisk7d: 5000},
		{ozonProductID: 2, productName: "B", coverageDays: 0.6, revenueAtRisk7d: 8000},
	}
	sortStockFocusCandidates(candidates)
	if candidates[0].ozonProductID != 2 || candidates[1].ozonProductID != 1 || candidates[2].ozonProductID != 3 {
		t.Fatalf("unexpected sort order: %+v", candidates)
	}
}

func TestBuildStockFocusCandidate_excludesZeroStock(t *testing.T) {
	doc := 1.5
	metrics := stockFocusMetrics{
		avgDailyRevenue7d: 1000,
		hasRevenue7d:      true,
	}
	if _, ok := buildStockFocusCandidate(1, nil, nil, "Test", 0, doc, metrics); ok {
		t.Fatal("zero stock must be excluded")
	}
}

func TestBuildStockFocusCandidate_requiresRevenue(t *testing.T) {
	doc := 1.5
	metrics := stockFocusMetrics{hasRevenue7d: false}
	if _, ok := buildStockFocusCandidate(1, nil, nil, "Test", 5, doc, metrics); ok {
		t.Fatal("missing revenue must be excluded")
	}
}

func TestBuildCalendarStockTrendSeries_preservesZeroStock(t *testing.T) {
	from := mustDate(t, "2026-06-11")
	to := mustDate(t, "2026-06-13")
	series := buildCalendarStockTrendSeries(from, to, map[string]stockTrendDayMetrics{
		"2026-06-11": {stock: 0, hasData: true},
		"2026-06-12": {stock: 2, hasData: true},
	})
	if len(series) != 3 {
		t.Fatalf("expected 3 points, got %d", len(series))
	}
	if !series[0].HasData || series[0].Stock != 0 {
		t.Fatalf("historical zero stock must be preserved: %+v", series[0])
	}
}

func TestPreviousStockFromTrend(t *testing.T) {
	trend := []StockTrendPoint{
		{Date: "2026-06-11", Stock: 14, HasData: true},
		{Date: "2026-06-12", Stock: 11, HasData: true},
		{Date: "2026-06-13", Stock: 2, HasData: true},
	}
	prev := previousStockFromTrend(trend)
	if prev == nil || *prev != 11 {
		t.Fatalf("previous stock got %v want 11", prev)
	}
}

func TestIsDashboardStockFocusSKU_excludesOutOfStockBucket(t *testing.T) {
	b := "out_of_stock"
	doc := 0.5
	if isDashboardStockFocusSKU(analytics.DashboardSKUMetricsItem{
		StockStatusBucket: &b,
		StockAvailable:    0,
		DaysOfCover:       &doc,
	}) {
		t.Fatal("out_of_stock bucket with zero stock must not qualify")
	}
}

func TestIsReplenishmentStockFocusCandidate_excludesCriticalOutOfStock(t *testing.T) {
	doc := 0.5
	item := analytics.ReplenishmentSKUItem{
		DepletionRisk:         "critical_out_of_stock",
		ReplenishmentPriority: "high",
		CurrentAvailableStock: 0,
		DaysOfCover:           &doc,
	}
	if isReplenishmentStockFocusCandidate(item) {
		t.Fatal("critical_out_of_stock with zero stock must not qualify")
	}
}
