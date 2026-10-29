package bundlevalidator

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardfacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/publication/adapter"
)

func healthScorePtr(v int32) *int32 { return &v }

func validBundleInput() Input {
	return Input{
		FactsPresent:    true,
		FactsIdentityOK: true,
		Facts: dashboardfacts.DashboardFactsV1{
			DataCoverage: dashboardfacts.DataCoverage{
				AccountMetricsDays: 7,
				FunnelAvailable:    true,
				AdMetricsDays:      7,
				StockSnapshotRows:  10,
			},
		},
		Projection: adapter.DashboardProjection{
			Frozen: dashboardhome.FrozenDashboardData{
				SellerPulse: dashboardhome.SellerPulse{
					KPIs:    []dashboardhome.KPICard{{Key: "revenue"}, {Key: "orders"}},
					Trend7D: []dashboardhome.TrendDayPoint{{Date: "2026-07-08"}},
				},
				SalesFunnel: dashboardhome.SalesFunnelWidget{
					Stages: []dashboardhome.FunnelStage{{Key: "impressions"}},
				},
				StockCoverage: dashboardhome.StockCoverageWidget{
					Segments: []dashboardhome.StockCoverageSegment{{Key: "deficit"}},
				},
				AdEfficiency: dashboardhome.AdEfficiencyWidget{
					Summary: &dashboardhome.AdEfficiencySummary{},
					Narrative: &dashboardhome.AdEfficiencyNarrative{
						Status:   "ok",
						Headline: "Реклама умеренно нагружает кабинет, но окупаемость кампаний низкая",
						Summary:  "За 2–8 июля на рекламу потрачено 163 800 ₽ — 16,8% общей выручки кабинета. Выручка, атрибутированная рекламным кампаниям, составила 178 500 ₽, поэтому доля расходов в рекламной выручке 91,8%. Проблемы локальны.",
						Checklist: []dashboardhome.AdEfficiencyCheckItem{
							{Title: "Кампания A с высоким ДРР", Text: "DRR кампании A 45% — расход не окупается атрибутированной выручкой"},
							{Title: "Кампания B без заказов", Text: "Кампания B без заказов за 3 дня при ненулевом расходе"},
						},
					},
				},
				FunnelAnalysis: dashboardhome.FunnelAnalysisWidget{
					AIAnalysis: dashboardhome.FunnelAIAnalysisWidget{ShortSummary: "funnel insight"},
				},
				RiskDistribution: dashboardhome.RiskDistributionWidget{
					Groups: []dashboardhome.RiskGroupCount{{Group: "stock_risk", Label: "Stock risk", Count: 1, Severity: "medium"}},
				},
			},
			Narrative: adapter.NarrativePayload{
				Headline:           "headline",
				ExecutiveSummary:   "summary",
				ImprovementSummary: "improvement plan",
				TopActions:         []adapter.NarrativeTopAction{{DriverID: "a1", Title: "Top action"}},
				AccountHealth: &adapter.NarrativeAccountHealth{
					HealthScore: healthScorePtr(70),
					Severity:    "medium",
					RiskDrivers: []adapter.NarrativeRiskDriverView{{
						ID: "stock", Category: "stock", CategoryLabel: "Остатки",
						Severity: "high", ImpactScore: 70, Title: "Stock risk", Reason: "Coverage low",
					}},
				},
				WhatChanged: &adapter.NarrativeWhatChanged{
					Summary: "what changed summary",
					Groups: []adapter.NarrativeWhatChangedGroup{
						{Key: "worsened", Summary: "DRR grew", Items: []adapter.NarrativeWhatChangedItem{{Label: "Реклама", Text: "DRR вырос"}}},
						{Key: "new_risks", Items: []adapter.NarrativeWhatChangedItem{}},
						{Key: "unchanged_risks", Items: []adapter.NarrativeWhatChangedItem{}},
						{Key: "improved", Summary: "Revenue grew", Items: []adapter.NarrativeWhatChangedItem{{Label: "Доход", Text: "Доход вырос"}}},
					},
				},
				SellerPulseNarrative: &dashboardhome.SellerPulseNarrative{
					Status:   "available",
					Headline: "pulse headline",
					Summary:  "pulse summary",
				},
			},
			HealthScore: healthScorePtr(70),
		},
		Actions:         materialActionsBundle(),
		AnalysisOutcome: contract.AnalysisOutcomeMaterialActionsFound,
	}
}

