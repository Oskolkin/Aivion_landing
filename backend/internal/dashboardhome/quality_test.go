package dashboardhome

import (
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
)

func TestBuildPartialDataQualityNotes_funnelOnly(t *testing.T) {
	notes := buildPartialDataQualityNotes([]string{partialReasonSalesFunnel})
	if len(notes) != 1 {
		t.Fatalf("notes=%v", notes)
	}
	if notes[0] == "" {
		t.Fatal("expected non-empty note")
	}
	for _, forbidden := range []string{"рекламы", "остатков"} {
		if containsFold(notes[0], forbidden) && !containsFold(notes[0], "воронк") {
			t.Fatalf("generic ads/stock mention in funnel-only note: %q", notes[0])
		}
	}
	if !containsFold(notes[0], "воронк") {
		t.Fatalf("expected funnel mention: %q", notes[0])
	}
}

func TestBuildAccountTrend7D_ordersMatchAccountMetrics(t *testing.T) {
	from := mustDate(t, "2026-05-19")
	to := mustDate(t, "2026-05-21")
	days := []accountMetricDay{
		{Date: "2026-05-19", Revenue: 100, Orders: 2},
		{Date: "2026-05-21", Revenue: 500, Orders: 3},
	}
	trend := buildAccountTrend7D(from, to, days)
	var anchor *TrendDayPoint
	for i := range trend {
		if trend[i].Date == "2026-05-21" {
			anchor = &trend[i]
			break
		}
	}
	if anchor == nil || anchor.Orders == nil {
		t.Fatal("missing trend point for anchor date")
	}
	if *anchor.Orders != 3 {
		t.Fatalf("orders=%d want 3", *anchor.Orders)
	}
	if anchor.Revenue == nil || *anchor.Revenue != 500 {
		t.Fatalf("revenue=%v want 500", anchor.Revenue)
	}
}

func TestAccountOrdersForDate(t *testing.T) {
	days := []accountMetricDay{{Date: "2026-05-21", Orders: 4, Revenue: 10}}
	ord, ok := accountOrdersForDate(days, mustDate(t, "2026-05-21"))
	if !ok || ord != 4 {
		t.Fatalf("ord=%d ok=%v", ord, ok)
	}
}

func TestAiSummaryContradictsAdEfficiency(t *testing.T) {
	points := []AdEfficiencyPoint{{Spend: 1200, CampaignName: "Test"}}
	if !aiSummaryContradictsAdEfficiency("Рекламные кампании не активны", "", points) {
		t.Fatal("expected contradiction")
	}
	if aiSummaryContradictsAdEfficiency("Рост выручки", "Расходы на рекламу выросли", points) {
		t.Fatal("unexpected contradiction")
	}
	if aiSummaryContradictsAdEfficiency("Рекламные кампании не активны", "", nil) {
		t.Fatal("no spend should not contradict")
	}
}

func TestIsReplenishmentStockRisk_matchesServiceCodes(t *testing.T) {
	if !isReplenishmentStockRisk(analytics.ReplenishmentSKUItem{DepletionRisk: "critical_out_of_stock"}) {
		t.Fatal("critical_out_of_stock")
	}
	if !isReplenishmentStockRisk(analytics.ReplenishmentSKUItem{ReplenishmentPriority: "high"}) {
		t.Fatal("high priority")
	}
	if isReplenishmentStockRisk(analytics.ReplenishmentSKUItem{DepletionRisk: "critical", ReplenishmentPriority: "critical"}) {
		t.Fatal("legacy strings must not match")
	}
}

func TestIsDashboardStockRiskSKU_fromCoverageBuckets(t *testing.T) {
	b := "critical_shortage"
	doc := 2.0
	if !isDashboardStockRiskSKU(analytics.DashboardSKUMetricsItem{
		StockStatusBucket: &b,
		StockAvailable:    5,
		DaysOfCover:       &doc,
	}) {
		t.Fatal("critical_shortage with stock and low cover")
	}
	if isDashboardStockRiskSKU(analytics.DashboardSKUMetricsItem{StockAvailable: 0}) {
		t.Fatal("zero stock must not qualify for stock focus")
	}
	high := 20.0
	if isDashboardStockRiskSKU(analytics.DashboardSKUMetricsItem{StockAvailable: 10, DaysOfCover: &high}) {
		t.Fatal("high coverage must not qualify")
	}
}

func containsFold(s, sub string) bool {
	return len(s) >= len(sub) && (sub == "" || indexFold(s, sub) >= 0)
}

func indexFold(s, sub string) int {
	ls, lsub := []rune(s), []rune(sub)
outer:
	for i := 0; i+len(lsub) <= len(ls); i++ {
		for j := range lsub {
			a, b := ls[i+j], lsub[j]
			if a|32 != b|32 && a != b {
				continue outer
			}
		}
		return i
	}
	return -1
}

func mustDate(t *testing.T, iso string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", iso)
	if err != nil {
		t.Fatal(err)
	}
	return d.UTC()
}
