package sourcerecon

import (
	"fmt"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestReconcileSnapshot_NilReturnsError(t *testing.T) {
	if _, err := ReconcileSnapshot(nil); err == nil {
		t.Fatal("expected error for nil snapshot")
	}
}

func TestReconcileSnapshot_ConsistentIsValid(t *testing.T) {
	snap := consistentSnapshot(t)
	report, err := ReconcileSnapshot(snap)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Valid {
		t.Fatalf("expected valid report, got errors=%+v", report.Errors)
	}
	if report.Hash == "" {
		t.Fatal("expected a non-empty hash")
	}
}

func TestReconcileSnapshot_DetectsMismatches(t *testing.T) {
	snap := consistentSnapshot(t)
	// Break the account↔SKU orders invariant and inject a foreign seller row.
	snap.AccountMetrics[0].OrdersCount = 99
	snap.SKUMetrics = append(snap.SKUMetrics, dbgen.DailySkuMetric{
		SellerAccountID: 777, MetricDate: snap.MetricDate, OzonProductID: 5,
		Revenue: mustNum(t, 10), OrdersCount: 1,
	})

	report, err := ReconcileSnapshot(snap)
	if err != nil {
		t.Fatal(err)
	}
	if report.Valid {
		t.Fatal("expected invalid report")
	}
	if !hasCode(report.Errors, "account_sku_orders_mismatch") {
		t.Fatalf("expected account_sku_orders_mismatch, got %+v", report.Errors)
	}
	if !hasCode(report.Errors, "foreign_seller_row") {
		t.Fatalf("expected foreign_seller_row, got %+v", report.Errors)
	}
}

func hasCode(issues []ReconciliationIssue, code string) bool {
	for _, i := range issues {
		if i.Code == code {
			return true
		}
	}
	return false
}

func consistentSnapshot(t *testing.T) *analysisdata.AnalysisDataSnapshot {
	t.Helper()
	md := mustDate(t, "2025-04-01")
	return &analysisdata.AnalysisDataSnapshot{
		SellerAccountID: 7,
		MetricDate:      pgtype.Date{Time: md, Valid: true},
		AccountMetrics: []dbgen.DailyAccountMetric{
			{SellerAccountID: 7, MetricDate: pgtype.Date{Time: md, Valid: true}, Revenue: mustNum(t, 300), OrdersCount: 6},
		},
		SKUMetrics: []dbgen.DailySkuMetric{
			{SellerAccountID: 7, MetricDate: pgtype.Date{Time: md, Valid: true}, OzonProductID: 501, Revenue: mustNum(t, 200), OrdersCount: 4, StockAvailable: 10},
			{SellerAccountID: 7, MetricDate: pgtype.Date{Time: md, Valid: true}, OzonProductID: 502, Revenue: mustNum(t, 100), OrdersCount: 2, StockAvailable: 5},
		},
	}
}

func mustDate(t *testing.T, iso string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", iso)
	if err != nil {
		t.Fatal(err)
	}
	return d.UTC()
}

func mustNum(t *testing.T, v float64) pgtype.Numeric {
	t.Helper()
	var n pgtype.Numeric
	if err := n.Scan(fmt.Sprintf("%.2f", v)); err != nil {
		t.Fatal(err)
	}
	return n
}
