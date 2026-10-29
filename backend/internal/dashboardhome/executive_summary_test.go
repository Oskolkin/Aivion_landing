package dashboardhome

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
)

func TestNormalizeSellerFacingSentences_CapitalizesAfterPeriod(t *testing.T) {
	got := normalizeSellerFacingSentences("первая мысль. вторая мысль")
	want := "Первая мысль. Вторая мысль"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestApplyUnifiedAccountHealth_UsesAIExecutiveSummary(t *testing.T) {
	metricDate := mustRiskDate(t, "2026-05-21")
	dto := analytics.DashboardMetricsDTO{
		Account: analytics.DashboardAccountMetrics{
			RevenueToday:     120000,
			RevenueDayDelta:  analytics.DashboardDelta{Pct: riskFloatPtr(-0.11)},
			RevenueWeekDelta: analytics.DashboardDelta{Pct: riskFloatPtr(-0.08)},
			OrdersToday:      55,
			OrdersYesterday:  63,
		},
	}
	resp := Response{
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{WidgetBase: pendingWidget("")},
			AdEfficiency: AdEfficiencyWidget{Points: []AdEfficiencyPoint{
				{CampaignExternalID: 51, CampaignName: "Перегрев по ad_waste SKU", Spend: 295178, Revenue: 3473, Orders: 3, EfficiencySignal: "weak_efficiency"},
			}},
		},
		SKUFocus: SKUFocus{
			StockRisk: []SKUFocusCard{}, Surplus: []SKUFocusCard{},
			OrderDrop: []SKUFocusCard{}, NoSalesWithStock: []SKUFocusCard{}, KeySKUs: []KeySkuCard{},
		},
	}
	drivers := buildRiskDrivers(metricDate, dto, &resp, nil, nil)
	build := buildDashboardNarrativeContext(metricDate, dto, &resp, drivers, computeHealthScoreBreakdown(drivers).FinalScore, computeHealthSeverity(computeHealthScoreBreakdown(drivers).FinalScore), nil, nil, nil, nil)
	fallbackWhatChanged := buildFallbackWhatChangedPayload(build.Context.WhatChangedContext)
	aiExecutiveSummary := "Сильнее всего индекс давит реклама: кампания «Перегрев по ad_waste SKU» тратит бюджет почти без заказов. Сначала сократите расход, затем проверьте связку товаров и запросов."
	resp.AIDiagnostics.AccountSummary.storedNarrative = &storedDashboardNarrative{
		ID:                  "narrative-exec",
		Status:              "completed",
		PromptVersion:       DashboardNarrativePromptVersion,
		ContextHash:         build.FactualContextHash,
		RenderedContextHash: build.ContextHash,
		RiskDriversHash:     build.RiskDriversHash,
		Payload: &dashboardNarrativePayload{
			WhatChanged:        fallbackWhatChanged,
			Headline:           "Главный риск дня в рекламе.",
			ExecutiveSummary:   aiExecutiveSummary,
			ImprovementSummary: "Индекс улучшится после сокращения рекламных расходов без результата.",
			HealthReasons: []dashboardNarrativeHealthReason{
				{
					DriverID: "driver-advertising",
					Category: "advertising",
					Title:    "Реклама тратит бюджет без результата",
					Reason:   "Кампания «Перегрев по ad_waste SKU» уже тратит деньги без заказов и выручки.",
				},
			},
			TopActions: []dashboardNarrativeTopAction{
				{
					DriverID:             "driver-advertising",
					SourceCandidateID:    "driver-advertising-action",
					Priority:             "critical",
					Category:             "advertising",
					Title:                "Сократите расход по кампании без заказов",
					Reason:               "Кампания «Перегрев по ad_waste SKU» продолжает расход без результата.",
					NextStep:             "Сначала поставьте кампанию на паузу. Затем проверьте запросы и товары в ней: если после корректировки заказов не появится, не возвращайте бюджет.",
					ExpectedEffect:       "Снизить риск рекламного перерасхода.",
					SupportingMetricRefs: []string{"ad_spend"},
				},
			},
			SellerPulseNarrative: sampleSellerPulseNarrativeForTests(),
		},
	}

	applyUnifiedAccountHealth(metricDate, dto, &resp, nil, nil, nil, nil, nil)

	if resp.AIDiagnostics.AccountSummary.NarrativeSource != "ai" {
		t.Fatalf("source=%q", resp.AIDiagnostics.AccountSummary.NarrativeSource)
	}
	if resp.AIDiagnostics.AccountSummary.ExecutiveSummary != aiExecutiveSummary {
		t.Fatalf("executive_summary=%q want AI text", resp.AIDiagnostics.AccountSummary.ExecutiveSummary)
	}
	if strings.Contains(resp.AIDiagnostics.AccountSummary.ExecutiveSummary, "перегрев по ad_waste sku") {
		t.Fatalf("campaign casing was lowered: %q", resp.AIDiagnostics.AccountSummary.ExecutiveSummary)
	}
	if !strings.Contains(resp.AIDiagnostics.AccountSummary.ExecutiveSummary, "Перегрев по ad_waste SKU") {
		t.Fatalf("campaign name lost: %q", resp.AIDiagnostics.AccountSummary.ExecutiveSummary)
	}
}

