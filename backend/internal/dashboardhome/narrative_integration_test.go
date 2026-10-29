package dashboardhome

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func TestNarrativeGenerationResult_DevExitErrorOnFailedRow(t *testing.T) {
	result := &NarrativeGenerationResult{
		RunID:        "run-failed",
		Status:       "failed",
		ErrorMessage: "validation_failed",
		ValidationErrors: []string{
			"top_action_0_unknown_driver_id",
		},
	}
	if result.IsSuccess() {
		t.Fatal("failed row must not be success")
	}
	if err := result.DevExitError(); err == nil || !strings.Contains(err.Error(), "run-failed") {
		t.Fatalf("dev exit error=%v", err)
	}
}

func TestParseDashboardNarrativePayload_UnwrapsNestedSellerPulse(t *testing.T) {
	raw := `{
		"headline": "Заголовок дня по рекламе.",
		"executive_summary": "Сильнее всего индекс давит реклама с низкой отдачей.",
		"improvement_summary": "Индекс улучшится после сокращения рекламных расходов.",
		"seller_pulse_narrative": {
			"seller_pulse_narrative": {
				"tone": "stable",
				"headline": "Ключевые показатели стабильны.",
				"summary": "Продолжайте мониторинг продаж и рисков.",
				"important_notes": [{"text": "Критичных ухудшений не видно."}],
				"quick_actions": [{"label": "Продолжить мониторинг", "kind": "monitoring", "href": "/app/products"}]
			}
		}
	}`
	payload, err := parseDashboardNarrativePayload(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if payload.SellerPulseNarrative == nil || payload.SellerPulseNarrative.Headline == "" {
		t.Fatalf("seller pulse not unwrapped: %#v", payload.SellerPulseNarrative)
	}
	_, _, _, build, drivers := sampleNarrativeBuild(t)
	fallbackWhatChanged := buildFallbackWhatChangedPayload(build.Context.WhatChangedContext)
	payload.WhatChanged = fallbackWhatChanged
	payload.TopActions = []dashboardNarrativeTopAction{
		{
			DriverID:          "driver-advertising",
			SourceCandidateID: "driver-advertising-action",
			Priority:          "critical",
			Category:          "advertising",
			Title:             "Остановите неэффективную рекламу",
			Reason:            "Кампания тратит бюджет без заказов.",
			NextStep:          "Поставьте кампанию на паузу.",
			ExpectedEffect:    "Снизить риск рекламного перерасхода.",
		},
	}
	outcome := acceptStoredDashboardNarrative(sampleStoredNarrative(build, payload), build, drivers, nil, nil, topActionsLimit)
	if !outcome.Applied {
		t.Fatalf("outcome=%+v", outcome)
	}
}

func TestAcceptanceRenderedHashDriftIsCurrentWithMinorDrift(t *testing.T) {
	_, _, _, build, drivers := sampleNarrativeBuild(t)
	payload := minimalAcceptedNarrativePayload(build)
	stored := sampleStoredNarrative(build, payload)
	stored.RenderedContextHash = "old-rendered-hash"
	stored.RiskDriversHash = "old-rendered-hash"

	outcome := acceptStoredDashboardNarrative(stored, build, drivers, nil, nil, topActionsLimit)
	if !outcome.Applied {
		t.Fatalf("outcome=%+v", outcome)
	}
	if outcome.Freshness != "current_with_minor_drift" {
		t.Fatalf("freshness=%q", outcome.Freshness)
	}
}

func TestFactualHashIgnoresTechnicalIDChanges(t *testing.T) {
	metricDate := mustRiskDate(t, "2026-05-21")
	dto := analytics.DashboardMetricsDTO{
		Account: analytics.DashboardAccountMetrics{RevenueToday: 120000, OrdersToday: 55},
	}
	resp := newDashboardResponseShellForMetrics()
	drivers := buildRiskDrivers(metricDate, dto, &resp, nil, nil)

	alertsA := []dbgen.Alert{{ID: 10, Severity: "high", AlertType: "ad_weak_campaign_efficiency"}}
	alertsB := []dbgen.Alert{{ID: 99, Severity: "high", AlertType: "ad_weak_campaign_efficiency"}}
	recsA := []dbgen.Recommendation{{ID: 5, RecommendationType: "review_ad_spend"}}
	recsB := []dbgen.Recommendation{{ID: 88, RecommendationType: "review_ad_spend"}}
	poolA := []TopActionItem{{ID: "recommendation-5", Category: "advertising", Priority: "high", Title: "Проверить рекламу"}}
	poolB := []TopActionItem{{ID: "recommendation-88", Category: "advertising", Priority: "high", Title: "Проверить рекламу"}}

	hashA := buildFactualContextHash(metricDate, dto, &resp, drivers, 70, "attention", alertsA, poolA, recsA)
	hashB := buildFactualContextHash(metricDate, dto, &resp, drivers, 70, "attention", alertsB, poolB, recsB)
	if hashA != hashB {
		t.Fatalf("technical id changes must not affect semantic factual hash: %q vs %q", hashA, hashB)
	}
}

func TestAcceptanceMissingSellerPulseDoesNotFallbackAccountSummary(t *testing.T) {
	_, _, resp, build, drivers := sampleNarrativeBuild(t)
	resp.SellerPulse.KPIs = []KPICard{
		{Key: "revenue", Label: "Выручка", Value: "80 000 ₽"},
	}
	build.SellerPulseRequired = true
	payload := minimalAcceptedNarrativePayload(build)
	payload.SellerPulseNarrative = nil

	outcome := acceptStoredDashboardNarrative(sampleStoredNarrative(build, payload), build, drivers, nil, nil, topActionsLimit)
	if !outcome.Applied {
		t.Fatalf("account summary must still apply: %+v", outcome)
	}
	if outcome.ValidationStatus != "completed_partial" {
		t.Fatalf("status=%q repair=%v", outcome.ValidationStatus, outcome.Repair.RepairableErrors)
	}
}

func TestRepairUnknownSourceCandidateIDByDriverCategory(t *testing.T) {
	_, _, _, build, drivers := sampleNarrativeBuild(t)
	ctx := buildDashboardNarrativeValidationContext(drivers, build)
	payload := minimalAcceptedNarrativePayload(build)
	payload.TopActions = []dashboardNarrativeTopAction{
		{
			DriverID:          "driver-advertising",
			SourceCandidateID: "recommendation-999",
			Priority:          "critical",
			Category:          "advertising",
			Title:             "Остановите неэффективную рекламу",
			Reason:            "Кампания тратит бюджет без заказов.",
			NextStep:          "Поставьте кампанию на паузу.",
			ExpectedEffect:    "Снизить риск рекламного перерасхода.",
		},
	}
	repair := repairDashboardNarrativePayload(payload, ctx)
	if len(repair.HardErrors) > 0 {
		t.Fatalf("hard=%v", repair.HardErrors)
	}
	if repair.Payload.TopActions[0].SourceCandidateID != "driver-advertising-action" {
		t.Fatalf("candidate=%q repairs=%v", repair.Payload.TopActions[0].SourceCandidateID, repair.RepairActions)
	}
}

func minimalAcceptedNarrativePayload(build dashboardNarrativeBuildResult) *dashboardNarrativePayload {
	return &dashboardNarrativePayload{
		Headline:           "Главный риск дня в рекламе.",
		ExecutiveSummary:   "Сильнее всего индекс давит реклама.",
		ImprovementSummary: "Индекс улучшится после сокращения рекламных расходов.",
		WhatChanged:        buildFallbackWhatChangedPayload(build.Context.WhatChangedContext),
		TopActions: []dashboardNarrativeTopAction{
			{
				DriverID:          "driver-advertising",
				SourceCandidateID: "driver-advertising-action",
				Priority:          "critical",
				Category:          "advertising",
				Title:             "Остановите неэффективную рекламу",
				Reason:            "Кампания тратит бюджет без заказов.",
				NextStep:          "Поставьте кампанию на паузу.",
				ExpectedEffect:    "Снизить риск рекламного перерасхода.",
			},
		},
	}
}

func TestParseDashboardNarrativePayload_UnwrapDoesNotBreakFlatSellerPulse(t *testing.T) {
	flat, err := json.Marshal(sampleSellerPulseNarrativeForTests())
	if err != nil {
		t.Fatal(err)
	}
	raw := `{"headline":"h","executive_summary":"e e e","improvement_summary":"i","seller_pulse_narrative":` + string(flat) + `}`
	payload, err := parseDashboardNarrativePayload(raw)
	if err != nil || payload.SellerPulseNarrative == nil {
		t.Fatalf("parse flat seller pulse: err=%v payload=%#v", err, payload)
	}
	_ = time.Now()
}
