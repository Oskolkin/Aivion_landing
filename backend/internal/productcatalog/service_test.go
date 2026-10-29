package productcatalog

import "testing"

func TestBuildOzonProductURL(t *testing.T) {
	if BuildOzonProductURL(nil) != "" {
		t.Fatal("expected empty for nil sku")
	}
	zero := int64(0)
	if BuildOzonProductURL(&zero) != "" {
		t.Fatal("expected empty for zero sku")
	}
	sku := int64(123456)
	got := BuildOzonProductURL(&sku)
	want := "https://www.ozon.ru/context/detail/id/123456"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNormalizeSortBy(t *testing.T) {
	if normalizeSortBy("") != "revenue_last_day" {
		t.Fatal("default sort")
	}
	if normalizeSortBy("orders_last_day") != "orders_last_day" {
		t.Fatal("orders sort")
	}
	if normalizeSortBy("unknown") != "revenue_last_day" {
		t.Fatal("fallback sort")
	}
}

func TestStockMatchesFilter(t *testing.T) {
	shortage := "shortage_7d"
	item := CatalogItem{
		LastDay: &LastDayMetrics{OrdersCount: 2},
		Stock:   &StockInfo{StockStatusBucket: &shortage},
	}
	if !stockMatchesFilter(item, "had_sales_last_day") {
		t.Fatal("expected sales match")
	}
	if !stockMatchesFilter(item, "stock_risk") {
		t.Fatal("expected stock risk match")
	}
}
