package dashboardhome

import (
	"testing"
	"time"
)

func TestBuildCalendarOrdersRevenueSparklineSeries_denseSevenDays(t *testing.T) {
	to := time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC)
	from := to.AddDate(0, 0, -6)
	byDate := map[string]sparklineDayMetrics{
		"2026-06-11": {orders: 0, revenue: 0, hasOrders: true, hasRevenue: true},
		"2026-06-13": {orders: 2, revenue: 8500, hasOrders: true, hasRevenue: true},
		"2026-06-14": {orders: 0, revenue: 0, hasOrders: true, hasRevenue: true},
		"2026-06-17": {orders: 2, revenue: 6990, hasOrders: true, hasRevenue: true},
	}
	points := buildCalendarOrdersRevenueSparklineSeries(from, to, byDate)
	if len(points) != 7 {
		t.Fatalf("len=%d want 7", len(points))
	}
	if points[0].Date != "2026-06-11" || !points[0].HasData || points[0].Value == nil || *points[0].Value != 0 {
		t.Fatalf("2026-06-11: %+v", points[0])
	}
	if points[1].Date != "2026-06-12" || points[1].HasData || points[1].Value == nil || *points[1].Value != 0 {
		t.Fatalf("missing day should be has_data=false: %+v", points[1])
	}
	if points[1].Revenue != nil {
		t.Fatalf("missing day revenue must be nil: %+v", points[1])
	}
	if points[2].Value == nil || *points[2].Value != 2 {
		t.Fatalf("2026-06-13 orders: %+v", points[2])
	}
	if points[3].Value == nil || *points[3].Value != 0 {
		t.Fatalf("zero orders day must stay: %+v", points[3])
	}
	for i := 1; i < len(points); i++ {
		if points[i].Date <= points[i-1].Date {
			t.Fatalf("dates not ascending: %s then %s", points[i-1].Date, points[i].Date)
		}
	}
}

func TestBuildCalendarSparklineSeries_missingDaysNotZero(t *testing.T) {
	to := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	from := to.AddDate(0, 0, -6)
	byDate := map[string]float64{
		"2026-05-16": 3,
		"2026-05-18": 0,
	}
	points := buildCalendarSparklineSeries(from, to, byDate)
	if len(points) != 7 {
		t.Fatalf("len=%d", len(points))
	}
	if points[3].Date != "2026-05-15" || points[3].Value != nil || points[3].HasData {
		t.Fatalf("missing day should be nil: %+v", points[3])
	}
	if points[4].Value == nil || *points[4].Value != 3 || !points[4].HasData {
		t.Fatalf("2026-05-16: %+v", points[4])
	}
	if points[6].Value == nil || *points[6].Value != 0 || !points[6].HasData {
		t.Fatalf("real zero: %+v", points[6])
	}
}
