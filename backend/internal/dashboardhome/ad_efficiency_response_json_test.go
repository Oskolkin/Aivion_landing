package dashboardhome

import (
	"encoding/json"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
)

func TestEmptyAdEfficiencyWidgetJSONShape(t *testing.T) {
	w := emptyAdEfficiencyWidget("")
	w.Narrative = &AdEfficiencyNarrative{Status: "not_enough_data"}
	assertAdEfficiencyJSONExcludesFactualFields(t, w, "not_enough_data")
}

func TestErrorAdEfficiencyWidgetJSONShape(t *testing.T) {
	w := errorAdEfficiencyWidget("")
	w.Narrative = &AdEfficiencyNarrative{Status: "unavailable"}
	assertAdEfficiencyJSONExcludesFactualFields(t, w, "error")
}

func TestAvailableAdEfficiencyWidgetJSONShape(t *testing.T) {
	drr := 22.5
	w := mapAdEfficiencyDashboardDTO(analytics.AdEfficiencyDashboardDTO{
		HasData:      true,
		WidgetStatus: "available",
		Period: analytics.AdEfficiencyPeriodDTO{
			MetricDate: "2026-03-10",
			DateFrom:   "2026-03-04",
			DateTo:     "2026-03-10",
			SampleDays: 7,
		},
		Summary: analytics.AdEfficiencySummaryDTO{
			CurrentDRRPct:       &drr,
			HighDRRThresholdPct: analytics.AdEfficiencyHighDRRThresholdPct,
		},
		Trend7D: []analytics.AdEfficiencyTrendPointDTO{
			{Date: "2026-03-10", DataStatus: "available"},
		},
	})
	raw, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"period", "summary", "trend_7d", "cta"} {
		if _, ok := m[key]; !ok {
			t.Fatalf("available response missing %q", key)
		}
	}
}

func TestPartialAdEfficiencyWidgetJSONShape(t *testing.T) {
	drr := 25.0
	w := mapAdEfficiencyDashboardDTO(analytics.AdEfficiencyDashboardDTO{
		HasData:      true,
		WidgetStatus: "partial",
		Period: analytics.AdEfficiencyPeriodDTO{
			MetricDate: "2026-03-10",
			DateFrom:   "2026-03-04",
			DateTo:     "2026-03-10",
			SampleDays: 3,
		},
		Summary: analytics.AdEfficiencySummaryDTO{
			CurrentDRRPct:       &drr,
			HighDRRThresholdPct: analytics.AdEfficiencyHighDRRThresholdPct,
		},
		Trend7D: []analytics.AdEfficiencyTrendPointDTO{
			{Date: "2026-03-08", DataStatus: "available"},
			{Date: "2026-03-09", DataStatus: "missing"},
			{Date: "2026-03-10", DataStatus: "available"},
		},
	})
	raw, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m["status"] != "partial" {
		t.Fatalf("status=%v", m["status"])
	}
	if _, ok := m["period"]; !ok {
		t.Fatal("partial should include populated period")
	}
	if _, ok := m["summary"]; !ok {
		t.Fatal("partial should include calculated summary")
	}
	period := m["period"].(map[string]any)
	if period["metric_date"] == "" || period["date_from"] == "" || period["date_to"] == "" {
		t.Fatalf("partial period must have real dates: %+v", period)
	}
	if m["message"] == nil || m["message"] == "" {
		t.Fatal("partial should include user-facing limitation message")
	}
}

func assertAdEfficiencyJSONExcludesFactualFields(t *testing.T, w AdEfficiencyWidget, wantStatus string) {
	t.Helper()
	raw, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m["status"] != wantStatus {
		t.Fatalf("status=%v want %s", m["status"], wantStatus)
	}
	if m["message"] == nil || m["message"] == "" {
		t.Fatal("expected message")
	}
	for _, key := range []string{"period", "summary", "trend_7d", "high_drr_campaigns", "no_order_campaigns", "points"} {
		if _, ok := m[key]; ok {
			t.Fatalf("%s response must omit %q, got %v", wantStatus, key, m[key])
		}
	}
	if _, ok := m["cta"]; !ok {
		t.Fatal("expected cta")
	}
	narr, ok := m["ad_efficiency_narrative"].(map[string]any)
	if ok {
		if narr["summary"] != nil && narr["summary"] != "" {
			t.Fatal("narrative must not include deterministic summary")
		}
		if checklist, ok := narr["checklist"].([]any); ok && len(checklist) > 0 {
			t.Fatal("narrative must not include checklist")
		}
	}
}