func TestApplyUnifiedAccountHealth_FallbackExecutiveSummaryWhenValidationFailed(t *testing.T) {
	metricDate := mustRiskDate(t, "2026-05-21")
	dto := analytics.DashboardMetricsDTO{
		Account: analytics.DashboardAccountMetrics{
			RevenueToday:    120000,
			RevenueDayDelta: analytics.DashboardDelta{Pct: riskFloatPtr(-0.11)},
			OrdersToday:     55,
			OrdersYesterday: 63,
		},
	}
	resp := Response{
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{WidgetBase: pendingWidget("")},
			AdEfficiency: AdEfficiencyWidget{Points: []AdEfficiencyPoint{
				{CampaignExternalID: 51, CampaignName: "Перегрев по ad_waste SKU", Spend: 295178, Revenue: 3473, Orders: 3, EfficiencySignal: "weak_efficiency"},
			}},
		},
		SKUFocus: SKUFocus{
			StockRisk: []SKUFocusCard{}, Surplus: []SKUFocusCard{},
			OrderDrop: []SKUFocusCard{}, NoSalesWithStock: []SKUFocusCard{}, KeySKUs: []KeySkuCard{},
		},
	}
	drivers := buildRiskDrivers(metricDate, dto, &resp, nil, nil)
	build := buildDashboardNarrativeContext(metricDate, dto, &resp, drivers, computeHealthScoreBreakdown(drivers).FinalScore, computeHealthSeverity(computeHealthScoreBreakdown(drivers).FinalScore), nil, nil, nil, nil)
	aiOnlySummary := "Только AI executive summary без остальных обязательных полей."
	resp.AIDiagnostics.AccountSummary.storedNarrative = &storedDashboardNarrative{
		ID:                  "narrative-invalid",
		Status:              "completed",
		PromptVersion:       DashboardNarrativePromptVersion,
		ContextHash:         build.FactualContextHash,
		RenderedContextHash: build.ContextHash,
		RiskDriversHash:     build.RiskDriversHash,
		Payload: &dashboardNarrativePayload{
			Headline:           "Headline",
			ExecutiveSummary:   aiOnlySummary,
			ImprovementSummary: "Improvement summary for validation.",
			WhatChanged:        testValidWhatChangedPayload(),
		},
	}

	applyUnifiedAccountHealth(metricDate, dto, &resp, nil, nil, nil, nil, nil)

	if resp.AIDiagnostics.AccountSummary.NarrativeSource != "ai" {
		t.Fatalf("source=%q", resp.AIDiagnostics.AccountSummary.NarrativeSource)
	}
	if resp.AIDiagnostics.AccountSummary.ExecutiveSummary != aiOnlySummary {
		t.Fatalf("usable AI executive_summary should apply: %q", resp.AIDiagnostics.AccountSummary.ExecutiveSummary)
	}
	if resp.AIDiagnostics.AccountSummary.ValidationStatus != "completed_partial" {
		t.Fatalf("validation_status=%q", resp.AIDiagnostics.AccountSummary.ValidationStatus)
	}
}

func TestFallbackExecutiveSummary_NotMechanicalReasonGlue(t *testing.T) {
	drivers := []RiskDriver{{
		ID:            "driver-advertising",
		Category:      "advertising",
		CategoryLabel: "Реклама",
		Severity:      "critical",
		ImpactScore:   98,
		Title:         "Слабая отдача рекламы",
		Reason:        "Кампания «Перегрев по ad_waste SKU» продолжает расходовать бюджет без заказов.",
	}}
	summary := buildRiskExecutiveSummary(drivers)
	if strings.Contains(summary, "продолжает расходовать бюджет") {
		t.Fatalf("fallback looks like glued driver.reason: %q", summary)
	}
	if strings.HasPrefix(summary, "Индекс сейчас сильнее всего снижают:") {
		t.Fatalf("fallback uses legacy glue prefix: %q", summary)
	}
	if !strings.Contains(summary, "Реклама") {
		t.Fatalf("fallback should mention risk direction, got %q", summary)
	}
}

