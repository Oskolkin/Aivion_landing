package outputvalidator

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// Acceptance regression fixtures reconstructed from real Unified runs 190/191
// on SHA cdafd492 (2026-07-22). Validators must keep blocking these failure
// modes; the prompt fix (v1_25) teaches the model to avoid them, but we do
// NOT auto-repair invented fact IDs or rewrite advertising text.

func TestAcceptanceRun190_UngroundedPercentsBlocked(t *testing.T) {
	// Run 190: free-text 26,2% / 80,0% without matching claims. Linguistic
	// «ДРР кабинета» is no longer a separate blocker — claim values are.
	idx := &PackReferenceIndex{
		HasCampaignsAny: true,
		accountMetrics: map[string]AccountMetricFact{
			"account:ad_spend_share_of_total_store_revenue_pct:current_7d": {
				FactID:     "account:ad_spend_share_of_total_store_revenue_pct:current_7d",
				MetricCode: AdMetricAdSpendShareOfTotalStoreRevenuePct, Value: 26.2, PeriodType: "current_7d",
			},
		},
	}
	dash := &contract.UnifiedDashboardV1{
		AdEfficiency: contract.UnifiedAdEfficiencyV1{
			Narrative: &contract.UnifiedAdEfficiencyNarrativeV1{
				Status:   "available",
				Headline: "Общий ДРР кабинета остаётся завышенным",
				Summary:  "Общий ДРР кабинета составляет 26,2%, при этом доля рекламы в выручке также фигурирует как 80,0%.",
			},
		},
	}
	report := &Report{}
	validateAdvertisingNarrativeStructure(report, dash, idx, contract.AnalysisOutcomeMaterialActionsFound)
	validateAdvertisingClaimGrounding(report, dash, idx, contract.AnalysisOutcomeMaterialActionsFound)
	if !hasCode(report.Errors, CodeAdvertisingClaimPercentWithoutClaim) {
		t.Fatalf("run190: expected %s for ungrounded percents, got %+v", CodeAdvertisingClaimPercentWithoutClaim, report.Errors)
	}
}

func TestAcceptanceRun190_CorrectStoreShareLabelWithClaimsPasses(t *testing.T) {
	idx := &PackReferenceIndex{HasCampaignsAny: true}
	dash := &contract.UnifiedDashboardV1{
		AdEfficiency: contract.UnifiedAdEfficiencyV1{
			Narrative: &contract.UnifiedAdEfficiencyNarrativeV1{
				Status:   "available",
				Headline: "Доля рекламы в общей выручке кабинета умеренная, но атрибутированный ДРР кампаний высокий",
				Summary:  "Доля рекламных расходов в общей выручке кабинета составила 26,2%. Атрибутированный ДРР кампаний достиг 80,0%.",
				MetricClaims: []contract.UnifiedMetricClaimV1{
					adClaim(AdMetricAdSpendShareOfTotalStoreRevenuePct, "account:ad_spend_share_of_total_store_revenue_pct:current_7d", 26.2),
					adClaim(AdMetricAttributedCampaignDRRPct, "account:attributed_campaign_drr_pct:current_7d", 80.0),
				},
			},
		},
	}
	report := &Report{}
	validateAdvertisingNarrativeStructure(report, dash, idx, contract.AnalysisOutcomeMaterialActionsFound)
	validateAdvertisingClaimGrounding(report, dash, idx, contract.AnalysisOutcomeMaterialActionsFound)
	if len(report.Errors) != 0 {
		t.Fatalf("correct store-share wording with claims must pass, got %+v", report.Errors)
	}
}

