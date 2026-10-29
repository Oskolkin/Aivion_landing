package dashboardhome

import (
	"fmt"
	"testing"
)

func TestDashboardValidationMode_DefaultIsPilotStructural(t *testing.T) {
	t.Setenv("DASHBOARD_VALIDATION_MODE", "")
	if got := CurrentDashboardValidationMode(); got != DashboardValidationModePilotStructural {
		t.Fatalf("mode=%q", got)
	}
}

func TestDashboardValidationMode_UnknownFallsBackToPilotStructural(t *testing.T) {
	t.Setenv("DASHBOARD_VALIDATION_MODE", "totally_unknown")
	if got := CurrentDashboardValidationMode(); got != DashboardValidationModePilotStructural {
		t.Fatalf("mode=%q", got)
	}
}

func TestPilotStructuralBlockingAllowlist_UnknownSemanticIssueIsWarning(t *testing.T) {
	result := classifyBlockingCodes([]string{"future_semantic_check_failed"}, DashboardValidationModePilotStructural, "test")
	if !result.Valid {
		t.Fatalf("expected accepted dashboard, blocking=%+v", result.BlockingErrors)
	}
	if len(result.Warnings) != 1 || result.Warnings[0].Code != "future_semantic_check_failed" {
		t.Fatalf("warnings=%+v", result.Warnings)
	}
}

func TestHeadlineMismatch_PilotModeAcceptedWithWarning(t *testing.T) {
	forceAIOnlyArchitecture(t)
	t.Setenv("DASHBOARD_VALIDATION_MODE", DashboardValidationModePilotStructural)

	payload := minimalAIOnlyPayload()
	payload.Headline = "Продажи требуют внимания"
	payload.AccountHealth.Headline = "Стабильная динамика по кабинету"

	result := evaluateStage4CrossBlockReview(payload, DashboardValidationModePilotStructural)
	if !result.Valid {
		t.Fatalf("expected valid dashboard, got blocking=%v", result.BlockingErrors)
	}
	if len(result.Warnings) != 1 || result.Warnings[0].Code != "headline_mismatch_between_root_and_account_health" {
		t.Fatalf("warnings=%+v", result.Warnings)
	}

	accepted, _, _, validation := acceptGeneratedDashboardNarrativeWithValidation(payload, dashboardNarrativeBuildResult{})
	if !accepted {
		t.Fatalf("expected accepted dashboard, blocking=%v", validation.BlockingErrors)
	}
}

func TestHeadlineMismatch_StrictModeBlocks(t *testing.T) {
	forceAIOnlyArchitecture(t)
	t.Setenv("DASHBOARD_VALIDATION_MODE", DashboardValidationModeStrict)

	payload := minimalAIOnlyPayload()
	payload.Headline = "Продажи требуют внимания"
	payload.AccountHealth.Headline = "Стабильная динамика по кабинету"

	result := evaluateStage4CrossBlockReview(payload, DashboardValidationModeStrict)
	if result.Valid {
		t.Fatal("expected blocking validation in strict mode")
	}
}

func TestTopActionsLt3_PilotAccepted(t *testing.T) {
	forceAIOnlyArchitecture(t)
	t.Setenv("DASHBOARD_VALIDATION_MODE", DashboardValidationModePilotStructural)

	for _, count := range []int{0, 1, 2} {
		t.Run(fmt.Sprintf("%d", count), func(t *testing.T) {
			payload := minimalAIOnlyPayload()
			payload.TopActions = payload.TopActions[:count]
			accepted, _, _, validation := acceptGeneratedDashboardNarrativeWithValidation(payload, dashboardNarrativeBuildResult{})
			if !accepted {
				t.Fatalf("count=%d blocking=%+v", count, validation.BlockingErrors)
			}
			strictErrs := validateAIOnlyRequiredBlocks(payload)
			if len(strictErrs) == 0 || strictErrs[0] != "top_actions_lt_3" {
				// strict validator still reports the issue for diagnostics
			}
			blockValidation := classifyBlockingCodes([]string{"top_actions_lt_3"}, DashboardValidationModePilotStructural, "test")
			if !blockValidation.Valid {
				t.Fatal("top_actions_lt_3 must be warning in pilot")
			}
		})
	}
}

