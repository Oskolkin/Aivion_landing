package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/metrics"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestMetricsRoutePattern_ActionsGetAction(t *testing.T) {
	registry := prometheus.NewRegistry()
	m := metrics.New(registry)

	r := chi.NewRouter()
	r.Use(Metrics(m))
	r.Get("/api/v1/actions/{action_key}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/actions/stock_replenish_sku_123", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusOK)
	}

	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}

	const wantPattern = "/api/v1/actions/{action_key}"
	found := false
	for _, mf := range metricFamilies {
		if mf.GetName() != "http_requests_total" {
			continue
		}
		for _, metric := range mf.GetMetric() {
			labels := labelMap(metric)
			if labels["method"] == http.MethodGet && labels["path"] == wantPattern && labels["status"] == "200" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatalf("expected http_requests_total for route %q", wantPattern)
	}
}

func labelMap(metric *dto.Metric) map[string]string {
	out := make(map[string]string, len(metric.Label))
	for _, label := range metric.Label {
		out[label.GetName()] = label.GetValue()
	}
	return out
}
