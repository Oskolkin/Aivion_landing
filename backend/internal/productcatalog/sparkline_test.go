package productcatalog

import (
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestBuildSparklineSeries_missingDaysNotZeroFilled(t *testing.T) {
	latest := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	from := latest.AddDate(0, 0, -6)

	metricsByDate := map[string]dbgen.DailySkuMetric{
		"2026-05-16": {
			OrdersCount: 3,
			Revenue:     mustNumeric(t, "300"),
		},
		"2026-05-18": {
			OrdersCount: 0,
			Revenue:     mustNumeric(t, "0"),
		},
	}

	points := buildSparklineSeries(from, latest, metricsByDate)
	if len(points) != 7 {
		t.Fatalf("len=%d", len(points))
	}

	// Day with sales activity.
	if points[4].Date != "2026-05-16" || points[4].OrdersCount == nil || *points[4].OrdersCount != 3 {
		t.Fatalf("2026-05-16 point=%+v", points[4])
	}

	// Missing day: no row in daily_sku_metrics — must not be zero-filled.
	if points[3].Date != "2026-05-15" {
		t.Fatalf("date=%s", points[3].Date)
	}
	if points[3].OrdersCount != nil || points[3].Revenue != nil {
		t.Fatalf("2026-05-15 should be missing data, got orders=%v revenue=%v", points[3].OrdersCount, points[3].Revenue)
	}

	// Row exists with zero orders (real zero, not missing).
	if points[6].Date != "2026-05-18" || points[6].OrdersCount == nil || *points[6].OrdersCount != 0 {
		t.Fatalf("2026-05-18 point=%+v", points[6])
	}
	if points[6].Revenue == nil || *points[6].Revenue != 0 {
		t.Fatalf("2026-05-18 revenue=%v", points[6].Revenue)
	}
}

func mustNumeric(t *testing.T, s string) pgtype.Numeric {
	t.Helper()
	var n pgtype.Numeric
	if err := n.Scan(s); err != nil {
		t.Fatal(err)
	}
	return n
}
