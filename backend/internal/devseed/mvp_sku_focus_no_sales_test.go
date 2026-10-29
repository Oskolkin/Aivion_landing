package devseed

import "testing"

func TestMVPNoSalesDemoSeries(t *testing.T) {
	if len(mvpNoSalesDemoSKUs) < 3 || len(mvpNoSalesDemoSKUs) > 5 {
		t.Fatalf("expected 3–5 demo no-sales SKUs, got %d", len(mvpNoSalesDemoSKUs))
	}

	names := make(map[string]struct{}, len(mvpNoSalesDemoSKUs))
	stockRiskNames := make(map[string]struct{}, len(mvpStockRiskDemoSKUs))
	for _, scenario := range mvpStockRiskDemoSKUs {
		stockRiskNames[scenario.productName()] = struct{}{}
	}

	for _, scenario := range mvpNoSalesDemoSKUs {
		name := scenario.productName()
		if scenario.CatalogIndex >= 80 {
			t.Fatalf("%s catalog index %d must be < default --products=80", name, scenario.CatalogIndex)
		}
		if scenario.Stock <= 0 {
			t.Fatalf("%s stock must be > 0", name)
		}
		if scenario.NoSalesDays < 14 {
			t.Fatalf("%s no_sales_days must be >= 14, got %d", name, scenario.NoSalesDays)
		}
		if _, ok := names[name]; ok {
			t.Fatalf("duplicate demo product name: %s", name)
		}
		names[name] = struct{}{}
		if _, ok := stockRiskNames[name]; ok {
			t.Fatalf("no-sales demo overlaps stock-risk demo: %s", name)
		}
		if mvpNoSalesDemoFrozenValue(scenario.Stock, scenario.BasePrice) <= 0 {
			t.Fatalf("%s must have positive frozen value", name)
		}
		if mvpNoSalesDemoOrdersForDay(6, scenario.NoSalesDays, 6) != 0 {
			t.Fatalf("%s anchor day must have zero orders", name)
		}
	}
}

func TestMVPNoSalesDemoSortOrder(t *testing.T) {
	type row struct {
		name   string
		frozen float64
	}
	rows := make([]row, 0, len(mvpNoSalesDemoSKUs))
	for _, scenario := range mvpNoSalesDemoSKUs {
		rows = append(rows, row{
			name:   scenario.productName(),
			frozen: mvpNoSalesDemoFrozenValue(scenario.Stock, scenario.BasePrice),
		})
	}
	for i := 1; i < len(rows); i++ {
		if rows[i-1].frozen < rows[i].frozen {
			t.Fatalf("demo SKUs should be defined in descending frozen value order, %s before %s", rows[i-1].name, rows[i].name)
		}
	}
}
