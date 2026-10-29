package dashboardhome

import (
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
)

func testWhatChangedContext() dashboardNarrativeWhatChangedContext {
	return dashboardNarrativeWhatChangedContext{
		CurrentMetricDate:  "2026-05-21",
		PreviousMetricDate: "2026-05-20",
		WorsenedFacts: []whatChangedFact{
			{
				ID: "revenue-down", Label: "Выручка", Direction: whatChangedKeyWorsened,
				CurrentValue: "101 721 ₽", PreviousValue: "122 566 ₽", DeltaAbs: "20 845 ₽",
			},
		},
		NewRiskFacts: []whatChangedFact{
			{
				ID: "ad-zero", Label: "Реклама", Direction: whatChangedKeyNewRisks,
				EntityName: "Холодный трафик", CurrentValue: "4 950 ₽",
			},
		},
		UnchangedRiskFacts: []whatChangedFact{
			{ID: "stock", Label: "Остатки", Direction: whatChangedKeyUnchangedRisks, Note: "сохраняются критичные риски"},
		},
		ImprovedFacts:  []whatChangedFact{},
		CampaignNames:  []string{"Холодный трафик"},
		AllowedNumbers: []string{"101 721 ₽", "122 566 ₽", "20 845 ₽", "4 950 ₽"},
	}
}

func testWhatChangedBundle() whatChangedValidationBundle {
	ctx := testWhatChangedContext()
	return buildWhatChangedValidationBundle(ctx, map[string]struct{}{
		"Холодный трафик": {},
	})
}

func testValidWhatChangedPayload() *dashboardNarrativeWhatChanged {
	return &dashboardNarrativeWhatChanged{
		Summary: "Относительно прошлой сводки сильнее проявились продажи и рекламные сигналы.",
		Groups: []dashboardNarrativeWhatChangedGroup{
			{
				Key: "worsened", Label: "Ухудшилось", Tone: "negative",
				Summary: "Главное ухудшение — продажи стали менее денежными.",
				Items: []dashboardNarrativeWhatChangedItem{
					{Label: "Выручка", Text: "Снизилась до 101 721 ₽, на 20 845 ₽ ниже прошлой сводки.", Highlights: []string{"101 721 ₽", "20 845 ₽"}},
				},
			},
			{
				Key: "new_risks", Label: "Новые риски", Tone: "warning",
				Summary: "Появились сигналы, которые могут быстро ухудшить результат.",
				Items: []dashboardNarrativeWhatChangedItem{
					{Label: "Реклама", Text: "«Холодный трафик» потратила 4 950 ₽ без заказов.", Highlights: []string{"4 950 ₽"}},
				},
			},
			{
				Key: "unchanged_risks", Label: "Активные проблемы", Tone: "neutral",
				Summary: "Часть проблем не новая, но продолжает давить на кабинет.",
				Items: []dashboardNarrativeWhatChangedItem{
					{Label: "Остатки", Text: "Сохраняются критичные риски у значимых товаров."},
				},
			},
			{
				Key: "improved", Label: "Улучшилось", Tone: "positive",
				Summary:   "Заметных улучшений по сравнению с прошлой сводкой не выявлено.",
				EmptyText: "Текущие риски пока не компенсируются улучшениями в продажах, остатках или рекламе.",
			},
		},
	}
}

func TestValidateWhatChangedPayload_HappyPath(t *testing.T) {
	errs := validateWhatChangedPayload(testValidWhatChangedPayload(), testWhatChangedBundle())
	if len(errs) > 0 {
		t.Fatalf("errs=%v", errs)
	}
}

func TestValidateWhatChangedPayload_AllGroupsPresent(t *testing.T) {
	payload := testValidWhatChangedPayload()
	if len(payload.Groups) != 4 {
		t.Fatalf("groups=%d", len(payload.Groups))
	}
}

func TestValidateWhatChangedPayload_RejectsLabelWithCount(t *testing.T) {
	payload := testValidWhatChangedPayload()
	payload.Groups[0].Label = "Ухудшилось · 3"
	errs := validateWhatChangedPayload(payload, testWhatChangedBundle())
	if len(errs) == 0 {
		t.Fatal("expected label count error")
	}
}

func TestValidateWhatChangedPayload_EmptyImprovedValid(t *testing.T) {
	payload := testValidWhatChangedPayload()
	errs := validateWhatChangedPayload(payload, testWhatChangedBundle())
	if len(errs) > 0 {
		t.Fatalf("empty improved should be valid: %v", errs)
	}
}

func TestValidateWhatChangedPayload_ActionVerbFails(t *testing.T) {
	payload := testValidWhatChangedPayload()
	payload.Groups[0].Summary = "Проверьте выручку и остановите рекламу."
	errs := validateWhatChangedPayload(payload, testWhatChangedBundle())
	if len(errs) == 0 {
		t.Fatal("expected action verb validation error")
	}
}

func TestValidateWhatChangedPayload_InventedNumberFails(t *testing.T) {
	payload := testValidWhatChangedPayload()
	payload.Groups[0].Items[0].Text = "Снизилась до 999 999 ₽."
	errs := validateWhatChangedPayload(payload, testWhatChangedBundle())
	if len(errs) == 0 {
		t.Fatal("expected invented number validation error")
	}
}

