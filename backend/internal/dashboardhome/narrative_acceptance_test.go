package dashboardhome

import (
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
)

func sampleNarrativeBuild(t *testing.T) (time.Time, analytics.DashboardMetricsDTO, Response, dashboardNarrativeBuildResult, []RiskDriver) {
	t.Helper()
	metricDate := mustRiskDate(t, "2026-05-21")
	dto := analytics.DashboardMetricsDTO{
		Account: analytics.DashboardAccountMetrics{
			RevenueToday:     120000,
			RevenueDayDelta:  analytics.DashboardDelta{Pct: riskFloatPtr(-0.11)},
			RevenueWeekDelta: analytics.DashboardDelta{Pct: riskFloatPtr(-0.08)},
			OrdersToday:      55,
			OrdersYesterday:  63,
			CancelsToday:     2,
			CancellationRate: riskFloatPtr(0.03),
		},
	}
	resp := Response{
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{WidgetBase: pendingWidget("")},
			AdEfficiency: AdEfficiencyWidget{Points: []AdEfficiencyPoint{
				{CampaignExternalID: 21, CampaignName: "Холодный трафик", Spend: 42000, Revenue: 0, Orders: 0, EfficiencySignal: "growth_without_result"},
			}},
		},
		SKUFocus: SKUFocus{
			StockRisk: []SKUFocusCard{}, Surplus: []SKUFocusCard{},
			OrderDrop: []SKUFocusCard{}, NoSalesWithStock: []SKUFocusCard{}, KeySKUs: []KeySkuCard{},
		},
	}
	drivers := buildRiskDrivers(metricDate, dto, &resp, nil, nil)
	score := computeHealthScoreBreakdown(drivers).FinalScore
	severity := computeHealthSeverity(score)
	build := buildDashboardNarrativeContext(metricDate, dto, &resp, drivers, score, severity, nil, nil, nil, nil)
	return metricDate, dto, resp, build, drivers
}

func sampleStoredNarrative(build dashboardNarrativeBuildResult, payload *dashboardNarrativePayload) *storedDashboardNarrative {
	return &storedDashboardNarrative{
		ID:                  "run-1",
		Status:              "completed",
		PromptVersion:       DashboardNarrativePromptVersion,
		ContextHash:         build.FactualContextHash,
		RenderedContextHash: build.ContextHash,
		RiskDriversHash:     build.RiskDriversHash,
		Payload:             payload,
	}
}

func TestRepairEmptyExpectedEffect(t *testing.T) {
	_, _, _, build, drivers := sampleNarrativeBuild(t)
	ctx := buildDashboardNarrativeValidationContext(drivers, build)
	payload := &dashboardNarrativePayload{
		Headline:           "Главный риск дня в рекламе.",
		ExecutiveSummary:   "Сильнее всего индекс давит реклама.",
		ImprovementSummary: "Индекс улучшится после сокращения рекламных расходов.",
		TopActions: []dashboardNarrativeTopAction{
			{
				DriverID:          "driver-advertising",
				SourceCandidateID: "driver-advertising-action",
				Priority:          "critical",
				Category:          "advertising",
				Title:             "Остановите неэффективную рекламу",
				Reason:            "Кампания тратит бюджет без заказов.",
				NextStep:          "Поставьте кампанию на паузу.",
				ExpectedEffect:    "",
			},
		},
	}
	repair := repairDashboardNarrativePayload(payload, ctx)
	if len(repair.HardErrors) > 0 {
		t.Fatalf("hard=%v", repair.HardErrors)
	}
	if repair.Payload.TopActions[0].ExpectedEffect != fallbackExpectedEffect("advertising") {
		t.Fatalf("expected_effect=%q", repair.Payload.TopActions[0].ExpectedEffect)
	}
}

