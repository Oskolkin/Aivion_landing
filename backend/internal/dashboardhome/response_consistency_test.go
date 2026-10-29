package dashboardhome

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
)

func TestCancellationRateNarrativeDenominatorConsistent(t *testing.T) {
	dto := analytics.DashboardMetricsDTO{
		Account: analytics.DashboardAccountMetrics{
			OrdersToday:  247,
			CancelsToday: 7,
			CancellationRate: func() *float64 {
				v := 0.07
				return &v
			}(),
		},
	}
	driver, ok := buildCancellationsRiskDriver(mustRiskDate(t, "2026-06-25"), dto, nil, nil)
	if !ok {
		t.Fatal("expected cancellations driver")
	}
	rate, _ := evidenceFloat(driver.Evidence, "cancellation_rate")
	expected := 7.0 / 247.0 * 100
	if rate < expected-0.1 || rate > expected+0.1 {
		t.Fatalf("rate=%.2f expected~%.2f", rate, expected)
	}
	if !strings.Contains(driver.Reason, "7 отмен при 247 заказах") {
		t.Fatalf("reason=%q", driver.Reason)
	}
	if !strings.Contains(driver.Reason, formatPercent(rate)) {
		t.Fatalf("reason should include formatted rate: %q", driver.Reason)
	}
	issues := ValidateDashboardHomeResponseConsistency(&Response{
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{RiskDrivers: []RiskDriver{driver}},
		},
	})
	for _, issue := range issues {
		if issue.Code == "cancellation_rate_denominator_mismatch" {
			t.Fatalf("unexpected issue: %+v", issue)
		}
	}
}

func TestRiskDistributionMessageDoesNotContradictRiskDrivers(t *testing.T) {
	resp := Response{
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{
				RiskDrivers: []RiskDriver{{ID: "driver-advertising", Category: "advertising"}},
			},
			RiskDistribution: RiskDistributionWidget{
				WidgetBase: WidgetBase{
					Status:  WidgetNotEnoughData,
					Message: "Открытых рисков не обнаружено.",
				},
			},
		},
	}
	reconcileRiskDistributionMessage(&resp, resp.AIDiagnostics.AccountSummary.RiskDrivers)
	if strings.Contains(resp.AIDiagnostics.RiskDistribution.Message, "не обнаружено") {
		t.Fatalf("message still contradicts drivers: %q", resp.AIDiagnostics.RiskDistribution.Message)
	}
	issues := ValidateDashboardHomeResponseConsistency(&resp)
	found := false
	for _, issue := range issues {
		if issue.Code == "risk_distribution_contradicts_drivers" {
			found = true
		}
	}
	if found {
		t.Fatal("validator should not flag after reconcile")
	}
}

func TestAccountSummaryNotFallbackAfterChunksCompleted(t *testing.T) {
	_, dto, resp, build, drivers := sampleNarrativeBuild(t)
	payload := &dashboardNarrativePayload{
		Headline:           "Главный риск дня в рекламе.",
		ExecutiveSummary:   "Сильнее всего индекс давит реклама.",
		ImprovementSummary: "Индекс улучшится после сокращения рекламных расходов.",
		TopActions: []dashboardNarrativeTopAction{
			{
				DriverID:       "driver-advertising",
				Priority:       "critical",
				Category:       "advertising",
				Title:          "Остановите неэффективную рекламу",
				Reason:         "Кампания тратит бюджет без заказов.",
				NextStep:       "Поставьте кампанию на паузу.",
				ExpectedEffect: "Снизится давление на индекс.",
			},
		},
		SellerPulseNarrative: &SellerPulseNarrative{
			Status:   "ready",
			Headline: "День требует внимания к рекламе.",
		},
		AdEfficiencyNarrative: &AdEfficiencyNarrative{
			Status:  "available",
			Summary: "ДРР выше целевого на части кампаний.",
		},
	}
	stored := sampleStoredNarrative(build, payload)
	resp.AIDiagnostics.AccountSummary.storedNarrative = stored
	resp.SellerPulse = SellerPulse{
		KPIs: []KPICard{
			{Key: "revenue", Label: "Выручка", Value: "120 000 ₽"},
			{Key: "orders", Label: "Заказы", Value: "55"},
			{Key: "drr", Label: "ДРР", Value: "14%"},
		},
	}
	applyUnifiedAccountHealth(mustRiskDate(t, "2026-05-21"), dto, &resp, nil, nil, nil, nil, &build)
	summary := resp.AIDiagnostics.AccountSummary
	if summary.NarrativeSource != "ai" {
		t.Fatalf("narrative_source=%q", summary.NarrativeSource)
	}
	if summary.ValidationStatus == "fallback_stale" || summary.Freshness == "fallback" {
		t.Fatalf("freshness=%q validation=%q", summary.Freshness, summary.ValidationStatus)
	}
	if len(resp.AIDiagnostics.TopActions) == 0 {
		t.Fatal("expected top actions from narrative")
	}
	if ad := resp.AIDiagnostics.AdEfficiency.Narrative; ad == nil || ad.Status == "pending" {
		t.Fatalf("ad_efficiency narrative=%+v", ad)
	}
	_ = drivers
}

