package dashboardhome

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
)

func TestQualifiesForNoSalesFocusTab(t *testing.T) {
	if !qualifiesForNoSalesFocusTab(10, 0, true) {
		t.Fatal("expected in-stock SKU with zero orders on confirmed day to qualify")
	}
	if qualifiesForNoSalesFocusTab(0, 0, true) {
		t.Fatal("zero stock must not qualify")
	}
	if qualifiesForNoSalesFocusTab(10, 1, true) {
		t.Fatal("positive orders must not qualify")
	}
	if qualifiesForNoSalesFocusTab(10, 0, false) {
		t.Fatal("missing confirmed metric row must not qualify")
	}
}

func TestCountConsecutiveNoSalesDays(t *testing.T) {
	metricDate := mustDate(t, "2026-06-17")
	ordersByDate := map[string]noSalesDayMetric{
		"2026-06-17": {orders: 0, hasData: true},
		"2026-06-16": {orders: 0, hasData: true},
		"2026-06-15": {orders: 2, hasData: true},
	}
	days, ok := countConsecutiveNoSalesDays(metricDate, ordersByDate)
	if !ok || days != 2 {
		t.Fatalf("expected 2 no-sales days, got %d ok=%v", days, ok)
	}

	ordersByDate["2026-06-16"] = noSalesDayMetric{orders: 0, hasData: false}
	days, ok = countConsecutiveNoSalesDays(metricDate, ordersByDate)
	if !ok || days != 1 {
		t.Fatalf("missing prior day should cap streak at 1, got %d ok=%v", days, ok)
	}

	if _, ok := countConsecutiveNoSalesDays(metricDate, map[string]noSalesDayMetric{}); ok {
		t.Fatal("missing check day must not produce streak")
	}
}

func TestResolveNoSalesUnitPrice_prefersReferencePrice(t *testing.T) {
	ref := 1290.0
	got := resolveNoSalesUnitPrice(&ref, map[string]noSalesDayMetric{
		"2026-06-17": {orders: 1, revenue: 500, hasData: true},
	})
	if got == nil || *got != 1290 {
		t.Fatalf("expected reference price, got %+v", got)
	}
}

func TestResolveNoSalesUnitPrice_historicalAverage(t *testing.T) {
	got := resolveNoSalesUnitPrice(nil, map[string]noSalesDayMetric{
		"2026-06-15": {orders: 2, revenue: 2000, hasData: true},
		"2026-06-16": {orders: 0, revenue: 0, hasData: true},
		"2026-06-17": {orders: 0, revenue: 0, hasData: true},
	})
	if got == nil || *got != 1000 {
		t.Fatalf("expected historical avg price 1000, got %+v", got)
	}
}

func TestCalcFrozenStockValue(t *testing.T) {
	price := 500.0
	got := calcFrozenStockValue(78, &price)
	if got == nil || *got != 39000 {
		t.Fatalf("frozen value got %+v want 39000", got)
	}
	if calcFrozenStockValue(0, &price) != nil {
		t.Fatal("zero stock must not produce frozen value")
	}
}

func TestSortNoSalesFocusCandidates(t *testing.T) {
	frozenA := 180000.0
	frozenB := 95000.0
	frozenC := 70000.0
	candidates := []noSalesFocusCandidate{
		{ozonProductID: 3, productName: "C", currentStock: 78, noSalesDays: 4, frozenStockValue: &frozenC},
		{ozonProductID: 1, productName: "A", currentStock: 342, noSalesDays: 2, frozenStockValue: &frozenA},
		{ozonProductID: 2, productName: "B", currentStock: 94, noSalesDays: 1, frozenStockValue: &frozenB},
	}
	sortNoSalesFocusCandidates(candidates)
	if candidates[0].ozonProductID != 1 || candidates[1].ozonProductID != 2 || candidates[2].ozonProductID != 3 {
		t.Fatalf("unexpected sort order: %+v", candidates)
	}
}

func TestBuildNoSalesFocusCandidates_excludesMissingMetricRow(t *testing.T) {
	metricDate := mustDate(t, "2026-06-17")
	skus := []analytics.DashboardSKUMetricsItem{
		{OzonProductID: 1, StockAvailable: 10, OrdersCount: 0},
	}
	metrics := map[int64]noSalesFocusMetrics{
		1: {ordersByDate: map[string]noSalesDayMetric{}},
	}
	if got := buildNoSalesFocusCandidates(skus, metricDate, metrics, nil); len(got) != 0 {
		t.Fatalf("missing confirmed metric row must exclude candidate, got %+v", got)
	}
}

func TestBuildNoSalesFocusCandidates_excludesPositiveOrders(t *testing.T) {
	metricDate := mustDate(t, "2026-06-17")
	checkKey := metricDate.Format("2006-01-02")
	skus := []analytics.DashboardSKUMetricsItem{
		{OzonProductID: 1, StockAvailable: 10, OrdersCount: 2},
	}
	metrics := map[int64]noSalesFocusMetrics{
		1: {ordersByDate: map[string]noSalesDayMetric{
			checkKey: {orders: 2, hasData: true},
		}},
	}
	if got := buildNoSalesFocusCandidates(skus, metricDate, metrics, nil); len(got) != 0 {
		t.Fatalf("positive orders must exclude candidate, got %+v", got)
	}
}

func TestBuildNoSalesFocusCandidates_includesValidCandidate(t *testing.T) {
	metricDate := mustDate(t, "2026-06-17")
	checkKey := metricDate.Format("2006-01-02")
	price := 500.0
	skus := []analytics.DashboardSKUMetricsItem{
		{OzonProductID: 1, StockAvailable: 78, OrdersCount: 0, ProductName: strPtr("Demo SKU")},
	}
	metrics := map[int64]noSalesFocusMetrics{
		1: {ordersByDate: map[string]noSalesDayMetric{
			checkKey:     {orders: 0, hasData: true},
			"2026-06-16": {orders: 0, hasData: true},
			"2026-06-15": {orders: 1, revenue: 500, hasData: true},
		}},
	}
	got := buildNoSalesFocusCandidates(skus, metricDate, metrics, map[int64]float64{1: price})
	if len(got) != 1 {
		t.Fatalf("expected one candidate, got %+v", got)
	}
	if got[0].noSalesDays != 2 {
		t.Fatalf("no_sales_days got %d want 2", got[0].noSalesDays)
	}
	if got[0].frozenStockValue == nil || *got[0].frozenStockValue != 39000 {
		t.Fatalf("frozen_stock_value got %+v want 39000", got[0].frozenStockValue)
	}
}