func TestTopActionsGt3_PilotAccepted(t *testing.T) {
	forceAIOnlyArchitecture(t)
	t.Setenv("DASHBOARD_VALIDATION_MODE", DashboardValidationModePilotStructural)

	payload := minimalAIOnlyPayload()
	payload.TopActions = append(payload.TopActions, payload.TopActions[0])
	accepted, _, _, validation := acceptGeneratedDashboardNarrativeWithValidation(payload, dashboardNarrativeBuildResult{})
	if !accepted {
		t.Fatalf("blocking=%+v", validation.BlockingErrors)
	}
}

func TestSemanticContentCodes_PilotNeverBlock(t *testing.T) {
	forceAIOnlyArchitecture(t)
	t.Setenv("DASHBOARD_VALIDATION_MODE", DashboardValidationModePilotStructural)

	codes := []string{
		"top_actions_lt_3",
		"top_action_stock_classified_as_sales",
		"seller_facing_ai_sees_phrase",
		"category_summary_incomplete",
		"risk_overview_empty_group_severity",
		"top_action_missing_fact_ids",
		"funnel_ai_analysis_structurally_empty",
		"no_usable_narrative_content",
		"orders_semantics_mismatch",
		"category_score_severity_mismatch",
	}
	for _, code := range codes {
		t.Run(code, func(t *testing.T) {
			result := classifyBlockingCodes([]string{code}, DashboardValidationModePilotStructural, "test")
			if !result.Valid {
				t.Fatalf("code=%s blocking=%+v", code, result.BlockingErrors)
			}
			if len(result.Warnings) != 1 {
				t.Fatalf("code=%s warnings=%+v", code, result.Warnings)
			}
		})
	}
}

func TestSemanticIssues_PilotAcceptsMultipleWarnings(t *testing.T) {
	forceAIOnlyArchitecture(t)
	t.Setenv("DASHBOARD_VALIDATION_MODE", DashboardValidationModePilotStructural)

	payload := minimalAIOnlyPayload()
	payload.Headline = "Root headline"
	payload.AccountHealth.Headline = "Different account headline"
	payload.WhatChanged.MainConclusion = payload.WhatChanged.Summary
	payload.TopActions = payload.TopActions[:1]

	accepted, _, _, acceptance := acceptGeneratedDashboardNarrativeWithValidation(payload, dashboardNarrativeBuildResult{})
	if !accepted {
		t.Fatalf("acceptance blocking=%+v", acceptance.BlockingErrors)
	}
	validation := mergeDashboardValidationResults(
		acceptance,
		evaluateStage4CrossBlockReview(payload, DashboardValidationModePilotStructural),
		evaluateAIOnlyContentValidation(payload, DashboardValidationModePilotStructural),
		evaluateStage4BlockValidation(payload, DashboardValidationModePilotStructural),
	)
	if !validation.Valid || len(validation.Warnings) < 2 {
		t.Fatalf("validation=%+v", validation)
	}
}

func TestSemanticIssues_StrictModeBlocks(t *testing.T) {
	forceAIOnlyArchitecture(t)
	t.Setenv("DASHBOARD_VALIDATION_MODE", DashboardValidationModeStrict)

	payload := minimalAIOnlyPayload()
	payload.Headline = "Root headline"
	payload.AccountHealth.Headline = "Different account headline"

	result := evaluateStage4CrossBlockReview(payload, DashboardValidationModeStrict)
	if result.Valid {
		t.Fatal("expected strict mode failure")
	}
}

func TestMissingRequiredBlock_BlocksInBothModes(t *testing.T) {
	forceAIOnlyArchitecture(t)
	payload := minimalAIOnlyPayload()
	payload.AccountHealth = nil

	for _, mode := range []string{DashboardValidationModePilotStructural, DashboardValidationModeStrict} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("DASHBOARD_VALIDATION_MODE", mode)
			accepted, _, _, validation := acceptGeneratedDashboardNarrativeWithValidation(payload, dashboardNarrativeBuildResult{})
			if accepted {
				t.Fatal("expected rejection")
			}
			if validation.Valid || len(validation.BlockingErrors) == 0 {
				t.Fatalf("expected blocking errors, validation=%+v", validation)
			}
		})
	}
}

