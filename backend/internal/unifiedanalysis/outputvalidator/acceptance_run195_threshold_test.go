package outputvalidator

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/thresholdpolicy"
)

// Acceptance run 195 (SHA dee1c61) emitted a correct high_drr_campaign_threshold
// claim but free-text «превысили порог 80%» was not classified as threshold
// context, so validation still required a share metric_claim.

func validateRun195Narrative(t *testing.T, narr *contract.UnifiedAdEfficiencyNarrativeV1) *Report {
	t.Helper()
	idx := run193PackIndex()
	dash := &contract.UnifiedDashboardV1{
		AdEfficiency: contract.UnifiedAdEfficiencyV1{Narrative: narr},
	}
	report := &Report{}
	validateAdvertisingNarrativeStructure(report, dash, idx, contract.AnalysisOutcomeMaterialActionsFound)
	validateAdvertisingClaimGrounding(report, dash, idx, contract.AnalysisOutcomeMaterialActionsFound)
	return report
}

func run195Narrative(threshold []contract.UnifiedThresholdClaimV1) *contract.UnifiedAdEfficiencyNarrativeV1 {
	return &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Доля рекламы стабильна, атрибутированный ДРР умеренный",
		Summary: "За 7 дней рекламный бюджет достиг 1 848 129 руб., что даёт долю рекламы в общей выручке в 26,2%. " +
			"По атрибутированным кампаниям ДРР составил 53,3% (на уровне прошлой недели), при этом 3 кампании превысили порог 80%. " +
			"Общий вклад рекламы поддерживает динамику, но требуется перераспределение на кампании с критическим ДРР.",
		MetricClaims: []contract.UnifiedMetricClaimV1{
			adClaim(AdMetricAdSpendShareOfTotalStoreRevenuePct, "account:ad_spend_share_of_total_store_revenue_pct:current_7d", 26.231698344160172),
			adClaim(AdMetricAttributedCampaignDRRPct, "account:attributed_campaign_drr_pct:current_7d", 53.333971468501254),
		},
		ThresholdClaims: threshold,
	}
}

func TestAcceptanceRun195_ExceedThresholdPhraseWithClaimPass(t *testing.T) {
	report := validateRun195Narrative(t, run195Narrative([]contract.UnifiedThresholdClaimV1{
		highDRRThresholdClaim(),
	}))
	for _, code := range []string{
		CodeAdvertisingClaimPercentWithoutClaim,
		CodeAdvertisingThresholdPolicyMismatch,
	} {
		if hasCode(report.Errors, code) {
			t.Fatalf("run195 prose + claim must PASS, unexpected %s: %+v", code, report.Errors)
		}
	}
	if len(report.Errors) != 0 {
		t.Fatalf("run195 prose + claim must PASS, got %+v", report.Errors)
	}
}

func TestAcceptanceRun195_ExceedThresholdWithoutClaimFail(t *testing.T) {
	report := validateRun195Narrative(t, run195Narrative(nil))
	if !hasCode(report.Errors, CodeAdvertisingClaimPercentWithoutClaim) {
		t.Fatalf("FAIL: missing threshold_claim expected %s, got %+v", CodeAdvertisingClaimPercentWithoutClaim, report.Errors)
	}
}

func TestAcceptanceRun195_WrongThresholdValueFail(t *testing.T) {
	report := validateRun195Narrative(t, run195Narrative([]contract.UnifiedThresholdClaimV1{{
		PolicyID:   thresholdpolicy.PolicyHighDRRCampaignThreshold,
		MetricCode: thresholdpolicy.MetricCampaignDRRPct,
		Scope:      thresholdpolicy.ScopeCampaign,
		Operator:   thresholdpolicy.OpGT,
		Value:      75,
	}}))
	if !hasCode(report.Errors, CodeAdvertisingThresholdPolicyMismatch) {
		t.Fatalf("FAIL: wrong value expected %s, got %+v", CodeAdvertisingThresholdPolicyMismatch, report.Errors)
	}
	if !hasCode(report.Errors, CodeAdvertisingClaimPercentWithoutClaim) {
		t.Fatalf("FAIL: free-text 80%% cannot match claim value 75, expected %s, got %+v", CodeAdvertisingClaimPercentWithoutClaim, report.Errors)
	}
}

func TestAcceptanceRun195_WrongOperatorFail(t *testing.T) {
	report := validateRun195Narrative(t, run195Narrative([]contract.UnifiedThresholdClaimV1{{
		PolicyID:   thresholdpolicy.PolicyHighDRRCampaignThreshold,
		MetricCode: thresholdpolicy.MetricCampaignDRRPct,
		Scope:      thresholdpolicy.ScopeCampaign,
		Operator:   thresholdpolicy.OpGTE,
		Value:      80,
	}}))
	if !hasCode(report.Errors, CodeAdvertisingThresholdPolicyMismatch) {
		t.Fatalf("FAIL: wrong operator expected %s, got %+v", CodeAdvertisingThresholdPolicyMismatch, report.Errors)
	}
}

func TestAcceptanceRun195_WrongScopeFail(t *testing.T) {
	report := validateRun195Narrative(t, run195Narrative([]contract.UnifiedThresholdClaimV1{{
		PolicyID:   thresholdpolicy.PolicyHighDRRCampaignThreshold,
		MetricCode: thresholdpolicy.MetricCampaignDRRPct,
		Scope:      thresholdpolicy.ScopeAccount,
		Operator:   thresholdpolicy.OpGT,
		Value:      80,
	}}))
	if !hasCode(report.Errors, CodeAdvertisingThresholdPolicyMismatch) {
		t.Fatalf("FAIL: wrong scope expected %s, got %+v", CodeAdvertisingThresholdPolicyMismatch, report.Errors)
	}
}
