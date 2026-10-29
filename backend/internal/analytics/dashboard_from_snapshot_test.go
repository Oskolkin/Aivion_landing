package analytics

import (
	"fmt"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestBuildDashboardMetricsFromSnapshot(t *testing.T) {
	md := mustDateOnly(t, "2025-04-01")
	snap := &analysisdata.AnalysisDataSnapshot{
		SellerAccountID: 7,
		MetricDate:      pgtype.Date{Time: md, Valid: true},
		AccountMetrics: []dbgen.DailyAccountMetric{
			{SellerAccountID: 7, MetricDate: pgDate(md), Revenue: mustNum(t, 200), OrdersCount: 4},
		},
		SKUMetrics: []dbgen.DailySkuMetric{
			{
				SellerAccountID: 7, MetricDate: pgDate(md), OzonProductID: 501,
				Revenue: mustNum(t, 200), OrdersCount: 4, StockAvailable: 2,
			},
		},
	}
	dto, err := BuildDashboardMetricsFromSnapshot(snap)
	if err != nil {
		t.Fatal(err)
	}
	if dto.SellerAccountID != 7 || dto.AsOfDate != "2025-04-01" {
		t.Fatalf("dto=%+v", dto)
	}
	if dto.Account.RevenueToday != 200 || dto.Account.OrdersToday != 4 {
		t.Fatalf("account=%+v", dto.Account)
	}
	if len(dto.SKUs) != 1 || dto.SKUs[0].Revenue != 200 {
		t.Fatalf("skus=%+v", dto.SKUs)
	}
	if dto.Account.OrdersByStatus != nil {
		t.Fatalf("orders_by_status should be omitted without snapshot data: %+v", dto.Account.OrdersByStatus)
	}
}

func mustDateOnly(t *testing.T, iso string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", iso)
	if err != nil {
		t.Fatal(err)
	}
	return d.UTC()
}

func pgDate(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: true}
}

func mustNum(t *testing.T, v float64) pgtype.Numeric {
	t.Helper()
	var n pgtype.Numeric
	if err := n.Scan(fmt.Sprintf("%.2f", v)); err != nil {
		t.Fatal(err)
	}
	return n
}
