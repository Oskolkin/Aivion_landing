package dashboardfacts

import (
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func TestBuildSellerPulseFacts_AOVAndAdShare(t *testing.T) {
	metricDate := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	snap := &analysisdata.AnalysisDataSnapshot{
		AccountMetrics: []dbgen.DailyAccountMetric{
			{MetricDate: date(metricDate.AddDate(0, 0, -1)), Revenue: mustNumeric(t, 201400), OrdersCount: 346},
			{MetricDate: date(metricDate), Revenue: mustNumeric(t, 184600), OrdersCount: 312},
		},
		AdMetrics: []dbgen.AdMetricsDaily{
			{MetricDate: date(metricDate), Spend: mustNumeric(t, 34350)},
		},
	}
	pct := -16800.0 / 201400.0
	adRatio := 34350.0 / 184600.0
	metrics := analytics.DashboardMetricsDTO{
		Account: analytics.DashboardAccountMetrics{
			RevenueToday:            184600,
			RevenueYesterday:        201400,
			OrdersToday:             312,
			OrdersYesterday:         346,
			AccountAdSpendToRevenue: &adRatio,
			RevenueDayDelta: analytics.DashboardDelta{
				Abs: -16800,
				Pct: &pct,
			},
		},
	}

	got := buildSellerPulseFacts(snap, metrics, metricDate)
	keys := map[string]bool{}
	for _, kpi := range got.KPIs {
		keys[kpi.Key] = true
		switch kpi.Key {
		case "average_order_value":
			if kpi.Label != "Средний чек" {
				t.Fatalf("aov label=%q", kpi.Label)
			}
		case "ad_spend_share_total_revenue":
			if kpi.Label != "Рекламные расходы / общая выручка" {
				t.Fatalf("ad share label=%q", kpi.Label)
			}
		case "ctr", "drr":
			t.Fatalf("legacy pulse key %q must not be emitted", kpi.Key)
		}
	}
	if !keys["average_order_value"] {
		t.Fatal("expected average_order_value KPI")
	}
	if !keys["ad_spend_share_total_revenue"] {
		t.Fatal("expected ad_spend_share_total_revenue KPI")
	}
}

func TestBuildAOVKPI_ZeroOrdersOmits(t *testing.T) {
	_, ok := buildAOVKPI(1000, 0, 900, 10, nil)
	if ok {
		t.Fatal("AOV must be omitted when orders_today=0")
	}
}