func TestRepairInventedNumberInExpectedEffect(t *testing.T) {
	_, _, _, build, drivers := sampleNarrativeBuild(t)
	ctx := buildDashboardNarrativeValidationContext(drivers, build)
	payload := &dashboardNarrativePayload{
		Headline:           "Главный риск дня в рекламе.",
		ExecutiveSummary:   "Сильнее всего индекс давит реклама.",
		ImprovementSummary: "Индекс улучшится после сокращения рекламных расходов.",
		TopActions: []dashboardNarrativeTopAction{
			{
				DriverID:          "driver-advertising",
				SourceCandidateID: "driver-advertising-action",
				Priority:          "critical",
				Category:          "advertising",
				Title:             "Остановите неэффективную рекламу",
				Reason:            "Кампания тратит бюджет без заказов.",
				NextStep:          "Поставьте кампанию на паузу.",
				ExpectedEffect:    "Сэкономит 100 000 ₽ за неделю",
			},
		},
	}
	repair := repairDashboardNarrativePayload(payload, ctx)
	if repair.Payload.TopActions[0].ExpectedEffect == "Сэкономит 100 000 ₽ за неделю" {
		t.Fatalf("invented number must be replaced: %q", repair.Payload.TopActions[0].ExpectedEffect)
	}
}

func TestAcceptanceUnknownMetricRefRemoved(t *testing.T) {
	_, dto, resp, build, drivers := sampleNarrativeBuild(t)
	fallbackWhatChanged := buildFallbackWhatChangedPayload(build.Context.WhatChangedContext)
	payload := &dashboardNarrativePayload{
		Headline:           "Главный риск дня в рекламе.",
		ExecutiveSummary:   "Сильнее всего индекс давит реклама.",
		ImprovementSummary: "Индекс улучшится после сокращения рекламных расходов.",
		WhatChanged:        fallbackWhatChanged,
		TopActions: []dashboardNarrativeTopAction{
			{
				DriverID:             "driver-advertising",
				SourceCandidateID:    "driver-advertising-action",
				Priority:             "critical",
				Category:             "advertising",
				Title:                "Остановите неэффективную рекламу",
				Reason:               "Кампания тратит бюджет без заказов.",
				NextStep:             "Поставьте кампанию на паузу.",
				ExpectedEffect:       "Снизить риск рекламного перерасхода.",
				SupportingMetricRefs: []string{"ad_spend", "unknown_metric"},
			},
		},
	}
	outcome := acceptStoredDashboardNarrative(sampleStoredNarrative(build, payload), build, drivers, nil, nil, topActionsLimit)
	if !outcome.Applied {
		t.Fatalf("outcome=%+v", outcome)
	}
	if outcome.ValidationStatus != "completed_partial" {
		t.Fatalf("status=%q repairs=%v", outcome.ValidationStatus, outcome.Repair.RepairActions)
	}
	_ = dto
	_ = resp
}

func TestAcceptancePartialOneInvalidAction(t *testing.T) {
	_, _, _, build, drivers := sampleNarrativeBuild(t)
	fallbackWhatChanged := buildFallbackWhatChangedPayload(build.Context.WhatChangedContext)
	payload := &dashboardNarrativePayload{
		Headline:           "Главный риск дня в рекламе.",
		ExecutiveSummary:   "Сильнее всего индекс давит реклама.",
		ImprovementSummary: "Индекс улучшится после сокращения рекламных расходов.",
		WhatChanged:        fallbackWhatChanged,
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
			{
				DriverID:       "driver-unknown",
				Title:          "Плохое действие",
				Reason:         "Нет драйвера",
				NextStep:       "Ничего",
				ExpectedEffect: "Ничего",
			},
		},
	}
	outcome := acceptStoredDashboardNarrative(sampleStoredNarrative(build, payload), build, drivers, nil, nil, topActionsLimit)
	if !outcome.Applied {
		t.Fatalf("outcome=%+v", outcome)
	}
	if outcome.ValidationStatus != "completed_partial" {
		t.Fatalf("status=%q", outcome.ValidationStatus)
	}
	if outcome.Repair.AIActionsAccepted != 1 {
		t.Fatalf("accepted=%d", outcome.Repair.AIActionsAccepted)
	}
}

