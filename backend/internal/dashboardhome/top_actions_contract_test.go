package dashboardhome

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
)

func TestTopActionsHomeResponseContract_CurrentAINarrative(t *testing.T) {
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
	actionPool := []TopActionItem{}
	drivers := buildRiskDrivers(metricDate, dto, &resp, nil, nil)
	score := computeHealthScoreBreakdown(drivers).FinalScore
	severity := computeHealthSeverity(score)
	build := buildDashboardNarrativeContext(metricDate, dto, &resp, drivers, score, severity, nil, actionPool, nil, nil)
	fallbackWhatChanged := buildFallbackWhatChangedPayload(build.Context.WhatChangedContext)

	resp.AIDiagnostics.AccountSummary.storedNarrative = &storedDashboardNarrative{
		ID:                  "narrative-contract-top-actions",
		Status:              "completed",
		PromptVersion:       DashboardNarrativePromptVersion,
		ContextHash:         build.FactualContextHash,
		RenderedContextHash: build.ContextHash,
		RiskDriversHash:     build.RiskDriversHash,
		Payload: &dashboardNarrativePayload{
			Headline:           "Главный риск дня в рекламе.",
			ExecutiveSummary:   "Сильнее всего индекс давит реклама: кампания «Холодный трафик» тратит бюджет без заказов.",
			ImprovementSummary: "Индекс улучшится после сокращения рекламных расходов без результата.",
			WhatChanged:        fallbackWhatChanged,
			HealthReasons: []dashboardNarrativeHealthReason{
				{
					DriverID: "driver-advertising",
					Category: "advertising",
					Title:    "Реклама тратит бюджет без результата",
					Reason:   "Кампания «Холодный трафик» уже тратит деньги без заказов и выручки.",
				},
			},
			TopActions: []dashboardNarrativeTopAction{
				{
					DriverID:             "driver-advertising",
					SourceCandidateID:    "driver-advertising-action",
					Priority:             "critical",
					Category:             "advertising",
					Title:                "Остановите неэффективную рекламу",
					Reason:               "Кампания «Холодный трафик» потратила бюджет без заказов.",
					NextStep:             "Сначала поставьте кампанию на паузу и проверьте запросы.",
					ExpectedEffect:       "Снизить риск рекламного перерасхода.",
					SupportingMetricRefs: []string{"ad_spend"},
				},
			},
			SellerPulseNarrative: sampleSellerPulseNarrativeForTests(),
		},
	}

	applyUnifiedAccountHealth(metricDate, dto, &resp, nil, nil, actionPool, nil, nil)

	raw, err := json.Marshal(map[string]any{
		"account_summary": resp.AIDiagnostics.AccountSummary,
		"top_actions":     resp.AIDiagnostics.TopActions,
	})
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}

	accountSummary, ok := payload["account_summary"].(map[string]any)
	if !ok {
		t.Fatal("account_summary must be object")
	}
	if accountSummary["narrative_source"] != "ai" {
		t.Fatalf("narrative_source=%v validation_status=%q", accountSummary["narrative_source"], resp.AIDiagnostics.AccountSummary.ValidationStatus)
	}
	if accountSummary["freshness"] != "current" {
		t.Fatalf("freshness=%v", accountSummary["freshness"])
	}

	topActions, ok := payload["top_actions"].([]any)
	if !ok || len(topActions) == 0 {
		t.Fatalf("top_actions=%#v", payload["top_actions"])
	}
	first, ok := topActions[0].(map[string]any)
	if !ok {
		t.Fatal("top action must be object")
	}
	for _, key := range []string{"source", "title", "reason", "next_step", "expected_effect", "supporting_metrics"} {
		if _, exists := first[key]; !exists {
			t.Fatalf("missing top_actions[0].%s in %#v", key, first)
		}
	}
	if first["source"] != "ai_summary" {
		t.Fatalf("source=%v", first["source"])
	}
	if first["expected_effect"] != "Снизить риск рекламного перерасхода." {
		t.Fatalf("expected_effect=%v", first["expected_effect"])
	}
	metrics, ok := first["supporting_metrics"].([]any)
	if !ok || len(metrics) == 0 {
		t.Fatalf("supporting_metrics=%#v", first["supporting_metrics"])
	}
}