func materialActionsBundle() []adapter.ProjectedAction {
	mk := func(id, tier string) adapter.ProjectedAction {
		return adapter.ProjectedAction{
			AnalysisActionID:   id,
			ExecutionTier:      tier,
			StepCount:          1,
			CompletionCriteria: "done when x",
		}
	}
	return []adapter.ProjectedAction{
		mk("a1", contract.ExecutionTierRequired),
		mk("a2", contract.ExecutionTierRequired),
		mk("a3", contract.ExecutionTierRequired),
		mk("a4", contract.ExecutionTierRecommended),
		mk("a5", contract.ExecutionTierRecommended),
	}
}

func TestValidatePublicationBundle_ValidMaterialActions(t *testing.T) {
	report := ValidatePublicationBundle(validBundleInput())
	if !report.Valid {
		t.Fatalf("expected valid bundle, got errors: %+v", report.Errors)
	}
}

// TestValidatePublicationBundle_UnknownAIRiskDriverIDRejected verifies that
// an ai_risk_driver_unknown_id MismatchWarning (the AI invented a
// risk_drivers[].id outside the backend healthpolicy registry) fails
// publication, even though the projection itself is otherwise complete.
func TestValidatePublicationBundle_UnknownAIRiskDriverIDRejected(t *testing.T) {
	in := validBundleInput()
	in.MismatchWarnings = []adapter.MismatchWarning{{
		Field:   "account_summary.risk_drivers",
		Reason:  adapter.ReasonAIRiskDriverUnknownID,
		AIValue: "stock_deficit",
	}}
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when AI risk_drivers references an unknown driver_id")
	}
	if report.FirstErrorCode() != CodeDashboardRiskDriverUnknownID {
		t.Fatalf("expected code %s, got %s", CodeDashboardRiskDriverUnknownID, report.FirstErrorCode())
	}
}

// TestValidatePublicationBundle_OtherMismatchWarningsDoNotBlock verifies
// that informational MismatchWarning reasons (e.g. a backend-wins KPI
// mismatch) never fail publication — only
// adapter.ReasonAIRiskDriverUnknownID does.
func TestValidatePublicationBundle_OtherMismatchWarningsDoNotBlock(t *testing.T) {
	in := validBundleInput()
	in.MismatchWarnings = []adapter.MismatchWarning{{
		Field:     "account_summary.health_score",
		Reason:    "backend_score_overrides_ai",
		AIValue:   "62",
		FactValue: "70",
	}}
	report := ValidatePublicationBundle(in)
	if !report.Valid {
		t.Fatalf("expected valid bundle for a non-blocking mismatch warning, got errors: %+v", report.Errors)
	}
}

func TestValidatePublicationBundle_FactsMissing(t *testing.T) {
	in := validBundleInput()
	in.FactsPresent = false
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when facts are missing")
	}
	if report.FirstErrorCode() != CodeDashboardFactsMissing {
		t.Fatalf("expected code %s, got %s", CodeDashboardFactsMissing, report.FirstErrorCode())
	}
}

func TestValidatePublicationBundle_FactsIdentityMismatch(t *testing.T) {
	in := validBundleInput()
	in.FactsIdentityOK = false
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when facts identity does not match")
	}
	if report.FirstErrorCode() != CodeDashboardFactsHashMismatch {
		t.Fatalf("expected code %s, got %s", CodeDashboardFactsHashMismatch, report.FirstErrorCode())
	}
}

