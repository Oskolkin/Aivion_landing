package dashboardfacts

import (
	"fmt"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestBuildDashboardFacts_revenueAndOrdersKPIs(t *testing.T) {
	snap := testSnapshot(t, 42, "2025-03-10")
	facts, report, err := BuildDashboardFacts(snap, PackIdentity{PackID: "pack:42:2025-03-10:abc", PackHash: "abc"})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Valid {
		t.Fatalf("validation failed: %+v", report)
	}
	revenueKPI, ordersKPI := kpiByKey(facts.SellerPulse.KPIs, "revenue"), kpiByKey(facts.SellerPulse.KPIs, "orders")
	if revenueKPI == nil || ordersKPI == nil {
		t.Fatalf("kpis=%v", facts.SellerPulse.KPIs)
	}
	if revenueKPI.Value != "1500 ₽" {
		t.Fatalf("revenue value=%q", revenueKPI.Value)
	}
	if ordersKPI.Value != "12" {
		t.Fatalf("orders value=%q", ordersKPI.Value)
	}
	if facts.DashboardMetrics.Account.RevenueToday != 1500 {
		t.Fatalf("account revenue=%v", facts.DashboardMetrics.Account.RevenueToday)
	}
}

func TestBuildDashboardFacts_trendObservedDaysOnly(t *testing.T) {
	snap := testSnapshot(t, 42, "2025-03-10")
	facts, _, err := BuildDashboardFacts(snap, PackIdentity{PackID: "pack:42:2025-03-10:abc", PackHash: "abc"})
	if err != nil {
		t.Fatal(err)
	}
	if len(facts.SellerPulse.Trend7D) != 2 {
		t.Fatalf("trend days=%d want 2 observed days only", len(facts.SellerPulse.Trend7D))
	}
	for _, day := range facts.SellerPulse.Trend7D {
		if day.Revenue == nil || day.Orders == nil {
			t.Fatalf("missing observed values on %s", day.Date)
		}
	}
	if facts.SellerPulse.Trend7D[0].Date != "2025-03-09" || facts.SellerPulse.Trend7D[1].Date != "2025-03-10" {
		t.Fatalf("trend=%v", facts.SellerPulse.Trend7D)
	}
}

func TestBuildDashboardFacts_deterministicHash(t *testing.T) {
	snap := testSnapshot(t, 42, "2025-03-10")
	identity := PackIdentity{PackID: "pack:42:2025-03-10:deadbeef", PackHash: "deadbeef"}
	a, _, err := BuildDashboardFacts(snap, identity)
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := BuildDashboardFacts(snap, identity)
	if err != nil {
		t.Fatal(err)
	}
	if a.FactsHash == "" {
		t.Fatal("empty facts_hash")
	}
	if a.FactsHash != b.FactsHash {
		t.Fatalf("hash mismatch: %s vs %s", a.FactsHash, b.FactsHash)
	}
}

func kpiByKey(kpis []KPICardFact, key string) *KPICardFact {
	for i := range kpis {
		if kpis[i].Key == key {
			return &kpis[i]
		}
	}
	return nil
}

func testSnapshot(t *testing.T, sellerID int64, metricDate string) *analysisdata.AnalysisDataSnapshot {
	t.Helper()
	md := mustDate(t, metricDate)
	prev := md.AddDate(0, 0, -1)
	return &analysisdata.AnalysisDataSnapshot{
		SellerAccountID: sellerID,
		MetricDate:      pgtype.Date{Time: md, Valid: true},
		AccountMetrics: []dbgen.DailyAccountMetric{
			{SellerAccountID: sellerID, MetricDate: date(md), Revenue: mustNumeric(t, 1500), OrdersCount: 12},
			{SellerAccountID: sellerID, MetricDate: date(prev), Revenue: mustNumeric(t, 1000), OrdersCount: 8},
		},
		SKUMetrics: []dbgen.DailySkuMetric{
			{
				SellerAccountID: sellerID, MetricDate: date(md), OzonProductID: 9001,
				Revenue: mustNumeric(t, 1500), OrdersCount: 12, StockAvailable: 5,
			},
			// Previous day mirrors the account total so the snapshot reconciles
			// (SUM(sku) == account) under source reconciliation.
			{
				SellerAccountID: sellerID, MetricDate: date(prev), OzonProductID: 9001,
				Revenue: mustNumeric(t, 1000), OrdersCount: 8, StockAvailable: 5,
			},
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

func date(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: true}
}

func mustNumeric(t *testing.T, v float64) pgtype.Numeric {
	t.Helper()
	var n pgtype.Numeric
	if err := n.Scan(fmt.Sprintf("%.2f", v)); err != nil {
		t.Fatal(err)
	}
	return n
}
