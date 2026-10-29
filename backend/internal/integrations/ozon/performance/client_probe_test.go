package performance

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListCampaignsEmptyListIsSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/client/campaign" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"list": []any{},
		})
	}))
	defer server.Close()

	transport := NewTransport()
	transport.baseURL = server.URL
	client := &Client{transport: transport}

	resp, err := client.ListCampaigns(context.Background(), "", "test-token", ListCampaignsRequest{Page: 1, PageSize: 1})
	if err != nil {
		t.Fatalf("empty campaign list should be valid: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
	if len(resp.Data.Items) != 0 {
		t.Fatalf("expected empty items, got %d", len(resp.Data.Items))
	}
}