func TestParseDashboardNarrativePayload_PreservesExecutiveSummaryInValidatedJSON(t *testing.T) {
	raw := `{
	  "headline":"Главный риск дня в рекламе.",
	  "executive_summary":"первая мысль. вторая мысль с «Перегрев по ad_waste SKU».",
	  "improvement_summary":"Индекс улучшится после сокращения рекламных расходов.",
	  "health_reasons":[],
	  "top_actions":[]
	}`
	payload, err := parseDashboardNarrativePayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(payload.ExecutiveSummary, "Первая мысль. Вторая") {
		t.Fatalf("executive_summary=%q", payload.ExecutiveSummary)
	}
	if !strings.Contains(payload.ExecutiveSummary, "Перегрев по ad_waste SKU") {
		t.Fatalf("campaign casing lost: %q", payload.ExecutiveSummary)
	}
	validated, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var stored map[string]any
	if err := json.Unmarshal(validated, &stored); err != nil {
		t.Fatal(err)
	}
	if stored["executive_summary"] != payload.ExecutiveSummary {
		t.Fatalf("validated_payload executive_summary=%v", stored["executive_summary"])
	}
}

func TestDashboardNarrativeContractExample_AIAndHomePayload(t *testing.T) {
	rawAI := `{
	  "headline": "Главный риск дня в рекламе.",
	  "executive_summary": "Сильнее всего индекс давит реклама: кампания «Перегрев по ad_waste SKU» тратит бюджет почти без заказов. Сначала сократите расход, затем проверьте связку товаров и запросов.",
	  "improvement_summary": "Индекс улучшится после сокращения рекламных расходов без результата.",
	  "health_reasons": [
	    {
	      "driver_id": "driver-advertising",
	      "category": "advertising",
	      "title": "Реклама тратит бюджет без результата",
	      "reason": "Кампания «Перегрев по ad_waste SKU» уже тратит деньги без заказов и выручки."
	    }
	  ],
	  "top_actions": [
	    {
	      "driver_id": "driver-advertising",
	      "source_candidate_id": "driver-advertising-action",
	      "priority": "critical",
	      "category": "advertising",
	      "title": "Сократите расход по кампании без заказов",
	      "reason": "Кампания «Перегрев по ad_waste SKU» продолжает расход без результата.",
	      "next_step": "Сначала поставьте кампанию на паузу. Затем проверьте запросы и товары в ней.",
	      "expected_effect": "Снизить риск рекламного перерасхода.",
	      "supporting_metric_refs": ["ad_spend"]
	    }
	  ],
	  "data_quality_notes": []
	}`
	payload, err := parseDashboardNarrativePayload(rawAI)
	if err != nil {
		t.Fatal(err)
	}
	validatedPayload, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	metricDate := mustRiskDate(t, "2026-05-21")
	dto := analytics.DashboardMetricsDTO{
		Account: analytics.DashboardAccountMetrics{
			RevenueToday:    120000,
			RevenueDayDelta: analytics.DashboardDelta{Pct: riskFloatPtr(-0.11)},
			OrdersToday:     55,
			OrdersYesterday: 63,
		},
	}
	resp := Response{
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{WidgetBase: pendingWidget("")},
			AdEfficiency: AdEfficiencyWidget{Points: []AdEfficiencyPoint{
				{CampaignExternalID: 51, CampaignName: "Перегрев по ad_waste SKU", Spend: 295178, Revenue: 3473, Orders: 3, EfficiencySignal: "weak_efficiency"},
			}},
		},
		SKUFocus: SKUFocus{
			StockRisk: []SKUFocusCard{}, Surplus: []SKUFocusCard{},
			OrderDrop: []SKUFocusCard{}, NoSalesWithStock: []SKUFocusCard{}, KeySKUs: []KeySkuCard{},
		},
	}
	drivers := buildRiskDrivers(metricDate, dto, &resp, nil, nil)
	build := buildDashboardNarrativeContext(metricDate, dto, &resp, drivers, computeHealthScoreBreakdown(drivers).FinalScore, computeHealthSeverity(computeHealthScoreBreakdown(drivers).FinalScore), nil, nil, nil, nil)
	resp.AIDiagnostics.AccountSummary.storedNarrative = &storedDashboardNarrative{
		ID:                  "narrative-contract",
		Status:              "completed",
		PromptVersion:       DashboardNarrativePromptVersion,
		ContextHash:         build.FactualContextHash,
		RenderedContextHash: build.ContextHash,
		RiskDriversHash:     build.RiskDriversHash,
		Payload:             payload,
	}
	applyUnifiedAccountHealth(metricDate, dto, &resp, nil, nil, nil, nil, nil)

	homePayload, err := json.Marshal(map[string]any{
		"account_summary": map[string]any{
			"executive_summary": resp.AIDiagnostics.AccountSummary.ExecutiveSummary,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("AI raw output:\n%s", rawAI)
	t.Logf("validated_payload:\n%s", string(validatedPayload))
	t.Logf("dashboard/home account_summary.executive_summary:\n%s", string(homePayload))
}