func TestValidateWhatChangedPayload_InvalidHighlightFails(t *testing.T) {
	payload := testValidWhatChangedPayload()
	payload.Groups[0].Items[0].Highlights = []string{"999 999 ₽"}
	errs := validateWhatChangedPayload(payload, testWhatChangedBundle())
	if len(errs) == 0 {
		t.Fatal("expected highlight validation error")
	}
}

func TestValidateWhatChangedPayload_UnknownCampaignNameFails(t *testing.T) {
	payload := testValidWhatChangedPayload()
	payload.Groups[1].Items[0].Text = "«Несуществующая кампания» потратила 4 950 ₽."
	errs := validateWhatChangedPayload(payload, testWhatChangedBundle())
	if len(errs) == 0 {
		t.Fatal("expected unknown campaign name error")
	}
}

func TestBuildFallbackWhatChangedPayload_KeepsContract(t *testing.T) {
	payload := buildFallbackWhatChangedPayload(testWhatChangedContext())
	errs := validateWhatChangedPayload(payload, testWhatChangedBundle())
	if len(errs) > 0 {
		t.Fatalf("fallback must satisfy contract: %v", errs)
	}
	improved := payload.Groups[3]
	if improved.Key != whatChangedKeyImproved {
		t.Fatalf("improved key=%s", improved.Key)
	}
	if !strings.Contains(improved.Summary, "не выявлено") {
		t.Fatalf("improved summary=%q", improved.Summary)
	}
}

func TestApplyWhatChangedToResponse_UsesNarrativeWhenValid(t *testing.T) {
	resp := &Response{AIDiagnostics: AIDiagnostics{}}
	payload := &dashboardNarrativePayload{WhatChanged: testValidWhatChangedPayload()}
	applyWhatChangedToResponse(resp, payload, testWhatChangedBundle(), true)
	if len(resp.AIDiagnostics.WhatChanged.Groups) != 4 {
		t.Fatalf("groups=%d", len(resp.AIDiagnostics.WhatChanged.Groups))
	}
	if resp.AIDiagnostics.WhatChanged.Groups[0].Label != "Ухудшилось" {
		t.Fatalf("label=%q", resp.AIDiagnostics.WhatChanged.Groups[0].Label)
	}
}

func TestBuildWhatChangedContext_FromMetrics(t *testing.T) {
	metricDate := mustNarrativeDate(t, "2026-05-21")
	dto := analytics.DashboardMetricsDTO{
		Account: analytics.DashboardAccountMetrics{
			RevenueToday:      101721,
			RevenueDayDelta:   analytics.DashboardDelta{Pct: riskFloatPtr(-0.17)},
			OrdersToday:       88,
			OrdersYesterday:   95,
			CancelsToday:      12,
			CancellationRate:  riskFloatPtr(0.136),
			AverageOrderValue: riskFloatPtr(1156),
		},
	}
	resp := &Response{
		AIDiagnostics: AIDiagnostics{
			AdEfficiency: AdEfficiencyWidget{Points: []AdEfficiencyPoint{
				{CampaignExternalID: 1, CampaignName: "Холодный трафик — без заказов", Spend: 4950, Orders: 0, Revenue: 0},
			}},
			StockCoverage: StockCoverageWidget{Segments: []StockCoverageSegment{
				{Key: "critical_0_3", SKUCount: 3},
			}},
			RiskDistribution: RiskDistributionWidget{Groups: []RiskGroupCount{
				{Severity: "critical", Count: 11},
				{Severity: "high", Count: 9},
			}},
		},
	}
	ctx := buildWhatChangedContext(metricDate, dto, resp, nil, &accountMetricsSnapshot{
		MetricDate: "2026-05-20",
		Revenue:    122566,
		Orders:     95,
		Cancels:    4,
	})
	if ctx.CurrentMetricDate != "2026-05-21" || ctx.PreviousMetricDate != "2026-05-20" {
		t.Fatalf("dates=%s/%s", ctx.CurrentMetricDate, ctx.PreviousMetricDate)
	}
	if len(ctx.WorsenedFacts) == 0 {
		t.Fatal("expected worsened facts")
	}
}

func TestNormalizeDashboardNarrativeWhatChanged_KeepsEmptyUnchangedRisks(t *testing.T) {
	p := &dashboardNarrativeWhatChanged{
		Summary: "Сводка по кабинету.",
		Groups: []dashboardNarrativeWhatChangedGroup{
			{
				Key: whatChangedKeyWorsened, Label: "Ухудшилось", Tone: "negative",
				Summary: "Заказы снизились.", Items: []dashboardNarrativeWhatChangedItem{{Label: "Заказы", Text: "−13%"}},
			},
			{
				Key: whatChangedKeyNewRisks, Label: "Новые риски", Tone: "warning",
				Summary: "Новый риск остатков.", Items: []dashboardNarrativeWhatChangedItem{{Label: "Остатки", Text: "5 SKU <2 шт"}},
			},
			{
				Key: whatChangedKeyUnchangedRisks, Label: "Активные проблемы", Tone: "neutral",
				EmptyText: "В этой группе значимых изменений не отмечено.",
				Items:     []dashboardNarrativeWhatChangedItem{},
			},
			{
				Key: whatChangedKeyImproved, Label: "Точки опоры", Tone: "positive",
				Summary: "ДРР в норме.", Items: []dashboardNarrativeWhatChangedItem{{Label: "ДРР", Text: "19%"}},
			},
		},
	}
	n := normalizeDashboardNarrativeWhatChanged(p)
	if len(n.Groups) != 4 {
		t.Fatalf("expected 4 groups after normalize, got %d", len(n.Groups))
	}
	if n.Groups[2].Key != whatChangedKeyUnchangedRisks {
		t.Fatalf("expected unchanged_risks at index 2, got %q", n.Groups[2].Key)
	}
}

