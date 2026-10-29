package ozon

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPromotionClientExposesOnlyReadEndpoints(t *testing.T) {
	var methods = map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods[r.URL.Path] = r.Method
		body, _ := io.ReadAll(r.Body)
		if r.URL.Path != ActionsListPath && len(body) == 0 {
			t.Errorf("POST %s had no request", r.URL.Path)
		}
		if r.URL.Path == ActionsListPath {
			_, _ = w.Write([]byte(`{"result":[{"id":1,"title":"Action"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"products":[{"id":55,"offer_id":"offer"}],"last_id":55}`))
	}))
	defer server.Close()
	client := NewClientWithConfig(server.Client(), server.URL, RetryConfig{MaxAttempts: 1})
	if _, err := client.promotionActionsBearer(context.Background(), "token", ActionsListPath); err != nil {
		t.Fatal(err)
	}
	if _, err := client.promotionProductsBearer(context.Background(), "token", ActionCandidatesPath, PromotionProductRequest{ActionID: 1, Limit: 100}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.promotionProductsBearer(context.Background(), "token", ActionProductsPath, PromotionProductRequest{ActionID: 1, Limit: 100}); err != nil {
		t.Fatal(err)
	}
	if methods[ActionsListPath] != http.MethodGet || methods[ActionCandidatesPath] != http.MethodPost || methods[ActionProductsPath] != http.MethodPost {
		t.Fatalf("unexpected methods: %v", methods)
	}
}

func TestPromotionProductKeepsProductIDSeparateFromSKU(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"products":[{"id":123,"offer_id":"x"}]}`))
	}))
	defer server.Close()
	client := NewClientWithConfig(server.Client(), server.URL, RetryConfig{MaxAttempts: 1})
	response, err := client.promotionProductsBearer(context.Background(), "token", ActionProductsPath, PromotionProductRequest{ActionID: 1, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	row := response.Data.Rows()[0]
	if int64(row.ProductID) != 123 || int64(row.SKU) != 0 {
		t.Fatalf("product ID became SKU: %+v", row)
	}
}