func TestTopActionsHomeResponseContract_FallbackStaleExposesFallbackActions(t *testing.T) {
	metricDate := mustRiskDate(t, "2026-05-21")
	dto := analytics.DashboardMetricsDTO{
		Account: analytics.DashboardAccountMetrics{
			RevenueToday: 120000,
			OrdersToday:  55,
		},
	}
	resp := Response{
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{WidgetBase: pendingWidget("")},
			AdEfficiency: AdEfficiencyWidget{Points: []AdEfficiencyPoint{
				{CampaignExternalID: 21, CampaignName: "Холодный трафик", Spend: 42000, Revenue: 0, Orders: 0, EfficiencySignal: "growth_without_result"},
			}},
		},
	}

	applyUnifiedAccountHealth(metricDate, dto, &resp, nil, nil, []TopActionItem{
		{
			ID:       "fallback-action",
			Source:   "recommendation",
			Priority: "high",
			Category: "advertising",
			Title:    "Fallback action",
			Reason:   "Рекламные расходы требуют проверки.",
			NextStep: "Проверьте кампанию без заказов.",
		},
	}, nil, nil)

	raw, err := json.Marshal(map[string]any{
		"account_summary": resp.AIDiagnostics.AccountSummary,
		"top_actions":     resp.AIDiagnostics.TopActions,
	})
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}

	accountSummary := payload["account_summary"].(map[string]any)
	if accountSummary["narrative_source"] != "fallback" {
		t.Fatalf("narrative_source=%v", accountSummary["narrative_source"])
	}
	if accountSummary["freshness"] != "fallback" {
		t.Fatalf("freshness=%v", accountSummary["freshness"])
	}
	topActions, ok := payload["top_actions"].([]any)
	if !ok || len(topActions) == 0 {
		t.Fatalf("fallback_stale must still expose fallback top actions: %#v", payload["top_actions"])
	}
	for _, raw := range topActions {
		action := raw.(map[string]any)
		for _, key := range []string{"source", "title", "reason", "next_step", "expected_effect"} {
			if v, exists := action[key]; !exists || strings.TrimSpace(toStringValue(v)) == "" {
				t.Fatalf("fallback top action missing %s: %#v", key, action)
			}
		}
		if action["source"] == "ai_summary" {
			t.Fatalf("fallback_stale must not use ai_summary narrative source: %#v", action)
		}
	}
}

func TestTopActionsHomeResponseContract_FallbackUsesAISummaryRecommendedActions(t *testing.T) {
	metricDate := mustRiskDate(t, "2026-05-21")
	dto := analytics.DashboardMetricsDTO{
		Account: analytics.DashboardAccountMetrics{
			RevenueToday: 120000,
			OrdersToday:  55,
		},
	}
	resp := Response{
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{WidgetBase: pendingWidget("")},
			AdEfficiency: AdEfficiencyWidget{Points: []AdEfficiencyPoint{
				{CampaignExternalID: 21, CampaignName: "Холодный трафик", Spend: 42000, Revenue: 0, Orders: 0, EfficiencySignal: "growth_without_result"},
			}},
		},
	}

	aiSummaryActions := buildAISummaryFallbackTopActions([]ActionItem{
		{
			Priority:          "critical",
			Action:            "Остановите кампанию без заказов",
			Reason:            "Кампания тратит бюджет без результата.",
			ExpectedEffect:    "Сэкономить рекламный бюджет уже на этой неделе.",
			RelatedEntityType: "campaign",
			Source:            "ai_summary",
		},
	})

	applyUnifiedAccountHealth(metricDate, dto, &resp, nil, nil, nil, nil, nil, aiSummaryActions)

	if len(resp.AIDiagnostics.TopActions) == 0 {
		t.Fatal("fallback must include AI-summary recommended actions")
	}
	found := false
	for _, action := range resp.AIDiagnostics.TopActions {
		if action.Source == "ai_summary" && action.ExpectedEffect == "Сэкономить рекламный бюджет уже на этой неделе." {
			found = true
		}
	}
	if !found {
		t.Fatalf("AI-summary recommended action with expected_effect must participate in fallback: %#v", resp.AIDiagnostics.TopActions)
	}
}

func TestTopActionsHomeResponseContract_NoCandidatesReturnsEmpty(t *testing.T) {
	metricDate := mustRiskDate(t, "2026-05-21")
	dto := analytics.DashboardMetricsDTO{
		Account: analytics.DashboardAccountMetrics{
			RevenueToday:     120000,
			RevenueDayDelta:  analytics.DashboardDelta{Pct: riskFloatPtr(0.02)},
			RevenueWeekDelta: analytics.DashboardDelta{Pct: riskFloatPtr(0.01)},
			OrdersToday:      55,
			OrdersYesterday:  54,
		},
	}
	resp := Response{
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{WidgetBase: pendingWidget("")},
		},
		SKUFocus: SKUFocus{StockRisk: []SKUFocusCard{}, Surplus: []SKUFocusCard{}},
	}

	applyUnifiedAccountHealth(metricDate, dto, &resp, nil, nil, nil, nil, nil)

	if len(resp.AIDiagnostics.AccountSummary.RiskDrivers) != 0 {
		t.Fatalf("expected no risk drivers for healthy account, got %#v", resp.AIDiagnostics.AccountSummary.RiskDrivers)
	}
	if len(resp.AIDiagnostics.TopActions) != 0 {
		t.Fatalf("no candidates must yield empty top actions, got %#v", resp.AIDiagnostics.TopActions)
	}
}

func toStringValue(v any) string {
	s, _ := v.(string)
	return s
}
