package funnel

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

type fakeAnalyticsAPI struct{ requests []ozon.AnalyticsDataRequest }

func (f *fakeAnalyticsAPI) FetchCanonicalAnalyticsData(_ context.Context, _ int64, req ozon.AnalyticsDataRequest) (*ozon.CanonicalAnalyticsResponse, error) {
	f.requests = append(f.requests, req)
	raw := []byte(`{"result":{"data":[]}}`)
	return &ozon.CanonicalAnalyticsResponse{Raw: raw, Meta: ozon.ResponseMeta{StatusCode: 200}}, nil
}

type fakeRegistrar struct{ next int64 }

func (f *fakeRegistrar) StoreAndRegisterPayload(_ context.Context, _ *collection.SourcePayloadStore, in collection.RegisterPayloadInput) (collection.SourcePayload, error) {
	f.next++
	metadata, _ := json.Marshal(in.Metadata)
	return collection.SourcePayload{ID: f.next, SellerAccountID: in.SellerAccountID, DomainRunID: in.DomainRunID, ReceivedAt: time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC), Metadata: metadata}, nil
}

func TestCollectSeparatesDailyAndDirectPeriodMetrics(t *testing.T) {
	api := &fakeAnalyticsAPI{}
	runner := &Runner{API: api, Collection: &fakeRegistrar{}}
	_, err := runner.Collect(context.Background(), 1, 2, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(api.requests) != 6 {
		t.Fatalf("got %d streams, want 6", len(api.requests))
	}
	for _, req := range api.requests {
		for _, metric := range req.Metrics {
			if metric == "ordered_units" || metric == "revenue" {
				t.Fatalf("sales metric leaked into funnel request: %s", metric)
			}
		}
	}
	current := api.requests[2]
	if current.DateFrom != "2026-08-14" || current.DateTo != "2026-08-20" {
		t.Fatalf("current direct period was not requested as an exact 7d window: %+v", current)
	}
	previous := api.requests[4]
	if previous.DateFrom != "2026-08-07" || previous.DateTo != "2026-08-13" {
		t.Fatalf("previous direct period was not requested as an exact 7d window: %+v", previous)
	}
	foundProviderConversion := false
	for _, metric := range current.Metrics {
		if metric == "conv_tocart_search" {
			foundProviderConversion = true
		}
	}
	if !foundProviderConversion {
		t.Fatal("provider conversion must be collected directly, not reconstructed")
	}
}

func TestPendingBroadMetricsAreNotCollected(t *testing.T) {
	api := &fakeAnalyticsAPI{}
	runner := &Runner{API: api, Collection: &fakeRegistrar{}}
	_, err := runner.Collect(context.Background(), 1, 2, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	for _, req := range api.requests {
		for _, metric := range req.Metrics {
			switch metric {
			case "hits_view", "hits_tocart", "session_view", "conv_tocart":
				t.Fatalf("pending/deprecated provider candidate was collected: %s", metric)
			}
		}
	}
}
