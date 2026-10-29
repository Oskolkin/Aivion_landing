package dashboardfacts

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func TestBuildSKUSparklines_MissingDaysAreNotZero(t *testing.T) {
	sellerID := int64(42)
	metricDate := mustDate(t, "2025-03-10")

	rows := []dbgen.DailySkuMetric{
		{SellerAccountID: sellerID, MetricDate: date(metricDate), OzonProductID: 9001, Revenue: mustNumeric(t, 500), OrdersCount: 3},
		// metricDate-1 intentionally missing for product 9001.
		{SellerAccountID: sellerID, MetricDate: date(metricDate.AddDate(0, 0, -2)), OzonProductID: 9001, Revenue: mustNumeric(t, 200), OrdersCount: 1},
	}

	sparklines := buildSKUSparklines(rows, metricDate.AddDate(0, 0, -6), metricDate)
	points := sparklines[9001]
	if len(points) != 7 {
		t.Fatalf("expected 7 sparkline points, got %d", len(points))
	}
	if points[6].Date != "2025-03-10" || !points[6].HasData {
		t.Fatalf("expected metric_date point to have data, got %+v", points[6])
	}
	if points[6].Value == nil || *points[6].Value != 3 {
		t.Fatalf("expected metric_date orders=3, got %v", points[6].Value)
	}
	if points[6].Revenue == nil || *points[6].Revenue != 500 {
		t.Fatalf("expected metric_date revenue=500, got %v", points[6].Revenue)
	}

	missingDay := points[5] // metricDate-1
	if missingDay.Date != "2025-03-09" {
		t.Fatalf("expected missing day to be 2025-03-09, got %s", missingDay.Date)
	}
	if missingDay.HasData {
		t.Fatalf("expected missing day HasData=false, got true")
	}
	if missingDay.Value != nil || missingDay.Revenue != nil {
		t.Fatalf("expected missing day Value/Revenue to stay nil (never coerced to zero), got value=%v revenue=%v", missingDay.Value, missingDay.Revenue)
	}
}

func TestBuildSKUSparklines_NoRowsForProduct(t *testing.T) {
	metricDate := mustDate(t, "2025-03-10")
	sparklines := buildSKUSparklines(nil, metricDate.AddDate(0, 0, -6), metricDate)
	if len(sparklines) != 0 {
		t.Fatalf("expected no sparklines, got %d", len(sparklines))
	}
}

func TestBuildDashboardFacts_SKUFactRecordCarriesSparkline(t *testing.T) {
	sellerID := int64(42)
	// testSnapshot already carries product 9001 on both the metric date and the
	// previous day (a reconciling two-day series), which is exactly what a
	// multi-day sparkline needs.
	snap := testSnapshot(t, sellerID, "2025-03-10")

	facts, report, err := BuildDashboardFacts(snap, PackIdentity{PackID: "pack:42:2025-03-10:abc", PackHash: "abc"})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Valid {
		t.Fatalf("validation failed: %+v", report)
	}
	rec, ok := facts.SKUFactsIndex.ByOzonProductID["9001"]
	if !ok {
		t.Fatal("expected sku fact record for 9001")
	}
	if len(rec.Sparkline7D) != 7 {
		t.Fatalf("expected 7 sparkline points, got %d", len(rec.Sparkline7D))
	}
	observed := 0
	for _, pt := range rec.Sparkline7D {
		if pt.HasData {
			observed++
		}
	}
	if observed != 2 {
		t.Fatalf("expected 2 observed days (today + yesterday), got %d", observed)
	}
	if facts.ContentCounts.SKUsWithSparkline != 1 {
		t.Fatalf("expected 1 sku with multi-day sparkline, got %d", facts.ContentCounts.SKUsWithSparkline)
	}
}
