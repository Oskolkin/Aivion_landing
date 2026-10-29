package outputvalidator

import (
	"fmt"
	"math"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// Canonical metric_code values the AI may use in
// ad_efficiency.narrative.metric_claims. Any other code is rejected —
// there is no free-text rewrite fallback (Wave 2: claim-based validation
// replaces the removed adapter-side text-rewrite heuristics).
const (
	AdMetricAdSpendShareOfTotalStoreRevenuePct = "ad_spend_share_of_total_store_revenue_pct"
	AdMetricAttributedCampaignDRRPct           = "attributed_campaign_drr_pct"
	AdMetricCampaignDRRPct                     = "campaign_drr_pct"
	AdMetricAdSpend                            = "ad_spend"
	AdMetricStoreTotalRevenue                  = "store_total_revenue"
	AdMetricAttributedAdRevenue                = "attributed_ad_revenue"
	AdMetricAdOrders                           = "ad_orders"
)

var adClaimAllowedMetricCodes = map[string]bool{
	AdMetricAdSpendShareOfTotalStoreRevenuePct: true,
	AdMetricAttributedCampaignDRRPct:           true,
	AdMetricCampaignDRRPct:                     true,
	AdMetricAdSpend:                            true,
	AdMetricStoreTotalRevenue:                  true,
	AdMetricAttributedAdRevenue:                true,
	AdMetricAdOrders:                           true,
}

// Percentage-bearing metric codes whose claim values may ground free-text %
// in the advertising narrative. Absolute money/count metrics do not.
var adPercentClaimMetricCodes = map[string]bool{
	AdMetricAdSpendShareOfTotalStoreRevenuePct: true,
	AdMetricAttributedCampaignDRRPct:           true,
	AdMetricCampaignDRRPct:                     true,
}

const adClaimValueRelTolerance = 0.02 // 2% relative
const adClaimValueAbsTolerance = 1.0  // absolute floor for small ints/percentages

// validateAdvertisingClaimGrounding enforces claim-based grounding for the
// advertising narrative: structured metric_claims and threshold_claims are
// validated first; every free-text percentage in headline/summary must then
// match at least one successfully validated claim value. Russian wording is
// never interpreted — only numbers and structured claims matter.
func validateAdvertisingClaimGrounding(report *Report, dash *contract.UnifiedDashboardV1, idx *PackReferenceIndex, outcome string) {
	if report == nil || dash == nil || !isMaterialOutcome(outcome) || idx == nil || !idx.HasAdvertisingData() {
		return
	}
	narrative := dash.AdEfficiency.Narrative
	if narrative == nil {
		return
	}
	base := "$.dashboard.ad_efficiency.narrative.metric_claims"

	validPercentValues := make([]float64, 0, len(narrative.MetricClaims)+len(narrative.ThresholdClaims))
	for i, claim := range narrative.MetricClaims {
		path := fmt.Sprintf("%s[%d]", base, i)
		before := len(report.Errors)
		validateAdvertisingClaimStructure(report, path, claim)
		if len(idx.accountMetrics) > 0 {
			validateAdvertisingClaimAgainstFacts(report, path, claim, idx)
		}
		if len(report.Errors) > before {
			continue
		}
		mc := strings.TrimSpace(claim.MetricCode)
		if claim.Value != nil && adPercentClaimMetricCodes[mc] {
			validPercentValues = append(validPercentValues, *claim.Value)
		}
	}

	validPercentValues = append(validPercentValues, validateAdvertisingThresholdClaims(report, narrative)...)
	validateAdvertisingFreeTextPercentClaims(report, narrative, validPercentValues)
}

// validateAdvertisingClaimStructure checks claim shape independent of Pack
// facts: a canonical metric_code, a non-empty fact_id, a valid period_type
// with period_start/period_end, and a non-nil value.
func validateAdvertisingClaimStructure(report *Report, path string, claim contract.UnifiedMetricClaimV1) {
	mc := strings.TrimSpace(claim.MetricCode)
	switch {
	case mc == "":
		report.addError(CodeAdvertisingClaimMetricMissing, path+".metric_code", "metric_code is required")
	case !adClaimAllowedMetricCodes[mc]:
		report.addError(CodeAdvertisingClaimUnknownMetricCode, path+".metric_code",
			fmt.Sprintf("metric_code %q is not one of the canonical advertising claim codes", mc))
	}
	if strings.TrimSpace(claim.FactID) == "" {
		report.addError(CodeAdvertisingClaimFactMissing, path+".fact_id", "fact_id is required")
	}
	if !validPulsePeriodType(claim.PeriodType) {
		report.addError(CodeAdvertisingClaimPeriodMissing, path+".period_type",
			"period_type must be current_day|previous_day|current_7d|previous_7d")
	}
	if strings.TrimSpace(claim.PeriodStart) == "" || strings.TrimSpace(claim.PeriodEnd) == "" {
		report.addError(CodeAdvertisingClaimPeriodMissing, path+".period_start",
			"period_start and period_end are required")
	}
	if claim.Value == nil {
		report.addError(CodeAdvertisingClaimValueMismatch, path+".value", "claim value is required")
	}
}

// validateAdvertisingClaimAgainstFacts grounds one claim against the
// indexed Pack account metrics: wrong/unresolved fact_id, metric_code that
// does not match the resolved fact, and value mismatch. When the claimed
// value actually matches the *opposite* revenue denominator's fact (store
// total revenue cited as attributed ad revenue, or vice versa), it reports
// the more specific denominator-confusion code instead of a generic
// mismatch.
func validateAdvertisingClaimAgainstFacts(report *Report, path string, claim contract.UnifiedMetricClaimV1, idx *PackReferenceIndex) {
	mc := strings.TrimSpace(claim.MetricCode)
	if mc == "" || !adClaimAllowedMetricCodes[mc] {
		return // already reported by validateAdvertisingClaimStructure
	}
	factID := strings.TrimSpace(claim.FactID)
	var fact AccountMetricFact
	var ok bool
	if factID != "" {
		fact, ok = idx.accountMetrics[factID]
		if !ok {
			report.addError(CodeAdvertisingClaimFactUnknown, path+".fact_id",
				fmt.Sprintf("fact_id %q not found in Pack account metrics", factID))
			return
		}
	} else {
		fact, ok = idx.LookupAccountMetric("", mc, claim.PeriodType)
		if !ok {
			report.addError(CodeAdvertisingClaimFactUnknown, path+".fact_id",
				fmt.Sprintf("metric %q / period %q not found in Pack account metrics", mc, claim.PeriodType))
			return
		}
	}
	if fact.MetricCode != "" && fact.MetricCode != mc {
		report.addError(CodeAdvertisingClaimMetricMismatch, path+".metric_code",
			fmt.Sprintf("metric_code %q does not match resolved Pack fact metric %q", mc, fact.MetricCode))
	}
	if claim.Value == nil {
		return
	}
	if adValuesClose(*claim.Value, fact.Value) {
		return
	}
	if opposite, ok := adOppositeDenominatorFact(idx, mc, claim.PeriodType); ok && adValuesClose(*claim.Value, opposite.Value) {
		report.addError(CodeAdvertisingRevenueDenominatorConfused, path+".value",
			fmt.Sprintf("claimed %s=%.2f matches the opposite revenue denominator (%.2f), not its own Pack fact (%.2f)",
				mc, *claim.Value, opposite.Value, fact.Value))
		return
	}
	report.addError(CodeAdvertisingClaimValueMismatch, path+".value",
		fmt.Sprintf("claim value %.4f diverges from Pack fact %.4f for %s", *claim.Value, fact.Value, mc))
}

func adOppositeDenominatorFact(idx *PackReferenceIndex, mc, periodType string) (AccountMetricFact, bool) {
	var opposite string
	switch mc {
	case AdMetricStoreTotalRevenue:
		opposite = AdMetricAttributedAdRevenue
	case AdMetricAttributedAdRevenue:
		opposite = AdMetricStoreTotalRevenue
	default:
		return AccountMetricFact{}, false
	}
	return idx.LookupAccountMetric("", opposite, periodType)
}

func adValuesClose(got, want float64) bool {
	diff := math.Abs(got - want)
	if diff <= adClaimValueAbsTolerance {
		return true
	}
	denom := math.Max(math.Abs(want), 1)
	return diff/denom <= adClaimValueRelTolerance
}

// validateAdvertisingFreeTextPercentClaims requires every free-text
// percentage in headline/summary to match at least one successfully
// validated metric_claim or threshold_claim value. Repeats of the same
// number may share one claim. Wording around the percent is ignored.
func validateAdvertisingFreeTextPercentClaims(report *Report, narrative *contract.UnifiedAdEfficiencyNarrativeV1, validClaimValues []float64) {
	if report == nil || narrative == nil {
		return
	}
	text := strings.TrimSpace(narrative.Headline + "\n" + narrative.Summary)
	if text == "" {
		return
	}
	for _, pct := range parsePercentsInText(text) {
		if percentGroundedByClaims(pct, validClaimValues) {
			continue
		}
		report.addError(CodeAdvertisingClaimPercentWithoutClaim,
			"$.dashboard.ad_efficiency.narrative.summary",
			fmt.Sprintf("free-text percentage %.1f%% has no matching validated metric_claim or threshold_claim value", pct))
	}
}

func percentGroundedByClaims(pct float64, validClaimValues []float64) bool {
	for _, v := range validClaimValues {
		if PercentMatchesClaim(pct, v) {
			return true
		}
	}
	return false
}
