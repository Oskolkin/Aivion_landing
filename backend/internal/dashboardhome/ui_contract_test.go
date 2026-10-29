package dashboardhome

import (
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
)

func TestSellerPulseImportantNotesFromRiskDrivers(t *testing.T) {
	resp := &Response{
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{
				RiskDrivers: []RiskDriver{
					{ID: "driver-stock", Title: "Риск остатков", Reason: "4 SKU в критичной зоне.", Severity: "high"},
					{ID: "driver-advertising", Title: "Реклама", Reason: "Высокий ДРР.", Severity: "medium"},
				},
			},
		},
		SellerPulse: SellerPulse{
			Narrative: &SellerPulseNarrative{Status: "available", Headline: "Пульс"},
		},
	}
	finalizeSellerPulseUIContract(resp)
	if len(resp.SellerPulse.Narrative.ImportantNotes) < 2 {
		t.Fatalf("notes=%d", len(resp.SellerPulse.Narrative.ImportantNotes))
	}
}

func TestSellerPulseQuickActionsFromTopActions(t *testing.T) {
	resp := &Response{
		AIDiagnostics: AIDiagnostics{
			TopActions: []TopActionItem{
				{Title: "Пополнить остатки", NextStep: "Открыть SKU", Priority: "high"},
				{Title: "Проверить рекламу", Priority: "medium"},
			},
		},
		SellerPulse: SellerPulse{
			Narrative: &SellerPulseNarrative{Status: "available", Headline: "Пульс"},
		},
	}
	finalizeSellerPulseUIContract(resp)
	if len(resp.SellerPulse.Narrative.QuickActions) < 2 {
		t.Fatalf("actions=%d", len(resp.SellerPulse.Narrative.QuickActions))
	}
}

func TestDeltaPctContractPercentNotRatio(t *testing.T) {
	ratio := -0.0997
	pct := ratioToPercent(&ratio)
	if pct > -5 || pct < -15 {
		t.Fatalf("pct=%v want about -9.97", pct)
	}
}

func TestWhatChangedMainConclusionNonGeneric(t *testing.T) {
	generic := "Ключевые изменения и активные сигналы по продажам, остаткам, рекламе и воронке."
	if !isGenericWhatChangedText(generic) {
		t.Fatal("expected generic detection")
	}
	wc := &dashboardNarrativeWhatChanged{
		Summary: generic,
		Groups: []dashboardNarrativeWhatChangedGroup{
			{Key: whatChangedKeyWorsened, Summary: "Выручка снизилась на 5,03%."},
		},
	}
	repairWhatChangedMainConclusion(wc, "AI видит снижение выручки и риски по остаткам.")
	if isGenericWhatChangedText(wc.MainConclusion) {
		t.Fatalf("main=%q", wc.MainConclusion)
	}
}

func TestHealthScoreBreakdownMatchesRiskDriversAfterSanitize(t *testing.T) {
	drivers := []RiskDriver{
		{ID: "driver-account-sales", Category: "sales", Severity: "medium", ImpactScore: 50, Title: "t", Reason: "r"},
		{ID: "driver-stock", Category: "stock", Severity: "high", ImpactScore: 70, Title: "t", Reason: "r"},
	}
	resp := &Response{SKUFocus: SKUFocus{StockRisk: []SKUFocusCard{{ProductName: "SKU A"}}}}
	sanitized := sanitizeRiskDriversForSellerFacing(drivers, resp)
	breakdown := computeHealthScoreBreakdown(sanitized)
	if len(breakdown.Drivers) != len(sanitized) {
		t.Fatalf("breakdown=%d drivers=%d", len(breakdown.Drivers), len(sanitized))
	}
}

func TestNoExcessiveFactIDsInSellerFacingResponse(t *testing.T) {
	facts := make([]string, 0, 50)
	for i := 0; i < 50; i++ {
		facts = append(facts, "sku.fact."+string(rune('a'+i%26)))
	}
	drivers := []RiskDriver{{
		ID: "driver-stock", Evidence: map[string]any{"fact_ids": facts},
	}}
	out := sanitizeRiskDriversForSellerFacing(drivers, &Response{})[0]
	if _, ok := out.Evidence["fact_ids"]; ok {
		t.Fatal("expected fact_ids stripped")
	}
	if out.Evidence["grounding_fact_count"] != 50 {
		t.Fatalf("count=%v", out.Evidence["grounding_fact_count"])
	}
}

func TestGenericCategoryNamesRejected(t *testing.T) {
	if !isGenericCategoryName("Category 17990001") {
		t.Fatal("expected generic")
	}
	if normalizeCategoryDisplayName("17990001", "Category 17990001") != "Товары для ванной" {
		t.Fatal("expected alias")
	}
}

func TestDashboardUIContractValidationCatchesMissingPulseBlocks(t *testing.T) {
	resp := &Response{
		SellerPulse: SellerPulse{
			Narrative: &SellerPulseNarrative{Status: "available", Headline: "ok"},
			KPIs:      []KPICard{{Key: "aov"}, {Key: "drr"}},
		},
		AIDiagnostics: AIDiagnostics{
			TopActions: []TopActionItem{{Title: "a"}, {Title: "b"}, {Title: "c"}},
		},
	}
	report := RunDashboardUIContractValidation(resp)
	if report.Summary.Critical == 0 {
		t.Fatal("expected critical issues")
	}
}

func TestBuildAOVSparkline7d(t *testing.T) {
	from := time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
	metrics := map[string]sparklineDayMetrics{
		"2026-06-30": {orders: 10, revenue: 1000, hasOrders: true, hasRevenue: true},
	}
	pts := buildAOVSparklineSeries(from, to, metrics)
	if len(pts) != 7 {
		t.Fatalf("len=%d", len(pts))
	}
	if !pts[6].HasData || pts[6].Value == nil || *pts[6].Value != 100 {
		t.Fatalf("last point=%+v", pts[6])
	}
}

func TestHighDRRCountUsesUnifiedThreshold(t *testing.T) {
	drr := 61.68
	if drr > analytics.AdEfficiencyHighDRRThresholdPct {
		t.Fatal("61.68 should not count as high DRR at 100% threshold")
	}
}
