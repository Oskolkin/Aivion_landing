package dashboardhome

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/stockpolicy"
)

func f64ptr(v float64) *float64 { return &v }

func TestReconcileFrozenStockFocusMembership_IncludesPlanning(t *testing.T) {
	t.Parallel()
	name := "Planning SKU"
	metrics := analytics.DashboardMetricsDTO{
		SKUs: []analytics.DashboardSKUMetricsItem{
			{OzonProductID: 1, StockAvailable: 0, DaysOfCover: f64ptr(0), RevenueLast7Days: 100},
			{OzonProductID: 2, StockAvailable: 5, DaysOfCover: f64ptr(2), RevenueLast7Days: 200},
			{OzonProductID: 3, StockAvailable: 10, DaysOfCover: f64ptr(5), RevenueLast7Days: 150},
			{OzonProductID: 4, StockAvailable: 20, DaysOfCover: f64ptr(10), RevenueLast7Days: 80, ProductName: &name},
			{OzonProductID: 5, StockAvailable: 50, DaysOfCover: f64ptr(30), RevenueLast7Days: 50},
		},
	}
	// Force canonical buckets via StockStatusBucket so classification is explicit.
	oos := stockpolicy.StatusOutOfStock
	crit := stockpolicy.StatusCriticalShortage
	short := stockpolicy.StatusShortage7D
	plan := stockpolicy.StatusPlanning14D
	healthy := stockpolicy.StatusHealthy
	metrics.SKUs[0].StockStatusBucket = &oos
	metrics.SKUs[1].StockStatusBucket = &crit
	metrics.SKUs[2].StockStatusBucket = &short
	metrics.SKUs[3].StockStatusBucket = &plan
	metrics.SKUs[4].StockStatusBucket = &healthy

	focus := SKUFocus{
		StockRisk: []SKUFocusCard{
			{OzonProductID: 1, Problem: "AI out", KeyMetric: "0"},
			{OzonProductID: 2, Problem: "AI crit", KeyMetric: "2"},
		},
		StockRiskTotalCount: 2,
	}
	reconcileFrozenStockFocusMembership(&focus, metrics)

	if focus.StockRiskTotalCount != 3 {
		t.Fatalf("attention total=%d want 3", focus.StockRiskTotalCount)
	}
	if len(focus.StockRisk) != 4 {
		t.Fatalf("list len=%d want 4 (risk+planning)", len(focus.StockRisk))
	}
	counts := map[string]int{}
	for _, c := range focus.StockRisk {
		counts[c.CoverageBucket]++
	}
	if counts[stockpolicy.SegmentPlanning8_14] != 1 {
		t.Fatalf("planning missing: %+v", counts)
	}
	if counts[stockpolicy.SegmentOutOfStock] != 1 || counts[stockpolicy.SegmentCritical0_3] != 1 || counts[stockpolicy.SegmentShortage4_7] != 1 {
		t.Fatalf("unexpected counts: %+v", counts)
	}
	// Preserve AI copy for existing cards.
	if focus.StockRisk[0].Problem != "AI out" {
		t.Fatalf("lost AI problem: %q", focus.StockRisk[0].Problem)
	}
}
