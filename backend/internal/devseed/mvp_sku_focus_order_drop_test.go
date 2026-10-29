package devseed

import (
	"testing"
)

func TestMVPOrderDropDemoSeries(t *testing.T) {
	wantDeltas := []int32{-1, -1, 0, -1, 0}
	for i, scenario := range mvpOrderDropDemoSKUs {
		if got := mvpOrderDropDemoOrdersDelta(scenario.Orders7D); got != wantDeltas[i] {
			t.Fatalf("%s orders delta got %d want %d", scenario.ProductName, got, wantDeltas[i])
		}

		var revenues [7]float64
		var totalOrders int32
		for d := 0; d < 7; d++ {
			orders := scenario.Orders7D[d]
			totalOrders += orders
			price := mvpOrderDropDemoUnitPrice(scenario.BasePrice, scenario.PriceFactors[d])
			revenues[d] = mvpOrderDropDemoDayRevenue(orders, price)
			if orders > 0 && revenues[d] <= 0 {
				t.Fatalf("%s day %d: orders=%d revenue=%.2f", scenario.ProductName, d, orders, revenues[d])
			}
			if orders == 0 && revenues[d] != 0 {
				t.Fatalf("%s day %d: zero orders must have zero revenue", scenario.ProductName, d)
			}
		}
		if totalOrders < 20 {
			t.Fatalf("%s current-week orders=%d want >= 20 for material WoW decline", scenario.ProductName, totalOrders)
		}
	}

	names := make(map[string]struct{}, len(mvpOrderDropDemoSKUs))
	for _, scenario := range mvpOrderDropDemoSKUs {
		if _, ok := names[scenario.ProductName]; ok {
			t.Fatalf("duplicate demo product name: %s", scenario.ProductName)
		}
		names[scenario.ProductName] = struct{}{}
	}
	if len(mvpOrderDropDemoSKUs) < 4 || len(mvpOrderDropDemoSKUs) > 6 {
		t.Fatalf("order-drop demos=%d want 4–6", len(mvpOrderDropDemoSKUs))
	}
}

func TestMVPOrderDropDemoOrdersPatternsDistinct(t *testing.T) {
	seen := make(map[[7]int32]struct{}, len(mvpOrderDropDemoSKUs))
	for _, scenario := range mvpOrderDropDemoSKUs {
		if _, ok := seen[scenario.Orders7D]; ok {
			t.Fatalf("duplicate orders pattern for %s", scenario.ProductName)
		}
		seen[scenario.Orders7D] = struct{}{}
	}
}