// TestValidatePublicationBundle_MissingNarrativeOnMaterial verifies that a
// material_actions_found bundle with an empty headline/executive_summary is
// rejected with unified_dashboard_narrative_missing.
func TestValidatePublicationBundle_MissingNarrativeOnMaterial(t *testing.T) {
	in := validBundleInput()
	in.Projection.Narrative.Headline = ""
	in.Projection.Narrative.ExecutiveSummary = ""

	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when narrative headline/executive_summary are missing")
	}
	found := false
	for _, issue := range report.Errors {
		if issue.Code == CodeDashboardNarrativeMissing {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected %s among errors, got %+v", CodeDashboardNarrativeMissing, report.Errors)
	}
}

func TestValidatePublicationBundle_MaterialActionsRequiresAtLeastOneAction(t *testing.T) {
	in := validBundleInput()
	in.Actions = nil
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when material_actions_found has 0 actions")
	}
	found := false
	for _, issue := range report.Errors {
		if issue.Code == CodeActionsMissing {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected %s among errors, got %+v", CodeActionsMissing, report.Errors)
	}
}

func TestValidatePublicationBundle_NoMaterialActionsMustHaveZeroActions(t *testing.T) {
	in := validBundleInput()
	in.AnalysisOutcome = contract.AnalysisOutcomeNoMaterialActions
	// Actions is still populated from validBundleInput(); no_material_actions requires 0.
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when no_material_actions has non-zero actions")
	}
	found := false
	for _, issue := range report.Errors {
		if issue.Code == CodeActionsBundleInconsistent {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected %s among errors, got %+v", CodeActionsBundleInconsistent, report.Errors)
	}
}

func TestValidatePublicationBundle_KPIsMissingWhenAccountCoveragePresent(t *testing.T) {
	in := validBundleInput()
	in.Projection.Frozen.SellerPulse.KPIs = nil
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when account metrics coverage present but KPIs missing")
	}
	found := false
	for _, issue := range report.Errors {
		if issue.Code == CodeDashboardKPIsMissing {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected %s among errors, got %+v", CodeDashboardKPIsMissing, report.Errors)
	}
}

func TestValidatePublicationBundle_TrendMissingWhenAccountCoveragePresent(t *testing.T) {
	in := validBundleInput()
	in.Projection.Frozen.SellerPulse.Trend7D = nil
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when account metrics coverage present but trend_7d missing")
	}
	found := false
	for _, issue := range report.Errors {
		if issue.Code == CodeDashboardTrendMissing {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected %s among errors, got %+v", CodeDashboardTrendMissing, report.Errors)
	}
}

func TestValidatePublicationBundle_FunnelMissingWhenAvailable(t *testing.T) {
	in := validBundleInput()
	in.Projection.Frozen.SalesFunnel.Stages = nil
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when funnel is available but stages are missing")
	}
	if report.FirstErrorCode() != CodeDashboardFunnelMissing {
		t.Fatalf("expected code %s, got %s", CodeDashboardFunnelMissing, report.FirstErrorCode())
	}
}

func TestValidatePublicationBundle_StockMissingWithoutExplicitStatus(t *testing.T) {
	in := validBundleInput()
	in.Projection.Frozen.StockCoverage.Segments = nil
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when stock snapshot present but no segments/explicit status")
	}
	if report.FirstErrorCode() != CodeDashboardStockMissing {
		t.Fatalf("expected code %s, got %s", CodeDashboardStockMissing, report.FirstErrorCode())
	}
}

// TestValidatePublicationBundle_StockNotEnoughDataIsValid verifies the
// insufficient_data escape hatch: an explicit not_enough_data status on the
// stock block is accepted even though there are no segments.
func TestValidatePublicationBundle_StockNotEnoughDataIsValid(t *testing.T) {
	in := validBundleInput()
	in.Projection.Frozen.StockCoverage.Segments = nil
	in.Projection.Frozen.StockCoverage.Status = dashboardhome.WidgetNotEnoughData
	report := ValidatePublicationBundle(in)
	if !report.Valid {
		t.Fatalf("expected valid bundle with explicit not_enough_data status, got errors: %+v", report.Errors)
	}
}

