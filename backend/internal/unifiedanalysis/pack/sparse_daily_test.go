package pack

import (
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestBuildAccountDailyTable_OmitsSyntheticZeroDays(t *testing.T) {
	idx := &Indexes{
		AccountMetricsByDate: map[string]dbgen.DailyAccountMetric{
			"2026-07-08": {
				MetricDate:  pgtype.Date{Time: time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), Valid: true},
				Revenue:     mustNumeric(t, "100"),
				OrdersCount: 2,
			},
		},
		FunnelByDate:          map[string]dbgen.ProductAnalyticsDaily{},
		CampaignUniverse:      nil,
		CampaignMetricsByDate: map[int64]map[string]campaignMetricDay{},
	}
	periods := Periods{
		Trend: analysisdata.DateRange{From: "2026-07-08", To: "2026-07-13"},
	}
	table := BuildAccountDailyTable(idx, periods, 1)
	if len(table.Rows) != 1 {
		t.Fatalf("expected 1 observed day, got %d rows: %v", len(table.Rows), table.Rows)
	}
	if table.Rows[0][2] != "2026-07-08" {
		t.Fatalf("date=%s", table.Rows[0][2])
	}

	// Real zero on an observed day must still be emitted.
	idx.AccountMetricsByDate["2026-07-09"] = dbgen.DailyAccountMetric{
		MetricDate:  pgtype.Date{Time: time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC), Valid: true},
		Revenue:     mustNumeric(t, "0"),
		OrdersCount: 0,
	}
	table = BuildAccountDailyTable(idx, periods, 1)
	if len(table.Rows) != 2 {
		t.Fatalf("expected observed zero day to be kept, got %d", len(table.Rows))
	}
}

func TestDaysWithoutSales_GapsAreNotZeros(t *testing.T) {
	byDate := map[string]skuMetricDay{
		"2026-07-08": {Orders: 0},
		// 09-12 missing — gaps must not inflate consecutive zero-sales count
		"2026-07-13": {Orders: 0},
	}
	got := daysWithoutSalesFromMetrics(byDate, "2026-07-13", analysisdata.DateRange{
		From: "2026-07-08",
		To:   "2026-07-13",
	}, 0)
	if got != 1 {
		t.Fatalf("expected only the observed zero day at metric date, got %d", got)
	}
}