func TestAcceptanceRun191_InventedFactIDsBlockedNoRepair(t *testing.T) {
	// Run 191 invented metric-first SKU fact IDs. HasFact must fail; there is
	// no auto-rewrite onto sku:<sku_id>:<metric>.
	idx := &PackReferenceIndex{
		SKUIds: map[string]struct{}{
			"801000100001": {},
			"801000100002": {},
			"801000100005": {},
			"801000100006": {},
			"801000100009": {},
			"801000100004": {},
			"801000100013": {},
		},
	}
	invented := []string{
		"sku:stock_coverage_days_7d:801000100001:current_7d",
		"sku:stock_coverage_days_7d:801000100002:current_7d",
		"sku:stock_coverage_days_7d:801000100006:current_7d",
		"sku:slow_turnover:801000100009:current_7d",
		"sku:orders_7d:801000100005:current_7d",
		"sku:orders_7d:801000100013:current_7d",
		"sku:revenue_7d:801000100004:current_7d",
	}
	for _, factID := range invented {
		if idx.HasFact(factID) {
			t.Fatalf("invented fact_id %q must NOT resolve (no auto-repair)", factID)
		}
	}

	report := &Report{}
	action := contract.UnifiedActionV1{
		Scope: contract.UnifiedActionScopeV1{
			ScopeType: "sku",
			EntityIDs: []string{"801000100001", "801000100002", "801000100006"},
		},
		EvidenceFactIDs: []string{
			"sku:stock_coverage_days_7d:801000100001:current_7d",
			"sku:stock_coverage_days_7d:801000100002:current_7d",
			"sku:stock_coverage_days_7d:801000100006:current_7d",
		},
		CompletionConditions: []contract.UnifiedCompletionConditionV1{{
			MetricField:    "stock_coverage_days_7d",
			BaselineFactID: "sku:stock_coverage_days_7d:801000100001:current_7d",
			BaselineValue:  floatPtr(3),
			Operator:       ">=",
			TargetValue:    floatPtr(14),
			PeriodDays:     func() *int { d := 7; return &d }(),
		}},
	}
	validateActionEvidence(report, "$.actions[0]", action, idx)
	validateCompletionConditions(report, "$.actions[0]", action, idx)
	if !hasCode(report.Errors, CodeActionEvidenceFactUnknown) {
		t.Fatalf("run191: expected %s, got %+v", CodeActionEvidenceFactUnknown, report.Errors)
	}
	if !hasCode(report.Errors, CodeActionCompletionBaselineMissing) {
		t.Fatalf("run191: expected %s for unresolved baseline, got %+v", CodeActionCompletionBaselineMissing, report.Errors)
	}
	// Correct shape still resolves when sku_id is in the second segment.
	if !idx.HasFact("sku:801000100001:stock_coverage_days_7d") {
		t.Fatal("canonical sku:<sku_id>:<metric> must resolve when sku exists")
	}
}

func TestAcceptanceRun191_FourActionsNotBlockedByCount(t *testing.T) {
	if contract.MinActionsMaterialV1 != 1 {
		t.Fatalf("MinActionsMaterialV1=%d want 1 (no quota of 5)", contract.MinActionsMaterialV1)
	}
	if contract.MinRequiredActionsMaterialV1 != 1 {
		t.Fatalf("MinRequiredActionsMaterialV1=%d want 1", contract.MinRequiredActionsMaterialV1)
	}
	report := &Report{}
	out := &contract.UnifiedAnalysisOutputV1{
		AnalysisOutcome: contract.AnalysisOutcomeMaterialActionsFound,
		Actions: []contract.UnifiedActionV1{
			{AnalysisActionID: "action_001", ExecutionTier: contract.ExecutionTierRequired, OperationalGoal: "g1", CompletionCriteria: "c1", EvidenceSummary: "e1", Steps: []contract.UnifiedActionStepV1{{Title: "s", Description: "t"}}},
			{AnalysisActionID: "action_002", ExecutionTier: contract.ExecutionTierRequired, OperationalGoal: "g2", CompletionCriteria: "c2", EvidenceSummary: "e2", Steps: []contract.UnifiedActionStepV1{{Title: "s", Description: "t"}}},
			{AnalysisActionID: "action_003", ExecutionTier: contract.ExecutionTierRequired, OperationalGoal: "g3", CompletionCriteria: "c3", EvidenceSummary: "e3", Steps: []contract.UnifiedActionStepV1{{Title: "s", Description: "t"}}},
			{AnalysisActionID: "action_004", ExecutionTier: contract.ExecutionTierRecommended, OperationalGoal: "g4", CompletionCriteria: "c4", EvidenceSummary: "e4", Steps: []contract.UnifiedActionStepV1{{Title: "s", Description: "t"}}},
		},
	}
	validateOutcomeConsistency(report, out)
	if hasCode(report.Errors, CodeActionsCountBelowMinimum) {
		t.Fatalf("4 actions must not emit %s: %+v", CodeActionsCountBelowMinimum, report.Errors)
	}
	if hasCode(report.Errors, CodeRequiredActionsCountBelowMinimum) {
		t.Fatalf("3 required must not emit required floor error: %+v", report.Errors)
	}
}