func TestValidatePublicationBundle_AdsMissingWhenAvailable(t *testing.T) {
	in := validBundleInput()
	in.Projection.Frozen.AdEfficiency.Summary = nil
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when ad metrics available but summary missing")
	}
	if report.FirstErrorCode() != CodeDashboardAdsMissing {
		t.Fatalf("expected code %s, got %s", CodeDashboardAdsMissing, report.FirstErrorCode())
	}
}

// TestValidatePublicationBundle_InsufficientDataAllowsFewerBlocks verifies
// that insufficient_data outcome tolerates empty dashboard blocks as long as
// coverage itself is absent (no invented data required).
func TestValidatePublicationBundle_InsufficientDataAllowsFewerBlocks(t *testing.T) {
	in := Input{
		FactsPresent:    true,
		FactsIdentityOK: true,
		Facts:           dashboardfacts.DashboardFactsV1{},
		Projection: adapter.DashboardProjection{
			Frozen: dashboardhome.FrozenDashboardData{},
			Narrative: adapter.NarrativePayload{
				Headline:         "Анализ ограничен",
				ExecutiveSummary: "Часть источников недоступна",
			},
		},
		Actions:         nil,
		AnalysisOutcome: contract.AnalysisOutcomeInsufficientData,
	}
	report := ValidatePublicationBundle(in)
	if !report.Valid {
		t.Fatalf("expected valid bundle for insufficient_data with no coverage, got errors: %+v", report.Errors)
	}
}

func TestValidatePublicationBundle_HealthScoreMissing(t *testing.T) {
	in := validBundleInput()
	in.Projection.HealthScore = nil
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when material outcome has a nil health_score")
	}
	if !containsCode(report.Errors, CodeDashboardHealthScoreMissing) {
		t.Fatalf("expected %s among errors, got %+v", CodeDashboardHealthScoreMissing, report.Errors)
	}
}

func TestValidatePublicationBundle_HealthScoreOutOfRange(t *testing.T) {
	in := validBundleInput()
	in.Projection.HealthScore = healthScorePtr(150)
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when health_score is out of range")
	}
	if !containsCode(report.Errors, CodeDashboardHealthScoreOutOfRange) {
		t.Fatalf("expected %s among errors, got %+v", CodeDashboardHealthScoreOutOfRange, report.Errors)
	}
}

func TestValidatePublicationBundle_WhatChangedMissing(t *testing.T) {
	in := validBundleInput()
	in.Projection.Narrative.WhatChanged = nil
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when material outcome has no what_changed narrative")
	}
	if !containsCode(report.Errors, CodeDashboardWhatChangedMissing) {
		t.Fatalf("expected %s among errors, got %+v", CodeDashboardWhatChangedMissing, report.Errors)
	}
}

func TestValidatePublicationBundle_WhatChangedThematicKeysRejected(t *testing.T) {
	in := validBundleInput()
	in.Projection.Narrative.WhatChanged = &adapter.NarrativeWhatChanged{
		Summary: "summary present",
		Groups: []adapter.NarrativeWhatChangedGroup{
			{Key: "revenue", Items: []adapter.NarrativeWhatChangedItem{{Label: "Revenue", Text: "up"}}},
			{Key: "ad_efficiency", Items: []adapter.NarrativeWhatChangedItem{{Label: "Ads", Text: "down"}}},
			{Key: "sku_orders_drop", Items: []adapter.NarrativeWhatChangedItem{{Label: "SKU", Text: "drop"}}},
			{Key: "slow_turnover", Items: []adapter.NarrativeWhatChangedItem{{Label: "Stock", Text: "slow"}}},
		},
	}
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when what_changed uses thematic keys")
	}
	if !containsCode(report.Errors, CodeDashboardWhatChangedMissing) {
		t.Fatalf("expected %s among errors, got %+v", CodeDashboardWhatChangedMissing, report.Errors)
	}
}