func TestFactualHashStaleOnMetricChange(t *testing.T) {
	metricDate := mustRiskDate(t, "2026-05-21")
	dto := analytics.DashboardMetricsDTO{
		Account: analytics.DashboardAccountMetrics{
			RevenueToday: 120000,
			OrdersToday:  55,
		},
	}
	resp := Response{AIDiagnostics: AIDiagnostics{AccountSummary: AccountSummaryWidget{WidgetBase: pendingWidget("")}}}
	buildOld := buildDashboardNarrativeContext(metricDate, dto, &resp, nil, 70, "attention", nil, nil, nil, nil)

	dtoChanged := dto
	dtoChanged.Account.RevenueToday = 90000
	buildNew := buildDashboardNarrativeContext(metricDate, dtoChanged, &resp, nil, 70, "attention", nil, nil, nil, nil)
	if buildOld.FactualContextHash == buildNew.FactualContextHash {
		t.Fatal("revenue change must change factual hash")
	}
}

func TestAcceptanceCompletedRepairedStatus(t *testing.T) {
	_, _, _, build, drivers := sampleNarrativeBuild(t)
	if len(drivers) < 2 {
		t.Fatalf("need at least 2 drivers, got %d", len(drivers))
	}
	limit := len(drivers)
	if limit > topActionsLimit {
		limit = topActionsLimit
	}
	topActions := make([]dashboardNarrativeTopAction, 0, limit)
	for i := 0; i < limit; i++ {
		driver := drivers[i]
		topActions = append(topActions, dashboardNarrativeTopAction{
			DriverID:          driver.ID,
			SourceCandidateID: "",
			Priority:          driver.Severity,
			Category:          topActionCategoryForDriver(driver.Category),
			Title:             driver.Title,
			Reason:            driver.Reason,
			NextStep:          "Проверьте сигнал и выберите следующий шаг.",
			ExpectedEffect:    "",
		})
	}
	payload := &dashboardNarrativePayload{
		Headline:           "Главный риск дня в рекламе.",
		ExecutiveSummary:   "Сильнее всего индекс давит реклама.",
		ImprovementSummary: "Индекс улучшится после сокращения рекламных расходов.",
		TopActions:         topActions,
	}
	outcome := acceptStoredDashboardNarrative(sampleStoredNarrative(build, payload), build, drivers, nil, nil, limit)
	if outcome.Repair.AIActionsAccepted != limit {
		t.Fatalf("accepted=%d want=%d received=%d partial=%v", outcome.Repair.AIActionsAccepted, limit, outcome.Repair.AIActionsReceived, outcome.Repair.Partial)
	}
	if len(outcome.TopActions) != limit {
		t.Fatalf("top_actions=%d want=%d fallback_filled=%d", len(outcome.TopActions), limit, outcome.FallbackActionsFilled)
	}
	if !outcome.Applied {
		t.Fatalf("applied=%v status=%q repairs=%v", outcome.Applied, outcome.ValidationStatus, outcome.Repair.RepairActions)
	}
	if outcome.ValidationStatus != "completed_repaired" && outcome.ValidationStatus != "completed_partial" {
		t.Fatalf("status=%q repairs=%v", outcome.ValidationStatus, outcome.Repair.RepairActions)
	}
	if !strings.Contains(strings.Join(outcome.Repair.RepairActions, ","), "expected_effect") {
		t.Fatalf("repairs=%v", outcome.Repair.RepairActions)
	}
}

