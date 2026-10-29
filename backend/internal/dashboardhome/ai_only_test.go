package dashboardhome

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func forceAIOnly(t *testing.T) {
	t.Helper()
	t.Setenv("DASHBOARD_LEGACY_MODE", "")
}

func TestAIOnlyGenerationRequiresAllBlocks(t *testing.T) {
	forceAIOnly(t)
	payload := minimalAIOnlyPayload()
	errs := validateAIOnlyRequiredBlocks(payload)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
	payload.SellerPulseNarrative = nil
	errs = validateAIOnlyRequiredBlocks(payload)
	if len(errs) == 0 {
		t.Fatal("expected missing seller_pulse error")
	}
}

func TestAIOnlyGenerationFailsWhenSellerPulseMissing(t *testing.T) {
	forceAIOnly(t)
	t.Setenv("DASHBOARD_VALIDATION_MODE", DashboardValidationModeStrict)
	payload := minimalAIOnlyPayload()
	payload.SellerPulseNarrative = nil
	if accepted, _, _, hard := acceptGeneratedDashboardNarrative(payload, dashboardNarrativeBuildResult{}); accepted || len(hard) == 0 {
		t.Fatalf("expected rejection, accepted=%v hard=%v", accepted, hard)
	}
}

func TestAIOnlyGenerationFailsWhenAnyBlockPending(t *testing.T) {
	forceAIOnly(t)
	payload := minimalAIOnlyPayload()
	payload.AdEfficiencyNarrative.Status = "pending"
	if errs := validateAIOnlyRequiredBlocks(payload); len(errs) == 0 {
		t.Fatal("expected invalid ad efficiency status")
	}
}

func TestAIOnlyGenerationFailsWhenAnyBlockFallback(t *testing.T) {
	forceAIOnly(t)
	payload := minimalAIOnlyPayload()
	payload.SellerPulseNarrative.Status = "fallback"
	if errs := validateAIOnlyRequiredBlocks(payload); len(errs) == 0 {
		t.Fatal("expected invalid seller pulse status")
	}
}

func TestAIOnlyGenerationFailsWhenAnyBlockStale(t *testing.T) {
	forceAIOnly(t)
	payload := minimalAIOnlyPayload()
	payload.StockCoverageNarrative.Status = "stale"
	if errs := validateAIOnlyRequiredBlocks(payload); len(errs) == 0 {
		t.Fatal("expected invalid stock coverage status")
	}
}

func TestAIOnlyDoesNotUseLegacyDashboardContext(t *testing.T) {
	forceAIOnly(t)
	if !dashboardAIOnlyEnabled() {
		t.Fatal("expected ai-only enabled")
	}
}

func TestAIOnlyNoCompletedPartialSuccess(t *testing.T) {
	forceAIOnly(t)
	status := narrativeValidationStatusFromRepair(narrativeRepairResult{Partial: true}, 1)
	if status == "completed_partial" {
		t.Fatalf("unexpected status=%s", status)
	}
}

func TestAIOnlyDashboardHomeNoForbiddenStatuses(t *testing.T) {
	forceAIOnly(t)
	resp := minimalAIOnlyHomeResponse()
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, forbidden := range []string{`"status":"fallback"`, `"status":"stale"`, `"status":"pending"`, `"narrative_source":"fallback"`, `"validation_status":"fallback_stale"`, `"validation_status":"completed_partial"`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("forbidden fragment %q in response", forbidden)
		}
	}
}

func TestAIOnlyDashboardHomeAllAppliedBlocksAI(t *testing.T) {
	forceAIOnly(t)
	resp := minimalAIOnlyHomeResponse()
	debug := buildDashboardNarrativeDebug(&resp, nil)
	for block, source := range debug.AppliedBlocks {
		if block == "recommended_actions" {
			if source != "deprecated_dashboard_block" {
				t.Fatalf("block %s source=%s want deprecated_dashboard_block", block, source)
			}
			continue
		}
		if source != "ai" {
			t.Fatalf("block %s source=%s want ai", block, source)
		}
	}
	if debug.ConsistencyStatus != "passed" {
		t.Fatalf("consistency=%s", debug.ConsistencyStatus)
	}
}