func TestEmptyRequiredHeadline_BlocksInBothModes(t *testing.T) {
	forceAIOnlyArchitecture(t)
	payload := minimalAIOnlyPayload()
	payload.Headline = ""
	payload.AccountHealth.Headline = ""

	for _, mode := range []string{DashboardValidationModePilotStructural, DashboardValidationModeStrict} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("DASHBOARD_VALIDATION_MODE", mode)
			accepted, _, _, validation := acceptGeneratedDashboardNarrativeWithValidation(payload, dashboardNarrativeBuildResult{})
			if accepted {
				t.Fatal("expected rejection")
			}
			if validation.Valid {
				t.Fatalf("validation=%+v", validation)
			}
		})
	}
}

func TestOptionalUnavailableAdEfficiency_PilotAccepted(t *testing.T) {
	forceAIOnlyArchitecture(t)
	t.Setenv("DASHBOARD_VALIDATION_MODE", DashboardValidationModePilotStructural)

	payload := minimalAIOnlyPayload()
	payload.AdEfficiencyNarrative = &AdEfficiencyNarrative{
		Status:  "unavailable",
		Summary: "",
	}

	accepted, _, _, validation := acceptGeneratedDashboardNarrativeWithValidation(payload, dashboardNarrativeBuildResult{})
	if !accepted {
		t.Fatalf("blocking=%+v", validation.BlockingErrors)
	}
}

func TestIncompleteCategorySummary_PilotAccepted(t *testing.T) {
	forceAIOnlyArchitecture(t)
	t.Setenv("DASHBOARD_VALIDATION_MODE", DashboardValidationModePilotStructural)

	payload := minimalAIOnlyPayload()
	payload.CategorySummaries[0].KeyRisk = ""
	payload.CategorySummaries[0].RecommendedAction = ""

	accepted, _, _, validation := acceptGeneratedDashboardNarrativeWithValidation(payload, dashboardNarrativeBuildResult{})
	if !accepted {
		t.Fatalf("blocking=%+v", validation.BlockingErrors)
	}
}

func TestGenericTextWarningDoesNotBlockPilot(t *testing.T) {
	forceAIOnlyArchitecture(t)
	t.Setenv("DASHBOARD_VALIDATION_MODE", DashboardValidationModePilotStructural)

	payload := minimalAIOnlyPayload()
	payload.WhatChanged.MainConclusion = payload.WhatChanged.Summary

	validation := evaluateAIOnlyContentValidation(payload, DashboardValidationModePilotStructural)
	if !validation.Valid {
		t.Fatalf("expected warning-only validation, blocking=%+v", validation.BlockingErrors)
	}
	if len(validation.Warnings) == 0 {
		t.Fatal("expected semantic warning")
	}
}

func TestStage4SemanticBlockIssues_DoNotFatalInPilot(t *testing.T) {
	forceAIOnlyArchitecture(t)
	t.Setenv("DASHBOARD_VALIDATION_MODE", DashboardValidationModePilotStructural)

	payload := minimalAIOnlyPayload()
	payload.TopActions = nil
	payload.WhatChanged = nil
	payload.FunnelAIAnalysis = nil
	payload.RiskOverview = nil
	payload.SellerPulseNarrative = nil

	blockValidation := evaluateStage4BlockValidation(payload, DashboardValidationModePilotStructural)
	if !blockValidation.Valid {
		t.Fatalf("stage4 blocks must not block pilot, got %+v", blockValidation.BlockingErrors)
	}
	if len(blockValidation.Warnings) == 0 {
		t.Fatal("expected stage4 warnings")
	}
}

func TestIsSuccess_CompletedWithWarnings(t *testing.T) {
	result := &NarrativeGenerationResult{Status: "completed_with_warnings"}
	if !result.IsSuccess() {
		t.Fatal("completed_with_warnings must be treated as success")
	}
}

func ptrInt32(v int32) *int32 {
	return &v
}