func TestValidatePublicationBundle_WhatChangedRequiresSummaryWhenItems(t *testing.T) {
	in := validBundleInput()
	in.Projection.Narrative.WhatChanged = &adapter.NarrativeWhatChanged{
		Summary: "summary present",
		Groups: []adapter.NarrativeWhatChangedGroup{
			{Key: "worsened", Items: []adapter.NarrativeWhatChangedItem{{Label: "Ads", Text: "DRR up"}}},
			{Key: "new_risks", Items: []adapter.NarrativeWhatChangedItem{}},
			{Key: "unchanged_risks", Items: []adapter.NarrativeWhatChangedItem{}},
			{Key: "improved", Items: []adapter.NarrativeWhatChangedItem{}},
		},
	}
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when what_changed group has items without summary")
	}
	if !containsCode(report.Errors, CodeDashboardWhatChangedMissing) {
		t.Fatalf("expected %s among errors, got %+v", CodeDashboardWhatChangedMissing, report.Errors)
	}
}

func TestValidatePublicationBundle_WhatChangedRequiresGroupWithItem(t *testing.T) {
	in := validBundleInput()
	in.Projection.Narrative.WhatChanged = &adapter.NarrativeWhatChanged{
		Summary: "summary present",
		Groups:  []adapter.NarrativeWhatChangedGroup{{Key: "revenue"}},
	}
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when what_changed groups have no items")
	}
	if !containsCode(report.Errors, CodeDashboardWhatChangedMissing) {
		t.Fatalf("expected %s among errors, got %+v", CodeDashboardWhatChangedMissing, report.Errors)
	}
}

func TestValidatePublicationBundle_SellerPulseNarrativeMissing(t *testing.T) {
	in := validBundleInput()
	in.Projection.Narrative.SellerPulseNarrative = nil
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when material outcome has no seller_pulse narrative")
	}
	if !containsCode(report.Errors, CodeDashboardSellerPulseMissing) {
		t.Fatalf("expected %s among errors, got %+v", CodeDashboardSellerPulseMissing, report.Errors)
	}
}

func TestValidatePublicationBundle_ImprovementSummaryMissing(t *testing.T) {
	in := validBundleInput()
	in.Projection.Narrative.ImprovementSummary = ""
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when material outcome has no improvement_summary")
	}
	if !containsCode(report.Errors, CodeDashboardAccountSummaryMissing) {
		t.Fatalf("expected %s among errors, got %+v", CodeDashboardAccountSummaryMissing, report.Errors)
	}
}

func TestValidatePublicationBundle_RiskDistributionMissingForMaterialActionsFound(t *testing.T) {
	in := validBundleInput()
	in.Projection.Frozen.RiskDistribution.Groups = nil
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when material_actions_found has empty risk_distribution.groups")
	}
	if !containsCode(report.Errors, CodeDashboardRiskDistributionMissing) {
		t.Fatalf("expected %s among errors, got %+v", CodeDashboardRiskDistributionMissing, report.Errors)
	}
}

// TestValidatePublicationBundle_RiskDistributionAllowedEmptyForNoMaterialActions
// verifies a healthy no_material_actions account can legitimately have zero
// risk groups.
func TestValidatePublicationBundle_RiskDistributionAllowedEmptyForNoMaterialActions(t *testing.T) {
	in := validBundleInput()
	in.AnalysisOutcome = contract.AnalysisOutcomeNoMaterialActions
	in.Actions = nil
	in.Projection.Frozen.RiskDistribution.Groups = nil
	report := ValidatePublicationBundle(in)
	if !report.Valid {
		t.Fatalf("expected valid bundle for no_material_actions with empty risk_distribution, got errors: %+v", report.Errors)
	}
}

func TestValidatePublicationBundle_TopActionsMissingForMaterialActionsFound(t *testing.T) {
	in := validBundleInput()
	in.Projection.Narrative.TopActions = nil
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when material_actions_found has no top actions")
	}
	if !containsCode(report.Errors, CodeDashboardTopActionsMissing) {
		t.Fatalf("expected %s among errors, got %+v", CodeDashboardTopActionsMissing, report.Errors)
	}
}

