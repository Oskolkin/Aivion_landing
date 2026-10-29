package outputvalidator

import (
	"fmt"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/thresholdpolicy"
)

const (
	CodeAdvertisingThresholdUnknownPolicy   = "advertising_threshold_unknown_policy"
	CodeAdvertisingThresholdPolicyMismatch  = "advertising_threshold_policy_mismatch"
	CodeAdvertisingThresholdMissingPolicyID = "advertising_threshold_missing_policy_id"
)

// validateAdvertisingThresholdClaims checks every structured threshold_claim
// against the versioned policy registry. Unknown IDs, wrong metric/scope/
// operator/value are hard errors — AI may not invent thresholds.
// Returns the values of claims that passed validation (for narrative grounding).
func validateAdvertisingThresholdClaims(report *Report, narrative *contract.UnifiedAdEfficiencyNarrativeV1) []float64 {
	if report == nil || narrative == nil {
		return nil
	}
	base := "$.dashboard.ad_efficiency.narrative.threshold_claims"
	valid := make([]float64, 0, len(narrative.ThresholdClaims))
	for i, claim := range narrative.ThresholdClaims {
		path := fmt.Sprintf("%s[%d]", base, i)
		before := len(report.Errors)
		policyID := strings.TrimSpace(claim.PolicyID)
		if policyID == "" {
			report.addError(CodeAdvertisingThresholdMissingPolicyID, path+".policy_id", "policy_id is required")
			continue
		}
		reg, ok := thresholdpolicy.Lookup(policyID)
		if !ok {
			report.addError(CodeAdvertisingThresholdUnknownPolicy, path+".policy_id",
				fmt.Sprintf("unknown threshold policy_id %q", policyID))
			continue
		}
		if !thresholdpolicy.MatchExact(policyID, strings.TrimSpace(claim.MetricCode), strings.TrimSpace(claim.Scope), strings.TrimSpace(claim.Operator), claim.Value) {
			report.addError(CodeAdvertisingThresholdPolicyMismatch, path,
				fmt.Sprintf("threshold claim does not match registry policy %q (want metric=%s scope=%s op=%s value=%g)",
					reg.PolicyID, reg.MetricCode, reg.Scope, reg.Operator, reg.Value))
		}
		if len(report.Errors) == before {
			valid = append(valid, claim.Value)
		}
	}
	return valid
}
