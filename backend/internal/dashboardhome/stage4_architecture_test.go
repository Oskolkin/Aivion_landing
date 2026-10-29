package dashboardhome

import (
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func TestAIOnlyDoesNotBuildRiskDrivers(t *testing.T) {
	forceAIOnlyArchitecture(t)
	metricDate := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
	dto := analytics.DashboardMetricsDTO{
		Account: analytics.DashboardAccountMetrics{RevenueToday: 1000, OrdersToday: 10},
	}
	resp := Response{
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{
				storedNarrative: &storedDashboardNarrative{
					Status:        "completed",
					PromptVersion: DashboardNarrativePromptVersion,
					Payload:       minimalAIOnlyPayload(),
				},
			},
			FunnelAnalysis: FunnelAnalysisWidget{AIAnalysis: FunnelAIAnalysisWidget{WidgetBase: WidgetBase{Status: WidgetAvailable}}},
		},
	}
	applyUnifiedAccountHealthAIOnly(metricDate, dto, &resp, nil, nil, nil, nil, nil)
	if len(resp.AIDiagnostics.AccountSummary.RiskDrivers) == 0 {
		t.Fatal("expected AI risk drivers")
	}
	if resp.AIDiagnostics.AccountSummary.HealthScore == nil {
		t.Fatal("expected AI health score")
	}
	if resp.AIDiagnostics.AccountSummary.HealthScoreBreakdown == nil {
		t.Fatal("expected health score breakdown from published account health / drivers")
	}
}

func TestAIOnlyWhatChangedDoesNotUseClassifiedFacts(t *testing.T) {
	forceAIOnlyArchitecture(t)
	wc := &dashboardNarrativeWhatChanged{
		Summary: "Изменения",
		Groups: []dashboardNarrativeWhatChangedGroup{
			{Key: whatChangedKeyWorsened, Label: "Ухудшилось", Tone: "negative", Summary: "Выручка снизилась", Items: []dashboardNarrativeWhatChangedItem{{Label: "Выручка", Text: "−5%"}}},
		},
	}
	bundle := whatChangedValidationBundle{
		Context: dashboardNarrativeWhatChangedContext{
			PeriodComparison: dashboardNarrativePeriodComparison{
				Facts: []periodComparisonFact{{ID: "account.revenue", Label: "Выручка", CurrentValue: "1000", PreviousValue: "1100"}},
			},
			WorsenedFacts: []whatChangedFact{{ID: "backend-only", Label: "Не должно попасть"}},
		},
	}
	repaired := repairWhatChangedAIOnly(wc, bundle)
	if repaired == nil || len(repaired.Groups) == 0 {
		t.Fatal("expected repaired what_changed")
	}
	for _, item := range repaired.Groups[0].Items {
		if item.Text == "Не должно попасть" {
			t.Fatal("backend classified facts must not enrich AI-only what_changed groups")
		}
	}
}

func TestAIOnlyCategoryScoreRequiredFromAI(t *testing.T) {
	forceAIOnlyArchitecture(t)
	payload := minimalAIOnlyPayload()
	payload.CategorySummaries[0].CategoryScore = nil
	if errs := validateAIOnlyCategoryScoresRequired(payload); len(errs) == 0 {
		t.Fatal("expected missing category score error")
	}
	score := int32(7)
	payload.CategorySummaries[0].CategoryScore = &score
	cards := normalizeCategorySummaryCardsAIOnly(payload.CategorySummaries)
	if len(cards) != 1 || cards[0].CategoryScore == nil {
		t.Fatal("expected AI category score preserved")
	}
	payload.CategorySummaries[0].CategoryScore = nil
	if len(normalizeCategorySummaryCardsAIOnly(payload.CategorySummaries)) != 0 {
		t.Fatal("cards without AI score must be dropped")
	}
}

func TestAIOnlyAccountHealthRequired(t *testing.T) {
	forceAIOnlyArchitecture(t)
	payload := minimalAIOnlyPayload()
	payload.AccountHealth = nil
	if errs := validateAIOnlyAccountHealth(payload); len(errs) == 0 {
		t.Fatal("expected missing account_health error")
	}
}

func TestStage4PipelineValidatesRequiredBlocks(t *testing.T) {
	forceAIOnlyArchitecture(t)
	payload := minimalAIOnlyPayload()
	entries := validateStage4Blocks(payload)
	if !allStage4BlocksValid(entries) {
		t.Fatalf("expected valid blocks, got %+v", entries)
	}
	payload.AccountHealth = nil
	entries = validateStage4Blocks(payload)
	if allStage4BlocksValid(entries) {
		t.Fatal("expected account_health validation failure")
	}
}

func TestBuildHomeAppliesStoredNarrativeWithoutRecommendedActions(t *testing.T) {
	forceAIOnlyArchitecture(t)
	payload := minimalAIOnlyPayload()
	resp := Response{
		Status: StatusReady,
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{
				storedNarrative: &storedDashboardNarrative{
					Status:        "completed",
					PromptVersion: DashboardNarrativePromptVersion,
					Payload:       payload,
				},
			},
			FunnelAnalysis: FunnelAnalysisWidget{AIAnalysis: FunnelAIAnalysisWidget{WidgetBase: WidgetBase{Status: WidgetAvailable}}},
		},
	}
	metricDate := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
	dto := analytics.DashboardMetricsDTO{Account: analytics.DashboardAccountMetrics{RevenueToday: 1}}
	applyUnifiedAccountHealthAIOnly(metricDate, dto, &resp, []dbgen.Recommendation{}, []dbgen.Alert{}, nil, nil, nil)
	if resp.AIDiagnostics.AccountSummary.NarrativeSource != "ai" {
		t.Fatalf("narrative source=%q", resp.AIDiagnostics.AccountSummary.NarrativeSource)
	}
	if len(resp.RecommendedActions) != 0 {
		t.Fatalf("recommended_actions must stay empty, got %d", len(resp.RecommendedActions))
	}
}

func forceAIOnlyArchitecture(t *testing.T) {
	t.Helper()
	t.Setenv("DASHBOARD_LEGACY_MODE", "")
	if !dashboardAIOnlyEnabled() {
		t.Fatal("expected ai-only mode")
	}
}