func TestSellerPulseNarrativeAvailableAfterChunksCompleted(t *testing.T) {
	_, dto, resp, build, _ := sampleNarrativeBuild(t)
	payload := &dashboardNarrativePayload{
		Headline:         "AI headline",
		ExecutiveSummary: "AI executive",
		SellerPulseNarrative: &SellerPulseNarrative{
			Tone:     "attention",
			Headline: "День требует внимания к рекламе.",
			Summary:  "Рекламные расходы давят на маржу.",
			ImportantNotes: []SellerPulseImportantNote{
				{Text: "ДРР выше комфортного диапазона.", MetricRefs: []string{"drr"}},
			},
			QuickActions: []SellerPulseQuickAction{
				{Label: "Проверить рекламу", Kind: "action", Href: "/app/actions?action=ad-1"},
			},
		},
	}
	stored := sampleStoredNarrative(build, payload)
	resp.AIDiagnostics.AccountSummary.storedNarrative = stored
	resp.SellerPulse = SellerPulse{
		KPIs: []KPICard{
			{Key: "revenue", Label: "Выручка", Value: "120 000 ₽"},
			{Key: "orders", Label: "Заказы", Value: "55"},
			{Key: "drr", Label: "ДРР", Value: "14%"},
		},
	}
	applySellerPulseNarrativeToResponse(&resp, &build)
	if resp.SellerPulse.Narrative == nil || resp.SellerPulse.Narrative.Status != "ready" {
		t.Fatalf("seller_pulse status=%v", resp.SellerPulse.Narrative)
	}
	_ = dto
}

func TestLegacyFallbackDoesNotOverwriteFreshChunksNarrative(t *testing.T) {
	_, dto, resp, legacyBuild, _ := sampleNarrativeBuild(t)
	datapackBuild := legacyBuild
	datapackBuild.FactualContextHash = "datapack-factual-hash"
	stored := sampleStoredNarrative(datapackBuild, &dashboardNarrativePayload{
		Headline:         "AI headline",
		ExecutiveSummary: "AI executive",
	})
	resp.AIDiagnostics.AccountSummary.storedNarrative = stored
	applyUnifiedAccountHealth(mustRiskDate(t, "2026-05-21"), dto, &resp, nil, nil, nil, nil, &datapackBuild)
	if resp.AIDiagnostics.AccountSummary.NarrativeSource != "ai" {
		t.Fatalf("expected ai apply with datapack build, got %q", resp.AIDiagnostics.AccountSummary.NarrativeSource)
	}
	applyUnifiedAccountHealth(mustRiskDate(t, "2026-05-21"), dto, &resp, nil, nil, nil, nil, nil)
	if resp.AIDiagnostics.AccountSummary.NarrativeSource == "ai" {
		t.Fatal("legacy build without override must not accept datapack factual hash")
	}
}

