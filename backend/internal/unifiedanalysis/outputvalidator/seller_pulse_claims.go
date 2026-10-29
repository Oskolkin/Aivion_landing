package outputvalidator

import (
	"fmt"
	"math"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

const claimValueRelTolerance = 0.02 // 2% relative
const claimValueAbsTolerance = 1.0  // absolute floor for small ints

func validateSellerPulseClaimGrounding(report *Report, dash *contract.UnifiedDashboardV1, idx *PackReferenceIndex, outcome string) {
	if dash == nil || !isMaterialOutcome(outcome) {
		return
	}
	n := dash.SellerPulse.Narrative
	if n == nil || idx == nil || len(idx.accountMetrics) == 0 {
		// Without indexed account_summary (unit fixtures / missing CSV) skip
		// numeric grounding; production packs always index after the CSV header fix.
		return
	}
	checkClaim := func(path string, claim contract.UnifiedMetricClaimV1) {
		scope := ParseFactIDScope(claim.FactID)
		if claim.FactID != "" && !FactScopeAllowedInSection(SectionSellerPulse, scope) {
			report.addError(CodeSellerPulseAIFactMismatch, path+".fact_id",
				fmt.Sprintf("Seller Pulse only allows account-level facts (expected scopes: %s); received %s fact %q (metric %q)",
					FormatAllowedScopes(SectionSellerPulse), scope, claim.FactID, claim.MetricCode))
			return
		}
		if IsSellerPulseForbiddenMetric(claim.MetricCode) {
			report.addError(CodeSellerPulseAIFactMismatch, path+".metric_code",
				fmt.Sprintf("Seller Pulse forbids entity-level metric %q (fact_id %q, scope %s); expected scopes: %s",
					claim.MetricCode, claim.FactID, scope, FormatAllowedScopes(SectionSellerPulse)))
			return
		}
		fact, ok := idx.LookupAccountMetric(claim.FactID, claim.MetricCode, claim.PeriodType)
		if !ok {
			// Unknown account/revenue/orders fact_ids are hard errors once indexed.
			if strings.HasPrefix(strings.TrimSpace(claim.FactID), "account:") ||
				strings.Contains(strings.ToLower(claim.MetricCode), "revenue") ||
				strings.Contains(strings.ToLower(claim.MetricCode), "orders") ||
				strings.Contains(strings.ToLower(claim.MetricCode), "drr") {
				report.addError(CodeSellerPulseAIFactMismatch, path+".fact_id",
					fmt.Sprintf("Seller Pulse section=%s fact_id %q / metric %q not found in account_summary (scope=%s, expected scopes: %s)",
						SectionSellerPulse, claim.FactID, claim.MetricCode, scope, FormatAllowedScopes(SectionSellerPulse)))
			}
			return
		}
		if claim.PeriodType != "" && fact.PeriodType != "" && claim.PeriodType != fact.PeriodType {
			report.addError(CodeSellerPulseAIPeriodMismatch, path+".period_type",
				fmt.Sprintf("period_type %q does not match fact period %q", claim.PeriodType, fact.PeriodType))
		}
		mc := strings.TrimSpace(claim.MetricCode)
		if mc != "" && fact.MetricCode != "" && !metricCodesCompatible(mc, fact.MetricCode) {
			report.addError(CodeSellerPulseAIMetricMismatch, path+".metric_code",
				fmt.Sprintf("metric_code %q does not match fact metric %q", mc, fact.MetricCode))
		}
		if claim.Value == nil {
			return
		}
		if !valuesClose(*claim.Value, fact.Value) {
			report.addError(CodeSellerPulseAIFactMismatch, path+".value",
				fmt.Sprintf("claim value %.4f diverges from pack fact %.4f", *claim.Value, fact.Value))
		}
	}
	for i, claim := range n.MetricClaims {
		checkClaim(fmt.Sprintf("$.dashboard.seller_pulse.narrative.metric_claims[%d]", i), claim)
	}
	for i, kp := range n.KeyPoints {
		for j, claim := range kp.Claims {
			checkClaim(fmt.Sprintf("$.dashboard.seller_pulse.narrative.key_points[%d].claims[%d]", i, j), claim)
		}
	}
	for i, fact := range n.ImportantFacts {
		for j, claim := range fact.Claims {
			checkClaim(fmt.Sprintf("$.dashboard.seller_pulse.narrative.important_facts[%d].claims[%d]", i, j), claim)
		}
	}
}

func metricCodesCompatible(a, b string) bool {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	if a == b {
		return true
	}
	aliases := map[string]string{
		"revenue":             "revenue_7d",
		"orders":              "orders_7d",
		"revenue_previous_7d": "revenue_7d",
		"orders_previous_7d":  "orders_7d",
		"drr":                 "ad_spend_share_of_total_revenue_pct",
		"account_drr_pct":     "account_drr_pct",
	}
	if aliases[a] == b || aliases[b] == a {
		return true
	}
	return false
}

func valuesClose(got, want float64) bool {
	diff := math.Abs(got - want)
	if diff <= claimValueAbsTolerance {
		return true
	}
	denom := math.Max(math.Abs(want), 1)
	return diff/denom <= claimValueRelTolerance
}