func TestNormalizeDashboardNarrativeWhatChanged_ReordersGroups(t *testing.T) {
	p := &dashboardNarrativeWhatChanged{
		Summary: "Сводка.",
		Groups: []dashboardNarrativeWhatChangedGroup{
			{
				Key: whatChangedKeyImproved, Label: "Улучшилось", Tone: "positive",
				Summary:   "Заметных улучшений по сравнению с прошлой сводкой не выявлено.",
				EmptyText: "Текущие риски пока не компенсируются улучшениями в продажах, остатках или рекламе.",
				Items:     []dashboardNarrativeWhatChangedItem{},
			},
			{
				Key: whatChangedKeyWorsened, Label: "Ухудшилось", Tone: "negative",
				Summary: "Ухудшение.", Items: []dashboardNarrativeWhatChangedItem{{Label: "Выручка", Text: "101 721 ₽"}},
			},
			{
				Key: whatChangedKeyUnchangedRisks, Label: "Активные проблемы", Tone: "neutral",
				Summary: "Проблемы остаются актуальными.", Items: []dashboardNarrativeWhatChangedItem{},
			},
			{
				Key: whatChangedKeyNewRisks, Label: "Новые риски", Tone: "warning",
				Summary: "Новые риски.", Items: []dashboardNarrativeWhatChangedItem{},
			},
		},
	}
	n := normalizeDashboardNarrativeWhatChanged(p)
	if n.Groups[0].Key != whatChangedKeyWorsened || n.Groups[1].Key != whatChangedKeyNewRisks ||
		n.Groups[2].Key != whatChangedKeyUnchangedRisks || n.Groups[3].Key != whatChangedKeyImproved {
		t.Fatalf("order=%v %v %v %v", n.Groups[0].Key, n.Groups[1].Key, n.Groups[2].Key, n.Groups[3].Key)
	}
}

func TestValidateWhatChangedPayload_InvalidGroupKey(t *testing.T) {
	p := testValidWhatChangedPayload()
	p.Groups[0].Key = "not_allowed"
	p.Groups[0].Label = "X"
	p.Groups[0].Tone = "negative"
	errs := validateWhatChangedPayload(p, testWhatChangedBundle())
	if len(errs) == 0 {
		t.Fatal("expected validation errors for invalid key")
	}
}

func TestValidateWhatChangedPayload_InvalidTone(t *testing.T) {
	p := testValidWhatChangedPayload()
	p.Groups[0].Tone = "positive"
	errs := validateWhatChangedPayload(p, testWhatChangedBundle())
	if len(errs) == 0 {
		t.Fatal("expected tone mismatch")
	}
}

func TestValidateWhatChangedPayload_ImprovedSummaryPhraseRequired(t *testing.T) {
	p := testValidWhatChangedPayload()
	p.Groups[3].Summary = "Всё стабильно."
	p.Groups[3].EmptyText = "Текущие риски пока не компенсируются улучшениями в продажах, остатках или рекламе."
	errs := validateWhatChangedPayload(p, testWhatChangedBundle())
	if len(errs) == 0 {
		t.Fatal("expected improved summary phrase error")
	}
}

func TestValidateWhatChangedPayload_HighlightAsKnownCampaignName(t *testing.T) {
	p := testValidWhatChangedPayload()
	p.Groups[1].Items[0].Text = "Рекламная кампания без заказов."
	p.Groups[1].Items[0].Highlights = []string{"Холодный трафик"}
	errs := validateWhatChangedPayload(p, testWhatChangedBundle())
	if len(errs) > 0 {
		t.Fatalf("highlight as allowed campaign name should pass: %v", errs)
	}
}

func TestWhatChangedWidgetFromNarrative_CanonicalGroupOrder(t *testing.T) {
	payload := testValidWhatChangedPayload()
	for i, j := 0, len(payload.Groups)-1; i < j; i, j = i+1, j-1 {
		payload.Groups[i], payload.Groups[j] = payload.Groups[j], payload.Groups[i]
	}
	w := whatChangedWidgetFromNarrative(payload)
	if w.Groups[0].Key != whatChangedKeyWorsened || w.Groups[3].Key != whatChangedKeyImproved {
		t.Fatalf("widget order wrong: %s .. %s", w.Groups[0].Key, w.Groups[3].Key)
	}
}