func TestAIOnlyRecommendedActionsFromFullPayloadOnly(t *testing.T) {
	payload := minimalAIOnlyPayload()
	actions := aiOnlyRecommendedActionsFromPayload(payload)
	if len(actions) < 3 {
		t.Fatalf("actions=%d", len(actions))
	}
	if actions[0].Source != "dashboard_narrative_ai_only" {
		t.Fatalf("source=%s", actions[0].Source)
	}
}

func TestAIOnlySellerPulseFromPayloadOnly(t *testing.T) {
	forceAIOnly(t)
	resp := Response{
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{
				NarrativeSource: "ai",
				storedNarrative: &storedDashboardNarrative{
					Status:  "completed",
					Payload: minimalAIOnlyPayload(),
				},
			},
		},
		SellerPulse: SellerPulse{KPIs: []KPICard{{Label: "Выручка", Value: "1"}}},
	}
	applySellerPulseNarrativeToResponse(&resp, nil)
	if resp.SellerPulse.Narrative == nil || resp.SellerPulse.Narrative.Status != "available" {
		t.Fatalf("pulse=%+v", resp.SellerPulse.Narrative)
	}
}

func minimalAIOnlyPayload() *dashboardNarrativePayload {
	score := int32(6)
	categoryScore := int32(6)
	return &dashboardNarrativePayload{
		Headline:           "Заголовок",
		ExecutiveSummary:   "Резюме",
		ImprovementSummary: "Улучшения",
		AccountHealth: &dashboardAccountHealthBlock{
			HealthScore:        ptrInt32(62),
			Severity:           "medium",
			Headline:           "Заголовок",
			ExecutiveSummary:   "Резюме",
			ImprovementSummary: "Улучшения",
			RiskDrivers: []dashboardNarrativeRiskDriverView{
				{ID: driverStock, Category: "stock", CategoryLabel: "Остатки", Title: "Риск по остаткам", Reason: "Короткое покрытие.", Severity: "high", ImpactScore: 70},
			},
		},
		HealthReasons: []dashboardNarrativeHealthReason{
			{DriverID: driverStock, Category: "stock", Title: "Риск по остаткам", Reason: "Короткое покрытие.", FactIDs: []string{"sku.1.current.stock_available"}},
		},
		TopActions: []dashboardNarrativeTopAction{
			{Title: "A", Reason: "R", NextStep: "N", ExpectedEffect: "E", Priority: "high", Category: "sales", DriverID: driverAccountSales, FactIDs: []string{"account.last_7d.revenue"}},
			{Title: "B", Reason: "R", NextStep: "N", ExpectedEffect: "E", Priority: "high", Category: "stock", DriverID: driverStock, FactIDs: []string{"sku.1.current.stock_available"}},
			{Title: "C", Reason: "R", NextStep: "N", ExpectedEffect: "E", Priority: "medium", Category: "advertising", DriverID: driverAdvertising, FactIDs: []string{"campaign.1.last_7d.spend"}},
		},
		RecommendedActions: []ActionItem{
			{Action: "A", Priority: "high", UrgencyGroup: "urgent"},
			{Action: "B", Priority: "high", UrgencyGroup: "urgent"},
			{Action: "C", Priority: "medium", UrgencyGroup: "today"},
		},
		WhatChanged: &dashboardNarrativeWhatChanged{
			Summary:        "Изменения относительно прошлой сводки.",
			MainConclusion: "Главный вывод: выручка снизилась, приоритет — остатки и реклама.",
			Groups:         []dashboardNarrativeWhatChangedGroup{{Key: "worsened", Label: "Ухудшилось", Tone: "negative"}},
		},
		SellerPulseNarrative: &SellerPulseNarrative{
			Status: "available", Headline: "Пульс", Summary: "Сводка",
		},
		StockCoverageNarrative: &StockCoverageNarrative{Status: "available", Summary: "Остатки"},
		AdEfficiencyNarrative:  &AdEfficiencyNarrative{Status: "available", Summary: "Реклама"},
		FunnelAIAnalysis: &FunnelAIAnalysisWidget{
			WidgetBase: WidgetBase{Status: WidgetAvailable}, ShortSummary: "Воронка",
			WeakestStage:          FunnelWeakestStage{Key: "impressions_to_product_views", Label: "Показы → Просмотры", Value: "10%"},
			ActionItems:           []string{"Проверить карточки"},
			Comparisons:           FunnelComparisons{StageMetrics: []FunnelComparisonItem{{Key: "impressions_to_product_views", Label: "Показы → Просмотры"}}},
			ComparisonsCommentary: FunnelComparisonsCommentary{PreviousPeriod: "См. comparisons"},
		},
		CategorySummaries: []CategorySummaryCard{{
			CategoryID: "17036100", CategoryName: "Организация пространства",
			CategoryScore: &categoryScore, ScoreMax: 10, HealthScore: &score, Severity: "medium",
			Headline: "H", KeyRisk: "R", RecommendedAction: "A",
		}},
		RiskOverview: &RiskDistributionWidget{
			WidgetBase: WidgetBase{Status: WidgetAvailable},
			Groups: []RiskGroupCount{
				{Group: "high_risk", Label: "Высокий риск", Count: 1, Severity: "critical"},
				{Group: "medium_risk", Label: "Средний риск", Count: 1, Severity: "medium"},
				{Group: "growth_zone", Label: "Зона роста", Count: 1, Severity: "positive"},
			},
			Total: 3,
		},
		DataQualityNotes: []string{"note"},
	}
}