func TestFactualHashIgnoresArrayOrderAndNilVsEmpty(t *testing.T) {
	metricDate := mustRiskDate(t, "2026-05-21")
	dto := analytics.DashboardMetricsDTO{
		Account: analytics.DashboardAccountMetrics{RevenueToday: 120000, OrdersToday: 55},
	}
	respA := newDashboardResponseShellForMetrics()
	respA.AIDiagnostics.AdEfficiency.Points = []AdEfficiencyPoint{
		{CampaignExternalID: 21, Spend: 42000, EfficiencySignal: "growth_without_result"},
		{CampaignExternalID: 7, Spend: 10000, EfficiencySignal: "ok"},
	}
	respB := newDashboardResponseShellForMetrics()
	respB.AIDiagnostics.AdEfficiency.Points = []AdEfficiencyPoint{
		{CampaignExternalID: 7, Spend: 10000, EfficiencySignal: "ok"},
		{CampaignExternalID: 21, Spend: 42000, EfficiencySignal: "growth_without_result"},
	}
	respEmptyNotes := newDashboardResponseShellForMetrics()
	respEmptyNotes.AIDiagnostics.AdEfficiency.Points = respA.AIDiagnostics.AdEfficiency.Points
	respEmptyNotes.DataQualityNotes = []string{}
	respNilNotes := newDashboardResponseShellForMetrics()
	respNilNotes.AIDiagnostics.AdEfficiency.Points = respA.AIDiagnostics.AdEfficiency.Points
	respNilNotes.DataQualityNotes = nil

	hashA := buildFactualContextHash(metricDate, dto, &respA, nil, 70, "attention", nil, nil, nil)
	hashB := buildFactualContextHash(metricDate, dto, &respB, nil, 70, "attention", nil, nil, nil)
	hashEmpty := buildFactualContextHash(metricDate, dto, &respEmptyNotes, nil, 70, "attention", nil, nil, nil)
	hashNil := buildFactualContextHash(metricDate, dto, &respNilNotes, nil, 70, "attention", nil, nil, nil)
	if hashA != hashB {
		t.Fatalf("array order must not affect factual hash: %q vs %q", hashA, hashB)
	}
	if hashEmpty != hashNil {
		t.Fatalf("nil vs empty notes must not affect factual hash: %q vs %q", hashEmpty, hashNil)
	}
}

func TestAcceptanceAllTopActionsInvalidFallsBack(t *testing.T) {
	_, _, _, build, drivers := sampleNarrativeBuild(t)
	payload := &dashboardNarrativePayload{
		Headline:           "Главный риск дня в рекламе.",
		ExecutiveSummary:   "Сильнее всего индекс давит реклама.",
		ImprovementSummary: "Индекс улучшится после сокращения рекламных расходов.",
		TopActions: []dashboardNarrativeTopAction{
			{DriverID: "driver-unknown", Title: "x", Reason: "y", NextStep: "z", ExpectedEffect: "w"},
		},
	}
	outcome := acceptStoredDashboardNarrative(sampleStoredNarrative(build, payload), build, drivers, nil, nil, topActionsLimit)
	if !outcome.Applied {
		t.Fatalf("summary should still apply: %+v", outcome)
	}
	if outcome.ValidationStatus != "completed_partial" {
		t.Fatalf("status=%q", outcome.ValidationStatus)
	}
	if outcome.FallbackActionsFilled == 0 {
		t.Fatal("expected fallback actions to fill slots")
	}
}

func TestAcceptanceUnrepairableNumberInReasonDropsAction(t *testing.T) {
	_, _, _, build, drivers := sampleNarrativeBuild(t)
	payload := &dashboardNarrativePayload{
		Headline:           "Главный риск дня в рекламе.",
		ExecutiveSummary:   "Сильнее всего индекс давит реклама.",
		ImprovementSummary: "Индекс улучшится после сокращения рекламных расходов.",
		TopActions: []dashboardNarrativeTopAction{
			{
				DriverID:          "driver-advertising",
				SourceCandidateID: "driver-advertising-action",
				Category:          "advertising",
				Title:             "Остановите неэффективную рекламу",
				Reason:            "Кампания увеличит продажи на 999999%",
				NextStep:          "Поставьте кампанию на паузу.",
				ExpectedEffect:    "Снизить риск рекламного перерасхода.",
			},
		},
	}
	outcome := acceptStoredDashboardNarrative(sampleStoredNarrative(build, payload), build, drivers, nil, nil, topActionsLimit)
	if outcome.Repair.AIActionsAccepted != 0 {
		t.Fatalf("invented promise should drop action, accepted=%d", outcome.Repair.AIActionsAccepted)
	}
	if !outcome.Applied {
		t.Fatalf("payload summaries should still apply: %+v", outcome)
	}
}

func TestAcceptGeneratedNarrativeRejectsEmptyPayload(t *testing.T) {
	t.Setenv("DASHBOARD_VALIDATION_MODE", DashboardValidationModePilotStructural)
	_, _, _, build, _ := sampleNarrativeBuild(t)
	accepted, _, _, hard := acceptGeneratedDashboardNarrative(&dashboardNarrativePayload{}, build)
	if accepted {
		t.Fatal("empty payload must not be accepted at generation time")
	}
	if len(hard) == 0 {
		t.Fatal("expected hard validation errors")
	}
}
