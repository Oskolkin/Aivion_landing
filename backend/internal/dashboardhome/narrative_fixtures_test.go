package dashboardhome

import (
	"strings"
	"testing"
)

func TestFixtureNestedSellerPulseAccepted(t *testing.T) {
	runFixtureAcceptanceTest(t, "nested_seller_pulse", `{
		"headline": "Главный риск дня в рекламе.",
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
		},
		"top_actions": [{
			"driver_id": "driver-advertising",
			"source_candidate_id": "driver-advertising-action",
			"priority": "critical",
			"category": "advertising",
			"title": "Остановите неэффективную рекламу",
			"reason": "Кампания тратит бюджет без заказов.",
			"next_step": "Поставьте кампанию на паузу.",
			"expected_effect": "Снизить риск рекламного перерасхода."
		}]
	}`)
}

func TestFixtureMarkdownJSONAccepted(t *testing.T) {
	runFixtureAcceptanceTest(t, "markdown_json", "```json\n"+`{
		"headline": "Главный риск дня в рекламе.",
		"executive_summary": "Сильнее всего индекс давит реклама с низкой отдачей.",
		"improvement_summary": "Индекс улучшится после сокращения рекламных расходов.",
		"top_actions": [{
			"driver_id": "driver-advertising",
			"source_candidate_id": "driver-advertising-action",
			"priority": "critical",
			"category": "advertising",
			"title": "Остановите неэффективную рекламу",
			"reason": "Кампания тратит бюджет без заказов.",
			"next_step": "Поставьте кампанию на паузу.",
			"expected_effect": "Снизить риск рекламного перерасхода."
		}]
	}`+"\n```")
}

func TestFixtureUnknownMetricRefAccepted(t *testing.T) {
	runFixtureAcceptanceTest(t, "unknown_metric_ref", `{
		"headline": "Главный риск дня в рекламе.",
		"executive_summary": "Сильнее всего индекс давит реклама с низкой отдачей.",
		"improvement_summary": "Индекс улучшится после сокращения рекламных расходов.",
		"top_actions": [{
			"driver_id": "driver-advertising",
			"source_candidate_id": "driver-advertising-action",
			"priority": "critical",
			"category": "advertising",
			"title": "Остановите неэффективную рекламу",
			"reason": "Кампания тратит бюджет без заказов.",
			"next_step": "Поставьте кампанию на паузу.",
			"expected_effect": "Снизить риск рекламного перерасхода.",
			"supporting_metric_refs": ["ad_spend", "unknown_metric"]
		}]
	}`)
}

func TestFixtureForbiddenRecommendationPartialAccepted(t *testing.T) {
	_, _, _, build, drivers := sampleNarrativeBuild(t)
	payload := minimalAcceptedNarrativePayload(build)
	payload.TopActions = []dashboardNarrativeTopAction{
		{
			DriverID: "driver-advertising", SourceCandidateID: "driver-advertising-action", Category: "advertising",
			Title: "Снизьте цену на 30%", Reason: "Снизьте цену на 30% для роста.", NextStep: "Понизьте цену.",
			ExpectedEffect: "Увеличит продажи.",
		},
		{
			DriverID: "driver-advertising", SourceCandidateID: "driver-advertising-action", Category: "advertising",
			Title: "Повысьте цену", Reason: "Повысьте цену на 20%.", NextStep: "Поднимите цену.",
			ExpectedEffect: "Рост продаж.",
		},
		{
			DriverID: "driver-advertising", SourceCandidateID: "driver-advertising-action", Category: "advertising",
			Title: "Остановите неэффективную рекламу", Reason: "Кампания тратит бюджет без заказов.",
			NextStep: "Поставьте кампанию на паузу.", ExpectedEffect: "Снизить риск рекламного перерасхода.",
		},
	}
	outcome := acceptStoredDashboardNarrative(sampleStoredNarrative(build, payload), build, drivers, nil, nil, topActionsLimit)
	if !outcome.Applied {
		t.Fatalf("fixture forbidden recommendation: %+v", outcome)
	}
	if outcome.ValidationStatus != "completed_partial" {
		t.Fatalf("status=%q", outcome.ValidationStatus)
	}
	if len(outcome.TopActions) == 0 {
		t.Fatal("expected refilled top actions")
	}
}

func TestFixtureAllForbiddenActionsStillAccepted(t *testing.T) {
	_, _, _, build, drivers := sampleNarrativeBuild(t)
	payload := minimalAcceptedNarrativePayload(build)
	payload.TopActions = []dashboardNarrativeTopAction{
		{DriverID: "driver-advertising", SourceCandidateID: "driver-advertising-action", Category: "advertising",
			Title: "Снизьте цену", Reason: "Снизьте цену на 30%.", NextStep: "Понизьте цену.", ExpectedEffect: "Рост."},
		{DriverID: "driver-advertising", SourceCandidateID: "driver-advertising-action", Category: "advertising",
			Title: "Повысьте цену", Reason: "Повысьте цену на 20%.", NextStep: "Поднимите цену.", ExpectedEffect: "Рост."},
		{DriverID: "driver-advertising", SourceCandidateID: "driver-advertising-action", Category: "advertising",
			Title: "Ещё цена", Reason: "Снизьте цену снова.", NextStep: "Понизьте.", ExpectedEffect: "Рост."},
	}
	outcome := acceptStoredDashboardNarrative(sampleStoredNarrative(build, payload), build, drivers, nil, nil, topActionsLimit)
	if !outcome.Applied {
		t.Fatalf("all forbidden should still accept summary: %+v", outcome)
	}
	if outcome.FallbackActionsFilled == 0 {
		t.Fatal("expected deterministic refill")
	}
}

func runFixtureAcceptanceTest(t *testing.T, name string, raw string) {
	t.Helper()
	_, _, _, build, drivers := sampleNarrativeBuild(t)
	payload, repairs, err := parseDashboardNarrativePayloadWithRepair(raw)
	if err != nil {
		t.Fatalf("%s parse: %v repairs=%v", name, err, repairs)
	}
	payload.WhatChanged = buildFallbackWhatChangedPayload(build.Context.WhatChangedContext)
	accepted, _, repair, hard := acceptGeneratedDashboardNarrative(payload, build)
	if !accepted || len(hard) > 0 {
		t.Fatalf("%s generation accept failed accepted=%v hard=%v repair=%v", name, accepted, hard, repair)
	}
	outcome := acceptStoredDashboardNarrative(sampleStoredNarrative(build, payload), build, drivers, nil, nil, topActionsLimit)
	if !outcome.Applied {
		t.Fatalf("%s stored accept failed: %+v", name, outcome)
	}
	if strings.HasPrefix(outcome.ValidationStatus, "fallback") {
		t.Fatalf("%s must not fallback: %s", name, outcome.ValidationStatus)
	}
}
