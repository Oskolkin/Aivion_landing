package ozon

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetSellerAPIRolesParsesMethods(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/roles" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"expires_at": "2026-02-18T09:54:23.296Z",
			"roles": []any{
				map[string]any{"name": "Admin", "methods": []any{"/v1/actions", "/v3/product/info/list"}},
			},
		})
	}))
	defer server.Close()

	client := NewClientWithConfig(server.Client(), server.URL, DefaultRetryConfig())
	resp, err := client.GetSellerAPIRoles(context.Background(), "client", "key")
	if err != nil {
		t.Fatalf("roles request failed: %v", err)
	}
	if resp.Data.ExpiresAt == "" || len(resp.Data.Roles) != 1 || len(resp.Data.Roles[0].Methods) != 2 {
		t.Fatalf("unexpected roles response: %+v", resp.Data)
	}
}

func TestGetRelatedSKUsParsesItems(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{
				map[string]any{
					"sku":             222,
					"product_id":      10,
					"delivery_schema": "FBO",
					"availability":    "AVAILABLE",
					"deleted_at":      nil,
				},
			},
			"errors": []any{},
		})
	}))
	defer server.Close()

	client := NewClientWithConfig(server.Client(), server.URL, DefaultRetryConfig())
	resp, err := client.GetRelatedSKUs(context.Background(), "client", "key", []int64{111})
	if err != nil {
		t.Fatalf("related sku request failed: %v", err)
	}
	if len(resp.Data.Items) != 1 || resp.Data.Items[0].SKU != 222 || resp.Data.Items[0].DeliverySchema != "FBO" {
		t.Fatalf("unexpected related sku response: %+v", resp.Data.Items)
	}
}

func TestGetRolesForbiddenIsUnauthorizedClass(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer server.Close()

	client := NewClientWithConfig(server.Client(), server.URL, DefaultRetryConfig())
	_, err := client.GetSellerAPIRoles(context.Background(), "client", "key")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
}
