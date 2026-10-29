package devseed

import "testing"

func TestMVPStockRiskDemoSeries(t *testing.T) {
	wantCoverage := []float64{0.6, 0.9, 3.5, 5.0}
	if len(mvpStockRiskDemoSKUs) != len(wantCoverage) {
		t.Fatalf("stock-risk demos=%d want %d", len(mvpStockRiskDemoSKUs), len(wantCoverage))
	}
	for i, scenario := range mvpStockRiskDemoSKUs {
		name := scenario.productName()
		if scenario.CatalogIndex >= 80 {
			t.Fatalf("%s catalog index %d must be < default --products=80", name, scenario.CatalogIndex)
		}
		if scenario.Stock7D[6] <= 0 {
			t.Fatalf("%s current stock must be > 0", name)
		}
		if scenario.CoverageDays != wantCoverage[i] {
			t.Fatalf("%s coverage got %.1f want %.1f", name, scenario.CoverageDays, wantCoverage[i])
		}

		seenStock := make(map[int32]struct{})
		for _, stock := range scenario.Stock7D {
			seenStock[stock] = struct{}{}
		}
		if len(seenStock) < 4 {
			t.Fatalf("%s stock series must have varied values, got %v", name, scenario.Stock7D)
		}

		revenues := [7]float64{}
		for d := 0; d < 7; d++ {
			revenues[d] = mvpStockRiskDemoDayRevenue(scenario.OrdersPerDay, scenario.BasePrice)
		}
		avg := 0.0
		for _, rev := range revenues {
			avg += rev
		}
		avg /= 7
		if mvpStockRiskDemoRevenueAtRisk(avg, scenario.CoverageDays) <= 0 {
			t.Fatalf("%s must have positive revenue at risk", name)
		}
	}

	names := make(map[string]struct{}, len(mvpStockRiskDemoSKUs))
	for _, scenario := range mvpStockRiskDemoSKUs {
		name := scenario.productName()
		if _, ok := names[name]; ok {
			t.Fatalf("duplicate demo product name: %s", name)
		}
		names[name] = struct{}{}
	}
}

func TestMVPStockRiskDemoStockPatternsDistinct(t *testing.T) {
	seen := make(map[[7]int32]struct{}, len(mvpStockRiskDemoSKUs))
	for _, scenario := range mvpStockRiskDemoSKUs {
		if _, ok := seen[scenario.Stock7D]; ok {
			t.Fatalf("duplicate stock pattern for %s", scenario.productName())
		}
		seen[scenario.Stock7D] = struct{}{}
	}
}

func TestMVPStockRiskDemoDoesNotOverlapOrderDrop(t *testing.T) {
	orderDrop := make(map[string]struct{}, len(mvpOrderDropDemoSKUs))
	for _, scenario := range mvpOrderDropDemoSKUs {
		orderDrop[scenario.ProductName] = struct{}{}
	}
	for _, scenario := range mvpStockRiskDemoSKUs {
		name := scenario.productName()
		if _, ok := orderDrop[name]; ok {
			t.Fatalf("stock-risk demo overlaps order-drop product: %s", name)
		}
	}
}
