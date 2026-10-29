package analytics

import (
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func TestBuildAdEfficiencyTrend7D_AscendingDateOrder(t *testing.T) {
	metricDate := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	dateFrom := metricDate.AddDate(0, 0, -6)
	daily := map[string]dailyAdAggregate{
		metricDate.Format("2006-01-02"): {spend: 10, revenue: 100, hasData: true},
	}
	trend := buildAdEfficiencyTrend7D(dateFrom, metricDate, daily)
	if len(trend) != 7 {
		t.Fatalf("want 7 points, got %d", len(trend))
	}
	for i := 1; i < len(trend); i++ {
		if trend[i].Date <= trend[i-1].Date {
			t.Fatalf("not ascending at %d: %s after %s", i, trend[i].Date, trend[i-1].Date)
		}
	}
	if trend[0].Date != dateFrom.Format("2006-01-02") {
		t.Fatalf("first=%s want %s", trend[0].Date, dateFrom.Format("2006-01-02"))
	}
	if trend[len(trend)-1].Date != metricDate.Format("2006-01-02") {
		t.Fatalf("last=%s want %s", trend[len(trend)-1].Date, metricDate.Format("2006-01-02"))
	}
}

func TestComputeAdEfficiencyDashboard_Trend7D_EndToEndOrder(t *testing.T) {
	metricDate := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	daily := []dbgen.AdMetricsDaily{
		adDailyRow(metricDate.AddDate(0, 0, -2), 1, 50, 200, 1),
		adDailyRow(metricDate, 1, 100, 400, 2),
	}
	accountRev := map[string]float64{
		metricDate.AddDate(0, 0, -2).Format("2006-01-02"): 200,
		metricDate.Format("2006-01-02"):                   400,
	}
	dto := computeAdEfficiencyDashboard(nil, daily, accountRev, metricDate, nil)
	for i := 1; i < len(dto.Trend7D); i++ {
		if dto.Trend7D[i].Date <= dto.Trend7D[i-1].Date {
			t.Fatalf("trend not ascending: %+v", dto.Trend7D)
		}
	}
	if dto.Period.DateFrom != dto.Trend7D[0].Date {
		t.Fatalf("period.date_from %s != first trend %s", dto.Period.DateFrom, dto.Trend7D[0].Date)
	}
	if dto.Period.DateTo != dto.Trend7D[len(dto.Trend7D)-1].Date {
		t.Fatalf("period.date_to %s != last trend %s", dto.Period.DateTo, dto.Trend7D[len(dto.Trend7D)-1].Date)
	}
}