func TestValidatePublicationBundle_TopActionsTooMany(t *testing.T) {
	in := validBundleInput()
	in.Projection.Narrative.TopActions = []adapter.NarrativeTopAction{
		{DriverID: "a1", Title: "One"},
		{DriverID: "a2", Title: "Two"},
		{DriverID: "a3", Title: "Three"},
		{DriverID: "a4", Title: "Four"},
	}
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid when more than 3 top actions")
	}
	if !containsCode(report.Errors, CodeDashboardTopActionsTooMany) {
		t.Fatalf("expected %s, got %+v", CodeDashboardTopActionsTooMany, report.Errors)
	}
}

func TestValidatePublicationBundle_TopActionUnresolved(t *testing.T) {
	in := validBundleInput()
	in.Projection.Narrative.TopActions = []adapter.NarrativeTopAction{
		{DriverID: "missing-action", Title: "Ghost"},
	}
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid when top action id does not resolve")
	}
	if !containsCode(report.Errors, CodeDashboardTopActionUnresolved) {
		t.Fatalf("expected %s, got %+v", CodeDashboardTopActionUnresolved, report.Errors)
	}
}

func TestValidatePublicationBundle_TopActionsNoArtificialFill(t *testing.T) {
	in := validBundleInput()
	in.Actions = []adapter.ProjectedAction{
		{AnalysisActionID: "a1", ExecutionTier: contract.ExecutionTierRequired, StepCount: 1, CompletionCriteria: "done"},
	}
	in.Projection.Narrative.TopActions = []adapter.NarrativeTopAction{
		{DriverID: "a1", Title: "One"},
		{DriverID: "missing-2", Title: "Padded"},
		{DriverID: "missing-3", Title: "Padded 2"},
	}
	issues := checkTopActions(in)
	if !containsCode(issues, CodeDashboardTopActionArtificialFill) {
		t.Fatalf("expected %s, got %+v", CodeDashboardTopActionArtificialFill, issues)
	}
	if !containsCode(issues, CodeDashboardTopActionUnresolved) {
		t.Fatalf("expected unresolved padded ids, got %+v", issues)
	}
}

func TestValidatePublicationBundle_ActionDetailIncomplete(t *testing.T) {
	in := validBundleInput()
	in.Actions = []adapter.ProjectedAction{{AnalysisActionID: "a1", StepCount: 0, CompletionCriteria: ""}}
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when an action has no steps/completion_criteria")
	}
	if !containsCode(report.Errors, CodeActionDetailIncomplete) {
		t.Fatalf("expected %s among errors, got %+v", CodeActionDetailIncomplete, report.Errors)
	}
}

func TestValidatePublicationBundle_PartialWidgetNotPublishable(t *testing.T) {
	in := validBundleInput()
	in.Projection.Frozen.StockCoverage.Status = dashboardhome.WidgetPartial
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when a widget status is partial on a material outcome")
	}
	if !containsCode(report.Errors, CodeBundlePartialNotPublishable) {
		t.Fatalf("expected %s among errors, got %+v", CodeBundlePartialNotPublishable, report.Errors)
	}
}

func TestValidatePublicationBundle_ContainsPlaceholderText(t *testing.T) {
	in := validBundleInput()
	in.Projection.Narrative.ExecutiveSummary = "TODO: write real summary"
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when executive_summary contains a placeholder marker")
	}
	if !containsCode(report.Errors, CodeBundleContainsPlaceholder) {
		t.Fatalf("expected %s among errors, got %+v", CodeBundleContainsPlaceholder, report.Errors)
	}
}