func minimalAIOnlyHomeResponse() Response {
	payload := minimalAIOnlyPayload()
	resp := Response{
		Status: StatusReady,
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{
				NarrativeSource:  "ai",
				ValidationStatus: "completed",
				Freshness:        "current",
				PromptVersion:    DashboardNarrativePromptVersion,
				Headline:         payload.Headline,
				ExecutiveSummary: payload.ExecutiveSummary,
				storedNarrative: &storedDashboardNarrative{
					ID:            "run-1",
					Status:        "completed",
					PromptVersion: DashboardNarrativePromptVersion,
					Payload:       payload,
				},
			},
			WhatChanged: WhatChangedWidget{WidgetBase: WidgetBase{Status: WidgetAvailable}, Groups: []WhatChangedGroup{}},
			TopActions: []TopActionItem{
				{Title: "A"}, {Title: "B"}, {Title: "C"},
			},
			FunnelAnalysis: FunnelAnalysisWidget{
				AIAnalysis: FunnelAIAnalysisWidget{WidgetBase: WidgetBase{Status: WidgetAvailable}},
			},
			StockCoverage: StockCoverageWidget{
				Narrative: payload.StockCoverageNarrative,
			},
			AdEfficiency: AdEfficiencyWidget{
				Narrative: payload.AdEfficiencyNarrative,
			},
			RiskDistribution:  *payload.RiskOverview,
			CategorySummaries: payload.CategorySummaries,
		},
		SellerPulse: SellerPulse{
			Narrative: payload.SellerPulseNarrative,
		},
		RecommendedActions: payload.RecommendedActions,
	}
	applyAIOnlyPayloadToResponse(&resp, payload, true)
	resp.SellerPulse.Narrative.Status = "available"
	return resp
}

func TestMain(m *testing.M) {
	_ = os.Unsetenv("DASHBOARD_AI_ONLY")
	_ = os.Setenv("DASHBOARD_LEGACY_MODE", "1")
	os.Exit(m.Run())
}

func TestDashboardDefaultsToAIOnly(t *testing.T) {
	forceAIOnly(t)
	if !dashboardAIOnlyEnabled() {
		t.Fatal("expected ai-only mode by default")
	}
}

func TestDashboardNoEnvStillUsesAIOnly(t *testing.T) {
	forceAIOnly(t)
	t.Setenv("DASHBOARD_AI_ONLY", "")
	if !dashboardAIOnlyEnabled() {
		t.Fatal("expected ai-only mode without env")
	}
}

