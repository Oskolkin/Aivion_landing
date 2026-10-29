package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/productcatalog"
)

func TestParseProductsPaginationEnforcesMaxLimit(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/products?limit=9999&offset=10", nil)
	limit, offset := parseProductsPagination(req)
	if limit != productcatalog.MaxLimit {
		t.Fatalf("limit=%d want %d", limit, productcatalog.MaxLimit)
	}
	if offset != 10 {
		t.Fatalf("offset=%d", offset)
	}
}

func TestParseProductsListParamsHasSales(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/products?has_sales_last_day=true&sort=orders_last_day", nil)
	params := parseProductsListParams(req)
	if params.HasSalesLastDay == nil || !*params.HasSalesLastDay {
		t.Fatal("expected has sales true")
	}
	if params.SortBy != "orders_last_day" {
		t.Fatalf("sort=%q", params.SortBy)
	}
}
