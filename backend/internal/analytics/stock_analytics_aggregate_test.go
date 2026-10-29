package analytics

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestAggregateStockAnalytics_IDCNotSummed(t *testing.T) {
	rows := []dbgen.StockAnalyticsSnapshot{
		{Sku: 1, WarehouseID: 1, AvailableStockCount: 50, ExcessStockCount: 10, Idc: pgtype.Int4{Int32: 120, Valid: true}},
		{Sku: 1, WarehouseID: 2, AvailableStockCount: 50, ExcessStockCount: 10, Idc: pgtype.Int4{Int32: 120, Valid: true}},
	}
	agg := aggregateStockAnalyticsBySKU(rows)[1]
	if agg.idc == nil || *agg.idc != 120 {
		t.Fatalf("expected idc=120 (max, not sum), got %v", agg.idc)
	}
}

func TestAggregateStockAnalytics_ExcessShareInputs(t *testing.T) {
	rows := []dbgen.StockAnalyticsSnapshot{
		{Sku: 2, WarehouseID: 1, AvailableStockCount: 50, ExcessStockCount: 10},
		{Sku: 2, WarehouseID: 2, AvailableStockCount: 50, ExcessStockCount: 20},
	}
	agg := aggregateStockAnalyticsBySKU(rows)[2]
	share := ExcessStockShare(agg.excess, agg.available)
	assertRatio(t, share, 0.3)
}

func TestAggregateStockAnalytics_DaysWithoutSalesMax(t *testing.T) {
	rows := []dbgen.StockAnalyticsSnapshot{
		{Sku: 3, WarehouseID: 1, DaysWithoutSales: pgtype.Int4{Int32: 5, Valid: true}},
		{Sku: 3, WarehouseID: 2, DaysWithoutSales: pgtype.Int4{Int32: 20, Valid: true}},
	}
	agg := aggregateStockAnalyticsBySKU(rows)[3]
	if agg.daysWithoutSales == nil || *agg.daysWithoutSales != 20 {
		t.Fatalf("expected max days_without_sales=20, got %v", agg.daysWithoutSales)
	}
}

func TestAggregateStockAnalytics_TurnoverGradeDeterministic(t *testing.T) {
	rows := []dbgen.StockAnalyticsSnapshot{
		{Sku: 4, WarehouseID: 1, TurnoverGrade: pgtype.Text{String: "ACTUAL", Valid: true}},
		{Sku: 4, WarehouseID: 2, TurnoverGrade: pgtype.Text{String: "SURPLUS", Valid: true}},
	}
	agg := aggregateStockAnalyticsBySKU(rows)[4]
	if agg.turnoverGrade == nil || *agg.turnoverGrade != "SURPLUS" {
		t.Fatalf("expected worst grade SURPLUS, got %v", agg.turnoverGrade)
	}
}

func TestPickWorseTurnoverGrade_Stable(t *testing.T) {
	a := pickWorseTurnoverGrade("DEFICIT", "POPULAR")
	b := pickWorseTurnoverGrade("POPULAR", "DEFICIT")
	if a != "DEFICIT" || b != "DEFICIT" {
		t.Fatalf("expected DEFICIT both ways, got %q and %q", a, b)
	}
}