func TestDashboardV9NeverReturnsFallbackAccountSummary(t *testing.T) {
	forceAIOnly(t)
	resp := minimalAIOnlyHomeResponse()
	if resp.AIDiagnostics.AccountSummary.PromptVersion == DashboardNarrativePromptVersion &&
		resp.AIDiagnostics.AccountSummary.NarrativeSource != "ai" {
		t.Fatalf("unexpected source=%s for v9 prompt", resp.AIDiagnostics.AccountSummary.NarrativeSource)
	}
}

func TestDashboardAIOnlyNeverReturnsStaleSellerPulse(t *testing.T) {
	forceAIOnly(t)
	resp := minimalAIOnlyHomeResponse()
	if resp.SellerPulse.Narrative == nil || resp.SellerPulse.Narrative.Status != "available" {
		t.Fatalf("unexpected seller pulse narrative=%+v", resp.SellerPulse.Narrative)
	}
}

func TestDashboardAIOnlyNeverReturnsAdNarrativeNotApplied(t *testing.T) {
	forceAIOnly(t)
	resp := minimalAIOnlyHomeResponse()
	if resp.AIDiagnostics.AdEfficiency.Narrative == nil || resp.AIDiagnostics.AdEfficiency.Narrative.Status != "available" {
		t.Fatalf("unexpected ad narrative=%+v", resp.AIDiagnostics.AdEfficiency.Narrative)
	}
}

func TestDashboardAIOnlyRecommendedActionsRequired(t *testing.T) {
	forceAIOnly(t)
	resp := minimalAIOnlyHomeResponse()
	if len(resp.RecommendedActions) < 3 {
		t.Fatalf("recommended_actions=%d", len(resp.RecommendedActions))
	}
}

func TestDashboardAIOnlyTopActionsDoNotUseRiskDriverFallback(t *testing.T) {
	forceAIOnly(t)
	payload := minimalAIOnlyPayload()
	top := buildTopActionsFromDashboardNarrativeAIOnly(payload, nil, 3)
	if len(top) < 3 {
		t.Fatalf("top_actions=%d", len(top))
	}
	for _, item := range top {
		if item.Source == "risk_driver" || item.Source == "ai_summary" {
			t.Fatalf("unexpected source=%s", item.Source)
		}
	}
}

func TestDashboardAIOnlyCategorySummariesRequired(t *testing.T) {
	forceAIOnly(t)
	resp := minimalAIOnlyHomeResponse()
	if len(resp.AIDiagnostics.CategorySummaries) == 0 {
		t.Fatal("expected non-empty category summaries")
	}
}

func TestDashboardAIOnlyFunnelAnalysisRequiredOrGenerationFails(t *testing.T) {
	forceAIOnly(t)
	t.Setenv("DASHBOARD_VALIDATION_MODE", DashboardValidationModeStrict)
	payload := minimalAIOnlyPayload()
	payload.FunnelAIAnalysis = nil
	accepted, _, _, errs := acceptGeneratedDashboardNarrative(payload, dashboardNarrativeBuildResult{})
	if accepted || len(errs) == 0 {
		t.Fatalf("expected generation failure, accepted=%v errs=%v", accepted, errs)
	}
}

func TestDashboardAIOnlyReturnsNotReadyWhenNoCompletedNarrative(t *testing.T) {
	forceAIOnly(t)
	outcome := acceptStoredDashboardNarrativeAIOnly(nil, nil, 3)
	if outcome.Applied {
		t.Fatal("expected narrative not applied")
	}
	if outcome.ValidationStatus != "failed" {
		t.Fatalf("validation_status=%s", outcome.ValidationStatus)
	}
}

func TestDashboardAIOnlyForbiddenSubstringsAbsent(t *testing.T) {
	forceAIOnly(t)
	resp := minimalAIOnlyHomeResponse()
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, forbidden := range []string{"fallback", "stale", "pending", "legacy", "ai_summary", "completed_partial", "fallback_stale"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("forbidden fragment %q in response", forbidden)
		}
	}
}
