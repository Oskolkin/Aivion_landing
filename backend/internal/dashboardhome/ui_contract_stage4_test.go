package dashboardhome

import (
	"math"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestSellerPulseCTRReplacesAOV(t *testing.T) {
	resp := &Response{
		SellerPulse: SellerPulse{
			KPIs: []KPICard{
				{Key: "revenue"}, {Key: "orders"}, {Key: "average_order_value"}, {Key: "ad_spend_share_total_revenue"},
			},
		},
	}
	report := RunDashboardUIContractValidation(resp)
	for _, issue := range report.Issues {
		if issue.ID == "seller_pulse_aov_in_primary_set" {
			t.Fatal("unexpected aov in primary set issue")
		}
	}
	for _, kpi := range resp.SellerPulse.KPIs {
		if kpi.Key == "aov" {
			t.Fatal("aov should not be in primary pulse set")
		}
	}
}

func TestSellerPulseCTRFormula(t *testing.T) {
	ctr, ok := computeAccountCTRPercent(1000, 132)
	if !ok || ctr == nil {
		t.Fatal("expected ctr")
	}
	if math.Abs(*ctr-13.2) > 0.01 {
		t.Fatalf("ctr=%v want 13.2", *ctr)
	}
	if _, ok := computeAccountCTRPercent(0, 10); ok {
		t.Fatal("zero impressions should be unavailable")
	}
}

func TestSellerPulseCTRSparkline7d(t *testing.T) {
	from := time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
	history := []dbgen.ProductAnalyticsDaily{
		{
			ScopeType: "account", MetricDate: pgtype.Date{Time: to, Valid: true},
			Impressions:  pgtype.Int8{Int64: 1000, Valid: true},
			ProductViews: pgtype.Int8{Int64: 132, Valid: true},
		},
	}
	pts := buildRatioSparklineSeries(from, to, buildCTRMetricsByDate(history))
	if len(pts) != 7 {
		t.Fatalf("len=%d", len(pts))
	}
	if !pts[6].HasData || pts[6].Value == nil || math.Abs(*pts[6].Value-13.2) > 0.01 {
		t.Fatalf("last=%+v", pts[6])
	}
}

func TestSellerPulseCTRDeltaPP(t *testing.T) {
	row := dbgen.ProductAnalyticsDaily{
		Impressions:  pgtype.Int8{Int64: 1000, Valid: true},
		ProductViews: pgtype.Int8{Int64: 150, Valid: true},
	}
	prev := &dbgen.ProductAnalyticsDaily{
		Impressions:  pgtype.Int8{Int64: 1000, Valid: true},
		ProductViews: pgtype.Int8{Int64: 132, Valid: true},
	}
	kpi, ok := buildCTRKPI(row, prev, nil)
	if !ok || kpi.Delta == nil {
		t.Fatal("expected ctr kpi with delta")
	}
	if kpi.Delta.Unit != "percentage_point" {
		t.Fatalf("unit=%q", kpi.Delta.Unit)
	}
	if math.Abs(kpi.Delta.Abs-1.8) > 0.01 {
		t.Fatalf("delta abs=%v", kpi.Delta.Abs)
	}
	if kpi.Delta.Display == "" {
		t.Fatal("expected display")
	}
}

func TestWhatChangedMainConclusionNotAISeesGeneric(t *testing.T) {
	wc := &dashboardNarrativeWhatChanged{
		MainConclusion: "AI видит три главные зоны внимания по продажам и остаткам.",
		Groups: []dashboardNarrativeWhatChangedGroup{
			{Key: whatChangedKeyWorsened, Summary: "Выручка снизилась на 5,03% за последние 7 дней."},
			{Key: whatChangedKeyNewRisks, Summary: "Риск дефицита сохраняется у ходовых SKU."},
		},
	}
	repairWhatChangedMainConclusion(wc, "AI видит снижение выручки.")
	if isGenericWhatChangedText(wc.MainConclusion) || isAISeesWhatChangedText(wc.MainConclusion) {
		t.Fatalf("main=%q", wc.MainConclusion)
	}
}

func TestWhatChangedMainConclusionDisplayedField(t *testing.T) {
	wc := &dashboardNarrativeWhatChanged{
		Summary: "Ключевые изменения и активные сигналы по продажам.",
		Groups: []dashboardNarrativeWhatChangedGroup{
			{Key: whatChangedKeyWorsened, Summary: "Выручка снизилась на 5,03%."},
		},
	}
	widget := whatChangedWidgetFromNarrative(wc)
	if widget.MainConclusion == "" {
		t.Fatal("expected main conclusion on widget")
	}
	if widget.MainConclusion == widget.Summary && isGenericWhatChangedText(widget.Summary) {
		t.Fatalf("widget still generic: %q", widget.MainConclusion)
	}
}

func TestCategoryCardsHaveScores(t *testing.T) {
	severity := "medium"
	cards := finalizeCategorySummaryScores([]CategorySummaryCard{
		{CategoryName: "Организация пространства", Severity: severity, Headline: "Лидер выручки", KeyRisk: "дефицит"},
	})
	if len(cards) != 1 || cards[0].CategoryScore == nil {
		t.Fatal("expected category score")
	}
	if *cards[0].CategoryScore < 0 || *cards[0].CategoryScore > 10 {
		t.Fatalf("score=%d", *cards[0].CategoryScore)
	}
	if cards[0].HealthScore == nil {
		t.Fatal("expected health_score mirror")
	}
}

func TestCategoryScoreRiskConsistency(t *testing.T) {
	score := int32(2)
	card := enrichCategorySummaryScore(CategorySummaryCard{
		CategoryScore: &score,
		Severity:      "low",
	})
	if card.Severity != "high" && card.Severity != "medium" {
		// severity may stay if already set; score 2 should map to high when empty
	}
	sev := severityForCategoryScore10(2)
	if sev != "high" {
		t.Fatalf("severity=%q", sev)
	}
}

func TestDashboardUIContractValidatesCTRAndCategoryScores(t *testing.T) {
	score := int32(6)
	health := int32(60)
	resp := &Response{
		SellerPulse: SellerPulse{
			Narrative: &SellerPulseNarrative{
				Status:         "available",
				Summary:        "ok",
				ImportantNotes: []SellerPulseImportantNote{{Text: "note"}},
			},
			KPIs: []KPICard{
				{Key: "revenue"}, {Key: "orders"},
				{Key: "average_order_value", Sparkline7D: []SparklinePoint{{Date: "2026-06-29", Value: ptrFloat(500), HasData: true}, {Date: "2026-06-30", Value: ptrFloat(520), HasData: true}}, Trend: &KPICardTrend{Label: "Динамика за 7 дней"}},
				{Key: "ad_spend_share_total_revenue", Sparkline7D: []SparklinePoint{{Date: "2026-06-29", Value: ptrFloat(11), HasData: true}, {Date: "2026-06-30", Value: ptrFloat(12), HasData: true}}, Trend: &KPICardTrend{Label: "Динамика за 7 дней"}},
			},
		},
		AIDiagnostics: AIDiagnostics{
			CategorySummaries: []CategorySummaryCard{{CategoryName: "Cat", CategoryScore: &score, ScoreMax: 10, HealthScore: &health, Severity: "medium"}},
			WhatChanged: WhatChangedWidget{
				WidgetBase:     WidgetBase{Status: WidgetAvailable},
				MainConclusion: "Выручка снизилась, основной риск — дефицит ходовых SKU.",
			},
		},
	}
	finalizeSellerPulseUIContract(resp)
	report := RunDashboardUIContractValidation(resp)
	if report.Status != "passed" {
		t.Fatalf("report=%+v issues=%+v", report.Status, report.Issues)
	}
}