func TestDashboardHomeConsistencyValidator(t *testing.T) {
	resp := Response{
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{
				storedNarrative: &storedDashboardNarrative{
					Status:        "completed",
					PromptVersion: DashboardNarrativePromptVersion,
				},
				Freshness:        "fallback",
				NarrativeSource:  "fallback",
				ValidationStatus: "fallback_stale",
			},
			AdEfficiency: AdEfficiencyWidget{
				Narrative: &AdEfficiencyNarrative{Status: "pending"},
			},
		},
		SellerPulse: SellerPulse{
			Narrative: &SellerPulseNarrative{Status: "stale"},
		},
	}
	issues := ValidateDashboardHomeResponseConsistency(&resp)
	if len(issues) < 3 {
		t.Fatalf("issues=%+v", issues)
	}
	if consistencyStatusFromIssues(issues) != "failed" {
		t.Fatalf("status=%s", consistencyStatusFromIssues(issues))
	}
}

func TestSurplusCountsOrLabelsConsistent(t *testing.T) {
	resp := Response{
		SKUFocus: SKUFocus{
			Surplus: []SKUFocusCard{{ProductName: "SKU-1"}, {ProductName: "SKU-2"}},
		},
		AIDiagnostics: AIDiagnostics{
			StockCoverage: StockCoverageWidget{
				Segments: []StockCoverageSegment{{Key: "surplus", SKUCount: 0}},
			},
			AccountSummary: AccountSummaryWidget{
				RiskDrivers: []RiskDriver{{
					ID: "driver-surplus",
					Evidence: map[string]any{
						"surplus_sku_count": 2,
					},
				}},
			},
		},
	}
	reconcileSurplusStockCoverageLabels(&resp)
	if !strings.Contains(resp.AIDiagnostics.StockCoverage.Segments[0].Label, "SURPLUS") {
		t.Fatalf("label=%q", resp.AIDiagnostics.StockCoverage.Segments[0].Label)
	}
	if resp.AIDiagnostics.StockCoverage.Message == "" {
		t.Fatal("expected slow-turnover note")
	}
}

func TestTopActionsRecommendedActionsConsistency(t *testing.T) {
	top := []TopActionItem{{
		Priority: "high",
		Title:    "Проверьте кампанию Alpha",
		Reason:   "Кампания Alpha тратит 12000 ₽ без заказов.",
	}}
	rec := recommendedActionsFromTopActions(top)
	if len(rec) != 1 || rec[0].Source != "dashboard_narrative" {
		t.Fatalf("rec=%+v", rec)
	}
}

func TestDashboardHomeResponseContractUnchanged(t *testing.T) {
	resp := Response{
		MetricDate: "2026-06-25",
		Status:     StatusReady,
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{WidgetBase: WidgetBase{Status: WidgetAvailable}},
			TopActions:     []TopActionItem{},
		},
		SellerPulse:        SellerPulse{KPIs: []KPICard{}},
		SKUFocus:           SKUFocus{},
		RecommendedActions: []ActionItem{},
	}
	raw, err := jsonMarshalDashboardHomeContract(resp)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		`"metric_date"`, `"status"`, `"ai_diagnostics"`, `"seller_pulse"`, `"sku_focus"`, `"recommended_actions"`,
	} {
		if !strings.Contains(raw, key) {
			t.Fatalf("missing contract key %s in %s", key, raw)
		}
	}
}

func jsonMarshalDashboardHomeContract(resp Response) (string, error) {
	type contractProbe struct {
		MetricDate         string        `json:"metric_date"`
		Status             GlobalStatus  `json:"status"`
		DataQualityNotes   []string      `json:"data_quality_notes"`
		AIDiagnostics      AIDiagnostics `json:"ai_diagnostics"`
		SellerPulse        SellerPulse   `json:"seller_pulse"`
		SKUFocus           SKUFocus      `json:"sku_focus"`
		RecommendedActions []ActionItem  `json:"recommended_actions"`
	}
	b, err := json.Marshal(contractProbe{
		MetricDate:         resp.MetricDate,
		Status:             resp.Status,
		DataQualityNotes:   resp.DataQualityNotes,
		AIDiagnostics:      resp.AIDiagnostics,
		SellerPulse:        resp.SellerPulse,
		SKUFocus:           resp.SKUFocus,
		RecommendedActions: resp.RecommendedActions,
	})
	return string(b), err
}