func TestValidatePublicationBundle_SKUFocusMissingWhenFactsIndexNonEmpty(t *testing.T) {
	in := validBundleInput()
	in.Facts.SKUFactsIndex = dashboardfacts.SKUFactsIndex{
		ByOzonProductID: map[string]dashboardfacts.SKUFactRecord{
			"1": {OzonProductID: 1},
		},
	}
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when sku facts index is non-empty but sku_focus is entirely empty")
	}
	if !containsCode(report.Errors, CodeDashboardSKUFocusMissing) {
		t.Fatalf("expected %s among errors, got %+v", CodeDashboardSKUFocusMissing, report.Errors)
	}
}

func TestValidatePublicationBundle_SKUFocusOKWhenAnyListNonEmpty(t *testing.T) {
	in := validBundleInput()
	in.Facts.SKUFactsIndex = dashboardfacts.SKUFactsIndex{
		ByOzonProductID: map[string]dashboardfacts.SKUFactRecord{
			"1": {OzonProductID: 1},
		},
	}
	in.Projection.Frozen.SKUFocus.OrderDrop = []dashboardhome.SKUFocusCard{{OzonProductID: 1, ProductName: "A"}}
	report := ValidatePublicationBundle(in)
	if !report.Valid {
		t.Fatalf("expected valid bundle when sku_focus has at least one entry, got errors: %+v", report.Errors)
	}
}

func TestValidatePublicationBundle_FunnelAnalysisMissingWhenStagesPresent(t *testing.T) {
	in := validBundleInput()
	in.Projection.Frozen.FunnelAnalysis.AIAnalysis.ShortSummary = ""
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when funnel stages are present but funnel_analysis AI text is empty")
	}
	if !containsCode(report.Errors, CodeDashboardFunnelAnalysisMissing) {
		t.Fatalf("expected %s among errors, got %+v", CodeDashboardFunnelAnalysisMissing, report.Errors)
	}
}

func TestValidatePublicationBundle_AdvertisingChecklistMissingWhenAdsAvailable(t *testing.T) {
	in := validBundleInput()
	in.Projection.Frozen.AdEfficiency.Narrative.Checklist = nil
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when ad metrics available but checklist is empty")
	}
	if !containsCode(report.Errors, CodeDashboardAdvertisingChecklistMissing) {
		t.Fatalf("expected %s among errors, got %+v", CodeDashboardAdvertisingChecklistMissing, report.Errors)
	}
}

func TestValidatePublicationBundle_AdvertisingChecklistTooShort(t *testing.T) {
	in := validBundleInput()
	in.Projection.Frozen.AdEfficiency.Narrative.Checklist = []dashboardhome.AdEfficiencyCheckItem{
		{Title: "Only one item", Text: "not enough"},
	}
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when checklist has fewer than 2 items")
	}
	if !containsCode(report.Errors, CodeDashboardAdvertisingChecklistMissing) {
		t.Fatalf("expected %s among errors, got %+v", CodeDashboardAdvertisingChecklistMissing, report.Errors)
	}
}

func TestValidatePublicationBundle_CategoriesMissingWhenCatalogHasCategories(t *testing.T) {
	in := validBundleInput()
	in.Facts.DataCoverage.CategoryCount = 3
	in.Facts.Categories = nil
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when catalog has categories but categories facts are empty")
	}
	if !containsCode(report.Errors, CodeDashboardCategoriesMissing) {
		t.Fatalf("expected %s among errors, got %+v", CodeDashboardCategoriesMissing, report.Errors)
	}
}

func TestValidatePublicationBundle_CategoryAnalysisMissingWhenCategoryFactsPresent(t *testing.T) {
	in := validBundleInput()
	in.Facts.Categories = []dashboardfacts.CategoryFacts{{CategoryID: 1, CategoryName: "Обувь"}}
	in.Projection.Narrative.CategorySummaries = nil
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when category facts are present but category_summaries is empty")
	}
	if !containsCode(report.Errors, CodeDashboardCategoryAnalysisMissing) {
		t.Fatalf("expected %s among errors, got %+v", CodeDashboardCategoryAnalysisMissing, report.Errors)
	}
}

