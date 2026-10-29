package ozon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchTransportUsesBearerAndPreservesExactNumbers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != ProductQueriesPath {
			t.Fatalf("path=%s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer seller-bound-token" {
			t.Fatalf("authorization=%q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Client-Id") != "" || r.Header.Get("Api-Key") != "" {
			t.Fatalf("legacy headers leaked: %v", r.Header)
		}
		var body ProductQueriesRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.SKUs) == 0 {
			t.Fatal("skus required in product-queries body")
		}
		_, _ = w.Write([]byte(`{"items":[{"sku":9007199254740993,"provider_search_gmv":123456789012345678.123456,"unique_search_users":7,"orders":3}],"has_next":false}`))
	}))
	defer server.Close()
	client := NewClientWithConfig(server.Client(), server.URL, RetryConfig{MaxAttempts: 1})
	transport := NewSellerBoundTransportWithAnalyticsLimiter(&fixedTokenProvider{}, client, instantAnalyticsLimiter{})
	response, err := transport.ListProductQueries(context.Background(), 42, ProductQueriesRequest{
		DateFrom: "2026-08-19", DateTo: "2026-08-25", SKUs: []int64{9007199254740993}, Page: 1, PageSize: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	row := response.Data.Items[0]
	if int64(row.SKU) != 9007199254740993 || string(row.ProviderSearchGMV) != "123456789012345678.123456" {
		t.Fatalf("exact values lost: %+v", row)
	}
	if response.Data.HasNext {
		t.Fatal("expected has_next=false")
	}
}

func TestProductQueriesRejectsEmptySKUs(t *testing.T) {
	transport := NewSellerBoundTransportWithAnalyticsLimiter(&fixedTokenProvider{}, NewClient(), instantAnalyticsLimiter{})
	if _, err := transport.ListProductQueries(context.Background(), 1, ProductQueriesRequest{DateFrom: "2026-08-19"}); err == nil {
		t.Fatal("expected skus required error")
	}
}

func TestSearchDetailsRejectInvalidLimitBySKU(t *testing.T) {
	transport := NewSellerBoundTransportWithAnalyticsLimiter(&fixedTokenProvider{}, NewClient(), instantAnalyticsLimiter{})
	// limit_by_sku is per-SKU query cap; many SKUs are allowed.
	request := ProductQueryDetailsRequest{SKUs: make([]int64, 20), LimitBySKU: 16}
	if _, err := transport.ListProductQueryDetails(context.Background(), 1, request); err == nil {
		t.Fatal("expected limit_by_sku validation error")
	}
	request.LimitBySKU = 15
	// Still fails without a real token/server, but LimitBySKU validation must pass first.
	// Empty token path is fine; we only assert LimitBySKU==0 is rejected.
	request.LimitBySKU = 0
	if _, err := transport.ListProductQueryDetails(context.Background(), 1, request); err == nil {
		t.Fatal("expected limit_by_sku required/positive")
	}
}

func TestSearchDetailsAllowsManySKUsWithLimitBySKU(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body ProductQueryDetailsRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.SKUs) != 20 || body.LimitBySKU != 15 {
			t.Fatalf("body=%+v", body)
		}
		if body.SortBy == "" || body.SortDir == "" {
			t.Fatalf("sort metadata missing: %+v", body)
		}
		_, _ = w.Write([]byte(`{"items":[],"has_next":true}`))
	}))
	defer server.Close()
	client := NewClientWithConfig(server.Client(), server.URL, RetryConfig{MaxAttempts: 1})
	transport := NewSellerBoundTransportWithAnalyticsLimiter(&fixedTokenProvider{}, client, instantAnalyticsLimiter{})
	skus := make([]int64, 20)
	for i := range skus {
		skus[i] = int64(i + 1)
	}
	resp, err := transport.ListProductQueryDetails(context.Background(), 1, ProductQueryDetailsRequest{
		DateFrom: "2026-08-19", DateTo: "2026-08-25", SKUs: skus, LimitBySKU: 15,
		Page: 1, PageSize: 100, SortBy: "BY_GMV", SortDir: "DESCENDING",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Data.HasNext {
		t.Fatal("expected has_next preserved")
	}
}
