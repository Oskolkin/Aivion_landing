package dashboardhome

import (
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
)

func TestSelectKeySkuCandidates_excludesZeroRevenue(t *testing.T) {
	candidates := buildKeySkuCandidates([]analytics.DashboardSKUMetricsItem{
		{OzonProductID: 1, RevenueLast7Days: 0, OrdersLast7Days: 5},
		{OzonProductID: 2, RevenueLast7Days: 1000, OrdersLast7Days: 3},
	})
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	selected := selectKeySkuCandidates(candidates, calcStoreTotalRevenue7dFromDashboardSKUs([]analytics.DashboardSKUMetricsItem{
		{OzonProductID: 2, RevenueLast7Days: 1000, OrdersLast7Days: 3},
	}))
	if len(selected) != 1 || selected[0].ozonProductID != 2 {
		t.Fatalf("unexpected selection: %+v", selected)
	}
}

func TestSelectKeySkuCandidates_cumulative70StopsWhenCrossed(t *testing.T) {
	names := []string{"A", "B", "C", "D", "E", "F", "G"}
	items := make([]analytics.DashboardSKUMetricsItem, 0, 7)
	for i := 0; i < 5; i++ {
		name := names[i]
		items = append(items, analytics.DashboardSKUMetricsItem{
			OzonProductID: int64(i + 1), ProductName: &name, RevenueLast7Days: 154, OrdersLast7Days: 10,
		})
	}
	nameF := names[5]
	items = append(items, analytics.DashboardSKUMetricsItem{
		OzonProductID: 6, ProductName: &nameF, RevenueLast7Days: 60, OrdersLast7Days: 8,
	})
	nameG := names[6]
	items = append(items, analytics.DashboardSKUMetricsItem{
		OzonProductID: 7, ProductName: &nameG, RevenueLast7Days: 170, OrdersLast7Days: 5,
	})
	candidates := buildKeySkuCandidates(items)
	selected := selectKeySkuCandidates(candidates, calcStoreTotalRevenue7dFromDashboardSKUs(items))
	if len(selected) != 5 {
		t.Fatalf("expected 5 selected SKUs (crosses ≥70%% at D), got %d: %+v", len(selected), selected)
	}
	if selected[len(selected)-1].cumulativeSharePct < keySkuRevenueShareThresholdPct {
		t.Fatalf("cumulative share should be >= %.0f, got %.1f", keySkuRevenueShareThresholdPct, selected[len(selected)-1].cumulativeSharePct)
	}
}

func TestSortKeySkuCandidates_revenueThenOrders(t *testing.T) {
	candidates := []keySkuCandidate{
		{ozonProductID: 1, productName: "A", revenue7d: 100, orders7d: 5},
		{ozonProductID: 2, productName: "B", revenue7d: 100, orders7d: 10},
		{ozonProductID: 3, productName: "C", revenue7d: 200, orders7d: 1},
	}
	sortKeySkuCandidates(candidates)
	if candidates[0].ozonProductID != 3 || candidates[1].ozonProductID != 2 || candidates[2].ozonProductID != 1 {
		t.Fatalf("unexpected order: %+v", candidates)
	}
}

func TestBuildCalendarKeySkuRevenueTrend_sevenPoints(t *testing.T) {
	from := mustKeySkuDate(t, "2026-06-11")
	to := mustKeySkuDate(t, "2026-06-17")
	series := buildCalendarKeySkuRevenueTrend(from, to, map[string]sparklineDayMetrics{
		"2026-06-11": {orders: 2, revenue: 1000, hasOrders: true, hasRevenue: true},
		"2026-06-13": {orders: 0, revenue: 0, hasOrders: true, hasRevenue: true},
	})
	if len(series) != 7 {
		t.Fatalf("expected 7 points, got %d", len(series))
	}
	if series[0].HasData == nil || !*series[0].HasData {
		t.Fatal("first point should have data")
	}
	if series[1].HasData != nil {
		t.Fatal("missing day should not set has_data")
	}
	if series[0].Revenue != 1000 || series[0].Orders != 2 {
		t.Fatalf("unexpected first point: %+v", series[0])
	}
}

func TestSelectKeySkuCandidates_sharesUseStoreTotal(t *testing.T) {
	storeTotal := 7_395_560.0
	candidates := []keySkuCandidate{
		{ozonProductID: 1, revenue7d: 998_400},
		{ozonProductID: 2, revenue7d: 900_000},
	}
	selected := selectKeySkuCandidates(candidates, storeTotal)
	if len(selected) != 2 {
		t.Fatalf("expected 2 selected, got %d", len(selected))
	}
	wantShare := roundKeySkuPct((998_400 / storeTotal) * 100)
	if selected[0].revenueSharePct != wantShare {
		t.Fatalf("share got %.1f want %.1f", selected[0].revenueSharePct, wantShare)
	}
	sumFirstTwo := 998_400.0 + 900_000.0
	wantCumulative := roundKeySkuPct((sumFirstTwo / storeTotal) * 100)
	if selected[1].cumulativeSharePct != wantCumulative {
		t.Fatalf("cumulative got %.1f want %.1f", selected[1].cumulativeSharePct, wantCumulative)
	}
	if sumFirstTwo/storeTotal >= 0.8 {
		t.Fatal("test fixture should keep first two below 80% of store total")
	}
}

func TestKeySkuProductIDs(t *testing.T) {
	ids := KeySkuProductIDs([]analytics.DashboardSKUMetricsItem{
		{OzonProductID: 10, RevenueLast7Days: 800},
		{OzonProductID: 20, RevenueLast7Days: 200},
		{OzonProductID: 30, RevenueLast7Days: 0},
	})
	if len(ids) != 1 || ids[0] != 10 {
		t.Fatalf("expected only high-revenue SKU, got %v", ids)
	}
}

func mustKeySkuDate(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatal(err)
	}
	return parsed.UTC()
}