func TestValidatePublicationBundle_CategoryAnalysisIncompleteEntry(t *testing.T) {
	in := validBundleInput()
	in.Facts.Categories = []dashboardfacts.CategoryFacts{{CategoryID: 1, CategoryName: "Обувь"}}
	in.Projection.Narrative.CategorySummaries = []dashboardhome.CategorySummaryCard{
		{CategoryID: "1", CategoryName: "Обувь"}, // missing CategoryScore/Headline
	}
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when a category_summaries entry is missing category_score/headline")
	}
	if !containsCode(report.Errors, CodeDashboardCategoryAnalysisMissing) {
		t.Fatalf("expected %s among errors, got %+v", CodeDashboardCategoryAnalysisMissing, report.Errors)
	}
}

func TestValidatePublicationBundle_CategoryAnalysisOKWhenComplete(t *testing.T) {
	in := validBundleInput()
	in.Facts.Categories = []dashboardfacts.CategoryFacts{{CategoryID: 1, CategoryName: "Обувь"}}
	score := int32(6)
	in.Projection.Narrative.CategorySummaries = []dashboardhome.CategorySummaryCard{
		{CategoryID: "1", CategoryName: "Обувь", CategoryScore: &score, Headline: "Обувь стабильна", KeyRisk: "Риск затоваривания"},
	}
	report := ValidatePublicationBundle(in)
	if !report.Valid {
		t.Fatalf("expected valid bundle when category_summaries is complete, got errors: %+v", report.Errors)
	}
}

func TestValidatePublicationBundle_SKUTrendMissingWhenSparklineAbsent(t *testing.T) {
	in := validBundleInput()
	in.Facts.DataCoverage.SKUMetricsDays = 3
	in.Projection.Frozen.SKUFocus.OrderDrop = []dashboardhome.SKUFocusCard{
		{OzonProductID: 1, ProductName: "SKU 1"},
	}
	report := ValidatePublicationBundle(in)
	if report.Valid {
		t.Fatal("expected invalid bundle when sku_focus card lacks trend_status and multi-day sparkline")
	}
	if !containsCode(report.Errors, CodeDashboardSKUTrendMissing) {
		t.Fatalf("expected %s among errors, got %+v", CodeDashboardSKUTrendMissing, report.Errors)
	}
}

func TestValidatePublicationBundle_SKUTrendSparseAllowedWithStatus(t *testing.T) {
	in := validBundleInput()
	in.Facts.DataCoverage.SKUMetricsDays = 3
	in.Projection.Frozen.SKUFocus.OrderDrop = []dashboardhome.SKUFocusCard{
		{
			OzonProductID: 1,
			ProductName:   "SKU 1",
			Problem:       "Падение заказов",
			TrendStatus:   dashboardhome.SKUFocusTrendNotEnoughData,
			TrendMessage:  "Недостаточно точек для тренда",
			Sparkline7D: []dashboardhome.SparklinePoint{
				{Date: "2026-07-07", HasData: true},
			},
		},
	}
	report := ValidatePublicationBundle(in)
	if !report.Valid {
		t.Fatalf("sparse trend with explicit status must be valid, got %+v", report.Errors)
	}
}

func TestValidatePublicationBundle_SKUTrendOKWithObservedSparkline(t *testing.T) {
	in := validBundleInput()
	in.Facts.DataCoverage.SKUMetricsDays = 3
	in.Projection.Frozen.SKUFocus.OrderDrop = []dashboardhome.SKUFocusCard{
		{
			OzonProductID: 1,
			ProductName:   "SKU 1",
			TrendStatus:   dashboardhome.SKUFocusTrendAvailable,
			Sparkline7D: []dashboardhome.SparklinePoint{
				{Date: "2026-07-06", HasData: true},
				{Date: "2026-07-07", HasData: true},
			},
		},
	}
	report := ValidatePublicationBundle(in)
	if !report.Valid {
		t.Fatalf("expected valid bundle when sku_focus card has a multi-day sparkline, got errors: %+v", report.Errors)
	}
}

func containsCode(issues []Issue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
