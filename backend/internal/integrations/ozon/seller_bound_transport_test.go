package ozon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	ozonperf "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon/performance"
)

func TestOAuthTokenValueCannotBeJSONSerialized(t *testing.T) {
	body, err := json.Marshal(OAuthTokens{AccessToken: "access-secret", RefreshToken: "refresh-secret"})
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "{}" {
		t.Fatalf("OAuth tokens are loggable as JSON: %s", body)
	}
}

type fixedTokenProvider struct {
	mu      sync.Mutex
	sellers []int64
}
type instantAnalyticsLimiter struct{}

func (instantAnalyticsLimiter) Wait(context.Context) error { return nil }

func (p *fixedTokenProvider) CanonicalAccessToken(_ context.Context, sellerID int64) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sellers = append(p.sellers, sellerID)
	return "seller-bound-token", nil
}

func TestSellerBoundTransportUsesBearerForCanonicalSellerAnalyticsAndPerformanceWithoutLegacyHeaders(t *testing.T) {
	var mu sync.Mutex
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		mu.Unlock()
		if got := r.Header.Get("Authorization"); got != "Bearer seller-bound-token" {
			t.Errorf("authorization=%q", got)
		}
		if r.Header.Get("Client-Id") != "" || r.Header.Get("Api-Key") != "" {
			t.Errorf("legacy auth headers leaked: %v", r.Header)
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v3/product/list" {
			_, _ = w.Write([]byte(`{"result":{"items":[],"last_id":""}}`))
			return
		}
		if r.URL.Path == analyticsDataPath {
			_, _ = w.Write([]byte(`{"result":{"data":[]}}`))
			return
		}
		if r.URL.Path == CanonicalStockAnalyticsPath {
			_, _ = w.Write([]byte(`{"items":[]}`))
			return
		}
		if r.URL.Path == CanonicalPriceListPath {
			_, _ = w.Write([]byte(`{"items":[],"cursor":"","total":0}`))
			return
		}
		_, _ = w.Write([]byte(`{"list":[]}`))
	}))
	defer server.Close()

	tokens := &fixedTokenProvider{}
	transport := NewSellerBoundTransport(
		tokens,
		NewClientWithConfig(server.Client(), server.URL, RetryConfig{MaxAttempts: 1}),
		ozonperf.NewClientWithConfig(server.Client(), server.URL),
	)
	transport.analyticsLimiter = instantAnalyticsLimiter{}
	if _, err := transport.ListProducts(context.Background(), 42, ListProductsRequest{}); err != nil {
		t.Fatal(err)
	}
	if _, err := transport.FetchCanonicalAnalyticsData(context.Background(), 42, AnalyticsDataRequest{Metrics: []string{"revenue", "ordered_units"}, Dimension: []string{"day"}, Limit: 1000}); err != nil {
		t.Fatal(err)
	}
	if _, err := transport.ListPerformanceCampaigns(context.Background(), 42, ozonperf.ListCampaignsRequest{}); err != nil {
		t.Fatal(err)
	}
	if _, err := transport.ListCanonicalStocks(context.Background(), 42, CanonicalStockRequest{}); err != nil {
		t.Fatal(err)
	}
	if _, err := transport.ListCanonicalPrices(context.Background(), 42, CanonicalPriceRequest{Limit: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := transport.GetPerformanceDailyStatistics(context.Background(), 42, ozonperf.CampaignStatisticsRequest{}); err != nil {
		t.Fatal(err)
	}
	if _, err := transport.GetPerformanceCampaignProducts(context.Background(), 42, ozonperf.CampaignProductsRequest{CampaignID: 1}); err != nil {
		t.Fatal(err)
	}
	if requests != 7 || len(tokens.sellers) != 7 {
		t.Fatalf("requests=%d sellers=%v", requests, tokens.sellers)
	}
	for _, seller := range tokens.sellers {
		if seller != 42 {
			t.Fatalf("cross-seller token request: %v", tokens.sellers)
		}
	}
}
