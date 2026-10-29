package ozon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestListProductsProductionWireContract(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v3/product/list" {
			t.Fatalf("path=%s", request.URL.Path)
		}
		if err := json.NewDecoder(request.Body).Decode(&requestBody); err != nil {
			t.Fatal(err)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"result":{"items":[{"product_id":123,"offer_id":"ABC","sku":456,"archived":false}],"last_id":"next-cursor","total":1}}`))
	}))
	defer server.Close()

	client := NewClientWithConfig(server.Client(), server.URL, RetryConfig{MaxAttempts: 1})
	response, err := client.ListProducts(context.Background(), "client", "secret", ListProductsRequest{
		Filter: ProductListFilter{OfferID: []string{"ABC"}, ProductID: []int64{123}, SKUs: []int64{456}, Visibility: "ALL"},
		LastID: "",
		Limit:  1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := requestBody["visibility"]; exists {
		t.Fatalf("top-level visibility leaked into request: %#v", requestBody)
	}
	filter, ok := requestBody["filter"].(map[string]any)
	if !ok || filter["visibility"] != "ALL" {
		t.Fatalf("filter=%#v", requestBody["filter"])
	}
	if !reflect.DeepEqual(filter["offer_id"], []any{"ABC"}) || !reflect.DeepEqual(filter["product_id"], []any{float64(123)}) || !reflect.DeepEqual(filter["skus"], []any{float64(456)}) {
		t.Fatalf("nested identifiers=%#v", filter)
	}
	if requestBody["last_id"] != "" || requestBody["limit"] != float64(1000) {
		t.Fatalf("pagination=%#v", requestBody)
	}
	if len(response.Data.Items) != 1 {
		t.Fatalf("items=%+v", response.Data.Items)
	}
	item := response.Data.Items[0]
	if item.ProductID != 123 || item.OfferID != "ABC" || item.SKU != 456 || item.Archived || response.Data.LastID != "next-cursor" {
		t.Fatalf("decoded response=%+v result=%+v", item, response.Data)
	}
}
