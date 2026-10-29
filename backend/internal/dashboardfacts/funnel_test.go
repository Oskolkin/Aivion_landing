package dashboardfacts

import (
	"math"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestBuildFunnelFactsUsesCompletedDaysAndObservedDailySeries(t *testing.T) {
	metricDate := funnelTestDate(t, "2026-07-08")
	rows := []dbgen.ProductAnalyticsDaily{
		funnelTestRow(t, "2026-07-02", "", 1000, 100, 20, 5, 4),
		funnelTestRow(t, "2026-07-03", "account", 1100, 110, 22, 6, 5),
		funnelTestRow(t, "2026-07-04", "", 1200, 120, 24, 7, 6),
		funnelTestRow(t, "2026-07-05", "account", 1300, 130, 26, 8, 7),
		funnelTestRow(t, "2026-07-06", "account", 1400, 140, 28, 10, 9),
		funnelTestRow(t, "2026-07-07", "", 1500, 180, 45, 18, 15),
		// metric_date is intentionally very large: it must affect neither stages nor series.
		funnelTestRow(t, "2026-07-08", "", 90000, 80000, 70000, 60000, 50000),
	}
	productRow := funnelTestRow(t, "2026-07-07", "", 999999, 999999, 999999, 999999, 999999)
	productRow.ScopeType = "product"
	rows = append(rows, productRow)

	facts := buildFunnelFacts(&analysisdata.AnalysisDataSnapshot{FunnelRows: rows}, metricDate)

	if facts.Status != "available" {
		t.Fatalf("status=%q", facts.Status)
	}
	if facts.CurrentCompletedDate != "2026-07-07" || facts.PreviousCompletedDate != "2026-07-06" {
		t.Fatalf("completed dates=%s/%s", facts.CurrentCompletedDate, facts.PreviousCompletedDate)
	}
	if len(facts.DailySeries) != 6 {
		t.Fatalf("daily series len=%d want 6", len(facts.DailySeries))
	}
	for i, wantDay := range []string{"02", "03", "04", "05", "06", "07"} {
		want := "2026-07-" + wantDay
		if facts.DailySeries[i].Date != want {
			t.Fatalf("daily_series[%d].date=%s want %s", i, facts.DailySeries[i].Date, want)
		}
	}
	for _, point := range facts.DailySeries {
		if point.Date == "2026-07-08" {
			t.Fatal("metric_date must not be present in daily_series")
		}
	}

	assertFunnelStage(t, facts.Stages, "impressions", 1500, 1400, 7.14, nil, nil, nil)
	assertFunnelStage(t, facts.Stages, "product_views", 180, 140, 28.57, floatPtr(12), floatPtr(10), floatPtr(2))
	assertFunnelStage(t, facts.Stages, "add_to_cart", 45, 28, 60.71, floatPtr(25), floatPtr(20), floatPtr(5))
	assertFunnelStage(t, facts.Stages, "orders", 18, 10, 80, floatPtr(40), floatPtr(35.71), floatPtr(4.29))
	assertFunnelStage(t, facts.Stages, "delivered_units", 15, 9, 66.67, floatPtr(83.33), floatPtr(90), floatPtr(-6.67))
	if facts.Overview7D.Status != "available" {
		t.Fatalf("overview_7d status=%q", facts.Overview7D.Status)
	}
	weeklyImpressions := funnelWeeklyStageByKey(t, facts.Overview7D.Stages, "impressions")
	if weeklyImpressions.Value == nil || *weeklyImpressions.Value != 97500 {
		t.Fatalf("overview weekly impressions=%v want current 7d sum 97500", weeklyImpressions.Value)
	}

	// This explicitly rejects the old current-window sum (1000+...+90000).
	if got := *funnelStageByKey(t, facts.Stages, "impressions").Value; got != 1500 {
		t.Fatalf("impressions=%d: completed-day value was summed", got)
	}
}

func TestBuildFunnelFactsMissingPreviousDayKeepsComparisonNull(t *testing.T) {
	metricDate := funnelTestDate(t, "2026-07-08")
	current := funnelTestRow(t, "2026-07-07", "account", 1500, 180, 45, 18, 15)
	facts := buildFunnelFacts(&analysisdata.AnalysisDataSnapshot{FunnelRows: []dbgen.ProductAnalyticsDaily{current}}, metricDate)
	if facts.Status != "available" {
		t.Fatalf("status=%q", facts.Status)
	}
	for _, stage := range facts.Stages {
		if stage.PreviousDayValue != nil || stage.ChangeVsPreviousDayPct != nil || stage.PreviousConversionFromPrevious != nil || stage.ConversionDeltaPp != nil {
			t.Fatalf("stage %s has synthetic comparison: %+v", stage.Key, stage)
		}
	}
}

func funnelTestRow(t *testing.T, date, scopeID string, impressions, views, cart, orders, delivered int64) dbgen.ProductAnalyticsDaily {
	t.Helper()
	return dbgen.ProductAnalyticsDaily{
		MetricDate:      pgtype.Date{Time: funnelTestDate(t, date), Valid: true},
		ScopeType:       "account",
		ScopeID:         scopeID,
		Impressions:     pgtype.Int8{Int64: impressions, Valid: true},
		ProductViews:    pgtype.Int8{Int64: views, Valid: true},
		AddToCart:       pgtype.Int8{Int64: cart, Valid: true},
		OrdersCount:     pgtype.Int8{Int64: orders, Valid: true},
		CompletedOrders: pgtype.Int8{Int64: delivered, Valid: true},
		SourceStatus:    pgtype.Text{String: "available", Valid: true},
	}
}

func funnelTestDate(t *testing.T, iso string) time.Time {
	t.Helper()
	date, err := time.Parse("2006-01-02", iso)
	if err != nil {
		t.Fatal(err)
	}
	return date.UTC()
}

func funnelStageByKey(t *testing.T, stages []FunnelStage, key string) FunnelStage {
	t.Helper()
	for _, stage := range stages {
		if stage.Key == key {
			return stage
		}
	}
	t.Fatalf("stage %s not found", key)
	return FunnelStage{}
}

func funnelWeeklyStageByKey(t *testing.T, stages []FunnelWeeklyStage, key string) FunnelWeeklyStage {
	t.Helper()
	for _, stage := range stages {
		if stage.Key == key {
			return stage
		}
	}
	t.Fatalf("weekly stage %s not found", key)
	return FunnelWeeklyStage{}
}

func assertFunnelStage(t *testing.T, stages []FunnelStage, key string, value, previous int64, change float64, conversion, previousConversion, conversionDelta *float64) {
	t.Helper()
	stage := funnelStageByKey(t, stages, key)
	if stage.Value == nil || *stage.Value != value || stage.PreviousDayValue == nil || *stage.PreviousDayValue != previous {
		t.Fatalf("stage %s values=%v/%v want %d/%d", key, stage.Value, stage.PreviousDayValue, value, previous)
	}
	assertOptionalFloat(t, key+" change", stage.ChangeVsPreviousDayPct, &change)
	assertOptionalFloat(t, key+" conversion", stage.ConversionFromPrevious, conversion)
	assertOptionalFloat(t, key+" previous conversion", stage.PreviousConversionFromPrevious, previousConversion)
	assertOptionalFloat(t, key+" conversion delta", stage.ConversionDeltaPp, conversionDelta)
}

func assertOptionalFloat(t *testing.T, label string, got, want *float64) {
	t.Helper()
	if got == nil || want == nil {
		if got != nil || want != nil {
			t.Fatalf("%s=%v want %v", label, got, want)
		}
		return
	}
	if math.Abs(*got-*want) > 0.001 {
		t.Fatalf("%s=%v want %v", label, *got, *want)
	}
}

func floatPtr(value float64) *float64 { return &value }
