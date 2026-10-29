package bundlevalidator

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

const accountDRRMatchTolerance = 0.15

var accountDRRNarrativeRe = regexp.MustCompile(`(?i)(?:общий\s+)?(?:drr|дрр)\s*(?:кабинета|кабин\w*|account|общий)?[^0-9%]{0,40}(\d+[.,]\d+|\d+)\s*%`)
var cabinetDRRContextRe = regexp.MustCompile(`(?i)(?:кабинет|account|общ\w+\s+(?:выручк|дрр)|дрр\s+кабин|общей\s+выручк|drr\s+today|дрр\s+сегодня|нагрузк\w+\s+на\s+(?:весь\s+)?кабинет)`)
var attributedRevenueContextRe = regexp.MustCompile(`(?i)(?:атрибут\w+|выручк\w+\s+с\s+реклам|рекламн\w+\s+выручк|окупаемост\w+\s+реклам)`)
var recommendationForbiddenRe = regexp.MustCompile(`(?i)(?:\bрекоменд|\bнужно\b|\bследует\b|\bпроверьте\b|\bостановите\b|\bпополните\b|\bперераспредел)`)
var bareCabinetDRRRe = regexp.MustCompile(`(?i)(?:(?:общий\s+)?(?:drr|дрр)\s+(?:кабинета|кабин\w*|account)|(?:общий\s+)(?:drr|дрр)|(?:drr|дрр)\s+кабин)`)

func checkAccountDRRConsistency(in Input) []Issue {
	frozen := in.Projection.Frozen
	facts := in.Facts

	adSummary := frozen.AdEfficiency.Summary
	if adSummary == nil {
		return nil
	}
	if isExplicitNoDataStatus(string(frozen.AdEfficiency.Status)) {
		return nil
	}

	var issues []Issue

	// Pulse (store share) and Advertising (attributed share) are intentionally different.
	// Only verify frozen advertising share matches facts artifact when both present.
	if facts.AdEfficiency.Summary != nil && facts.AdEfficiency.Summary.CurrentDRRPct != nil && adSummary.CurrentDRRPct != nil {
		if math.Abs(*facts.AdEfficiency.Summary.CurrentDRRPct-*adSummary.CurrentDRRPct) > accountDRRMatchTolerance {
			issues = append(issues, Issue{
				Code:    CodeAccountDRRSourceMismatch,
				Field:   "ad_efficiency.summary.current_drr_pct",
				Message: "frozen advertising attributed share diverges from facts artifact",
			})
		}
	}

	issues = append(issues, checkAccountDRRPeriodAggregate(frozen.AdEfficiency)...)
	issues = append(issues, checkAccountDRRRepeatedAggregate(frozen.AdEfficiency.Trend7D)...)
	issues = append(issues, checkAttributedDailyShare(frozen.AdEfficiency.Trend7D)...)
	issues = append(issues, checkAccountDRRCampaignScope(frozen.AdEfficiency)...)
	issues = append(issues, checkAdvertisingNarrativeArithmetic(frozen.AdEfficiency)...)
	issues = append(issues, checkAdvertisingNarrativeNoRecommendation(frozen.AdEfficiency)...)

	return issues
}

func checkAttributedDailyShare(trend []dashboardhome.AdEfficiencyTrendPoint) []Issue {
	var issues []Issue
	for _, pt := range trend {
		if pt.DataStatus != "available" || pt.DRRPct == nil || pt.Revenue <= 0 {
			continue
		}
		want := math.Round((pt.Spend/pt.Revenue*100)*100) / 100
		if math.Abs(*pt.DRRPct-want) > accountDRRMatchTolerance {
			issues = append(issues, Issue{
				Code:  CodeAccountDRRDailySeriesMismatch,
				Field: fmt.Sprintf("ad_efficiency.trend_7d[%s].drr_pct", pt.Date),
				Message: fmt.Sprintf(
					"daily attributed share on %s: got %.2f want %.2f",
					pt.Date, *pt.DRRPct, want,
				),
			})
		}
	}
	return issues
}

func checkAccountDRRPeriodAggregate(ad dashboardhome.AdEfficiencyWidget) []Issue {
	if ad.Summary == nil || ad.Summary.AverageDRR7dPct == nil {
		return nil
	}
	var spendSum, revSum float64
	for _, pt := range ad.Trend7D {
		if pt.DataStatus != "available" || pt.Revenue <= 0 {
			continue
		}
		spendSum += pt.Spend
		revSum += pt.Revenue
	}
	if revSum <= 0 {
		return nil
	}
	want := math.Round((spendSum/revSum*100)*100) / 100
	if math.Abs(*ad.Summary.AverageDRR7dPct-want) > accountDRRMatchTolerance {
		return []Issue{{
			Code:  CodeAccountDRRInvalidAggregation,
			Field: "ad_efficiency.summary.average_drr_7d_pct",
			Message: fmt.Sprintf(
				"7d attributed share %.2f != spend_sum/attributed_sum %.2f",
				*ad.Summary.AverageDRR7dPct, want,
			),
		}}
	}
	return nil
}

func checkAccountDRRRepeatedAggregate(trend []dashboardhome.AdEfficiencyTrendPoint) []Issue {
	type day struct {
		drr, spend, revenue float64
	}
	var days []day
	for _, pt := range trend {
		if pt.DataStatus != "available" || pt.DRRPct == nil || pt.Revenue <= 0 {
			continue
		}
		days = append(days, day{drr: *pt.DRRPct, spend: pt.Spend, revenue: pt.Revenue})
	}
	if len(days) < 3 {
		return nil
	}
	first := days[0].drr
	allSame := true
	for i := 1; i < len(days); i++ {
		if math.Abs(days[i].drr-first) > 0.01 {
			allSame = false
			break
		}
	}
	if !allSame {
		return nil
	}
	for _, d := range days {
		expected := math.Round((d.spend/d.revenue*100)*100) / 100
		if math.Abs(expected-first) > accountDRRMatchTolerance {
			return []Issue{{
				Code:    CodeAccountDRRRepeatedAggregateSeries,
				Field:   "ad_efficiency.trend_7d",
				Message: "daily share series repeats one aggregate that does not match daily spend/attributed revenue",
			}}
		}
	}
	return nil
}

func checkAccountDRRCampaignScope(ad dashboardhome.AdEfficiencyWidget) []Issue {
	var issues []Issue
	for i, row := range ad.HighDRRCampaigns {
		if row.CampaignExternalID <= 0 {
			issues = append(issues, Issue{
				Code:    CodeAccountDRRCampaignScopeIncomplete,
				Field:   fmt.Sprintf("ad_efficiency.high_drr_campaigns[%d]", i),
				Message: "campaign external id must be positive",
			})
		}
	}
	for i, row := range ad.NoOrderCampaigns {
		if row.CampaignExternalID <= 0 {
			issues = append(issues, Issue{
				Code:    CodeAccountDRRCampaignScopeIncomplete,
				Field:   fmt.Sprintf("ad_efficiency.no_order_campaigns[%d]", i),
				Message: "campaign external id must be positive",
			})
		}
	}
	return issues
}

func checkAdvertisingNarrativeNoRecommendation(ad dashboardhome.AdEfficiencyWidget) []Issue {
	if ad.Narrative == nil {
		return nil
	}
	if strings.TrimSpace(ad.Narrative.Recommendation) != "" {
		return []Issue{{
			Code:    "advertising_recommendation_not_allowed",
			Field:   "ad_efficiency.narrative.recommendation",
			Message: "advertising AI narrative must not include recommendation; actions belong in Actions",
		}}
	}
	text := strings.TrimSpace(ad.Narrative.Headline + " " + ad.Narrative.Summary)
	if recommendationForbiddenRe.MatchString(text) {
		return []Issue{{
			Code:    "advertising_recommendation_not_allowed",
			Field:   "ad_efficiency.narrative.summary",
			Message: "advertising headline/summary must not contain recommendations or imperative steps",
		}}
	}
	return nil
}

// Narrative percent vs summary-fact tolerance for claim-aware share checks.
// Display rounding (26.23 → 26,2%) and day vs 7d aggregate noise stay within this band.
const advertisingNarrativeShareTolerance = 1.5

// Canonical advertising percent metric codes (must match outputvalidator).
const (
	adMetricStoreSpendSharePct    = "ad_spend_share_of_total_store_revenue_pct"
	adMetricAttributedCampaignDRR = "attributed_campaign_drr_pct"
	adMetricCampaignDRRPct        = "campaign_drr_pct"
)

var freeTextPercentRe = regexp.MustCompile(`(\d+[.,]\d+|\d+)\s*%`)

// checkAdvertisingNarrativeArithmetic validates advertising narrative percentages.
//
// When structured metric_claims / threshold_claims are present (schema v9+/v10+),
// metric identity comes ONLY from those claims — never from Russian wording
// heuristics (кабинет / ДРР / атрибут…). Narrative-wide “any % near cabinet =
// store share” scanning caused run 205's attributed_campaign_drr_pct 53.3% to
// be compared against store spend share ~26.6%.
//
// Legacy publications without claims keep the older sentence-context scan as a
// fail-closed store-share / bare-DRR check only.
func checkAdvertisingNarrativeArithmetic(ad dashboardhome.AdEfficiencyWidget) []Issue {
	if ad.Narrative == nil || ad.Summary == nil {
		return nil
	}
	text := strings.TrimSpace(strings.Join([]string{
		ad.Narrative.Headline,
		ad.Narrative.Summary,
	}, " "))
	if text == "" {
		return nil
	}

	if len(ad.Narrative.MetricClaims) > 0 || len(ad.Narrative.ThresholdClaims) > 0 {
		return checkAdvertisingNarrativeClaimAware(ad, text)
	}
	return checkAdvertisingNarrativeLegacyHeuristics(ad, text)
}

func checkAdvertisingNarrativeClaimAware(ad dashboardhome.AdEfficiencyWidget, text string) []Issue {
	var issues []Issue
	n := ad.Narrative

	percentClaims := collectAdvertisingPercentClaims(n)
	issues = append(issues, checkFreeTextPercentsGroundedByClaims(text, percentClaims)...)
	issues = append(issues, checkAdvertisingClaimsAgainstSummaryFacts(ad.Summary, n)...)
	return issues
}

type advertisingPercentClaim struct {
	MetricCode  string
	Value       float64
	FactID      string
	PeriodType  string
	IsThreshold bool
	JSONPath    string
}

func collectAdvertisingPercentClaims(n *dashboardhome.AdEfficiencyNarrative) []advertisingPercentClaim {
	if n == nil {
		return nil
	}
	out := make([]advertisingPercentClaim, 0, len(n.MetricClaims)+len(n.ThresholdClaims))
	for i, c := range n.MetricClaims {
		mc := strings.TrimSpace(c.MetricCode)
		if c.Value == nil {
			continue
		}
		if mc != adMetricStoreSpendSharePct && mc != adMetricAttributedCampaignDRR && mc != adMetricCampaignDRRPct {
			continue
		}
		out = append(out, advertisingPercentClaim{
			MetricCode: mc,
			Value:      *c.Value,
			FactID:     strings.TrimSpace(c.FactID),
			PeriodType: strings.TrimSpace(c.PeriodType),
			JSONPath:   fmt.Sprintf("ad_efficiency.narrative.metric_claims[%d]", i),
		})
	}
	for i, c := range n.ThresholdClaims {
		out = append(out, advertisingPercentClaim{
			MetricCode:  strings.TrimSpace(c.MetricCode),
			Value:       c.Value,
			IsThreshold: true,
			JSONPath:    fmt.Sprintf("ad_efficiency.narrative.threshold_claims[%d]", i),
		})
	}
	return out
}

func checkFreeTextPercentsGroundedByClaims(text string, claims []advertisingPercentClaim) []Issue {
	var issues []Issue
	for _, pct := range parseFreeTextPercents(text) {
		if len(matchingPercentClaims(pct, claims)) == 0 {
			issues = append(issues, Issue{
				Code:  "advertising_claim_percent_without_claim",
				Field: "ad_efficiency.narrative.summary",
				Message: fmt.Sprintf(
					"free-text percentage %.1f%% has no matching validated metric_claim or threshold_claim value",
					pct,
				),
			})
		}
	}
	return issues
}

func matchingPercentClaims(pct float64, claims []advertisingPercentClaim) []advertisingPercentClaim {
	var out []advertisingPercentClaim
	for _, c := range claims {
		if percentMatchesClaimValue(pct, c.Value) {
			out = append(out, c)
		}
	}
	return out
}

// percentMatchesClaimValue mirrors outputvalidator.PercentMatchesClaim
// (display rounding of claim values to 0/1/2 decimals).
func percentMatchesClaimValue(textValue, claimValue float64) bool {
	const eps = 1e-9
	if math.Abs(textValue-claimValue) <= 0.05+eps {
		return true
	}
	for _, places := range []float64{1, 10, 100} {
		rounded := math.Round(claimValue*places) / places
		if math.Abs(textValue-rounded) <= 0.05+eps {
			return true
		}
	}
	return false
}

func parseFreeTextPercents(text string) []float64 {
	matches := freeTextPercentRe.FindAllStringSubmatch(text, -1)
	out := make([]float64, 0, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		raw := strings.ReplaceAll(m[1], ",", ".")
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			continue
		}
		out = append(out, v)
	}
	return out
}

func checkAdvertisingClaimsAgainstSummaryFacts(
	summary *dashboardhome.AdEfficiencySummary,
	n *dashboardhome.AdEfficiencyNarrative,
) []Issue {
	if summary == nil || n == nil {
		return nil
	}
	var issues []Issue
	for i, c := range n.MetricClaims {
		if c.Value == nil {
			continue
		}
		path := fmt.Sprintf("ad_efficiency.narrative.metric_claims[%d]", i)
		mc := strings.TrimSpace(c.MetricCode)
		switch mc {
		case adMetricStoreSpendSharePct:
			want := expectedStoreSpendSharePct(summary, c.PeriodType)
			if want == nil {
				continue
			}
			if math.Abs(*c.Value-*want) > advertisingNarrativeShareTolerance {
				issues = append(issues, Issue{
					Code:  "advertising_total_revenue_share_mismatch",
					Field: path,
					Message: fmt.Sprintf(
						"store-spend-share claim %.1f%% conflicts with facts (~%.1f%%)",
						*c.Value, *want,
					),
				})
			}
			if looksLikeCampaignFactID(c.FactID) {
				issues = append(issues, Issue{
					Code:    "advertising_store_share_campaign_fact",
					Field:   path + ".fact_id",
					Message: "store spend share claim must not use a campaign-level fact_id",
				})
			}
		case adMetricAttributedCampaignDRR:
			want := expectedAttributedCampaignDRRPct(summary, c.PeriodType)
			if want == nil {
				continue
			}
			if math.Abs(*c.Value-*want) > advertisingNarrativeShareTolerance {
				issues = append(issues, Issue{
					Code:  "advertising_attributed_revenue_share_mismatch",
					Field: path,
					Message: fmt.Sprintf(
						"attributed campaign DRR claim %.1f%% conflicts with facts (~%.1f%%)",
						*c.Value, *want,
					),
				})
			}
		case adMetricCampaignDRRPct:
			// Campaign-level DRR is validated against Pack campaign facts in
			// outputvalidator. Bundle must not compare it to store share or
			// account attributed aggregates.
			if strings.TrimSpace(c.FactID) == "" {
				issues = append(issues, Issue{
					Code:    "advertising_campaign_drr_fact_missing",
					Field:   path + ".fact_id",
					Message: "campaign_drr_pct claim requires a fact_id",
				})
			} else if !looksLikeCampaignFactID(c.FactID) {
				issues = append(issues, Issue{
					Code:    "advertising_campaign_drr_fact_scope",
					Field:   path + ".fact_id",
					Message: "campaign_drr_pct claim fact_id must be campaign-scoped",
				})
			}
		}
	}
	for i, c := range n.ThresholdClaims {
		path := fmt.Sprintf("ad_efficiency.narrative.threshold_claims[%d]", i)
		if strings.TrimSpace(c.PolicyID) == "" {
			issues = append(issues, Issue{
				Code:    "advertising_threshold_policy_missing",
				Field:   path + ".policy_id",
				Message: "threshold claim requires policy_id",
			})
		}
		// Thresholds are policy values — never compare to store/attributed shares.
	}
	return issues
}

func looksLikeCampaignFactID(factID string) bool {
	return strings.HasPrefix(strings.TrimSpace(factID), "campaign:")
}

func expectedStoreSpendSharePct(summary *dashboardhome.AdEfficiencySummary, periodType string) *float64 {
	use7d := periodType == "current_7d" || periodType == "previous_7d"
	if use7d && summary.TotalStoreRevenue7d > 0 {
		v := math.Round((summary.TotalSpend7d/summary.TotalStoreRevenue7d*100)*10) / 10
		return &v
	}
	if summary.TotalStoreRevenueToday > 0 && summary.TotalSpendToday > 0 {
		v := math.Round((summary.TotalSpendToday/summary.TotalStoreRevenueToday*100)*10) / 10
		return &v
	}
	if summary.TotalStoreRevenue7d > 0 {
		v := math.Round((summary.TotalSpend7d/summary.TotalStoreRevenue7d*100)*10) / 10
		return &v
	}
	return nil
}

func expectedAttributedCampaignDRRPct(summary *dashboardhome.AdEfficiencySummary, periodType string) *float64 {
	use7d := periodType == "current_7d" || periodType == "previous_7d"
	if use7d {
		if summary.AverageDRR7dPct != nil {
			return summary.AverageDRR7dPct
		}
		if summary.TotalAttributedRevenue7d > 0 {
			v := math.Round((summary.TotalSpend7d/summary.TotalAttributedRevenue7d*100)*10) / 10
			return &v
		}
	}
	if summary.CurrentDRRPct != nil {
		return summary.CurrentDRRPct
	}
	if summary.AverageDRR7dPct != nil {
		return summary.AverageDRR7dPct
	}
	if summary.TotalAttributedRevenue7d > 0 {
		v := math.Round((summary.TotalSpend7d/summary.TotalAttributedRevenue7d*100)*10) / 10
		return &v
	}
	return nil
}

// checkAdvertisingNarrativeLegacyHeuristics is the pre-claim path for
// publications that never carried metric_claims. Do not use for claim-bearing
// narratives: Russian sentence context misclassifies attributed/campaign DRR
// as store spend share (see run 205 / advertising_total_revenue_share_mismatch).
func checkAdvertisingNarrativeLegacyHeuristics(ad dashboardhome.AdEfficiencyWidget, text string) []Issue {
	attributedShare := ad.Summary.CurrentDRRPct
	periodShare := ad.Summary.AverageDRR7dPct
	storeShareToday := (*float64)(nil)
	if ad.Summary.TotalStoreRevenueToday > 0 && ad.Summary.TotalSpendToday > 0 {
		v := math.Round((ad.Summary.TotalSpendToday/ad.Summary.TotalStoreRevenueToday*100)*10) / 10
		storeShareToday = &v
	}

	var issues []Issue
	for _, sentence := range splitSentences(text) {
		mentionsStore := cabinetDRRContextRe.MatchString(sentence)
		mentionsAttr := attributedRevenueContextRe.MatchString(sentence)
		if mentionsStore && mentionsAttr {
			issues = append(issues, Issue{
				Code:    "advertising_revenue_denominator_mixed",
				Field:   "ad_efficiency.narrative.summary",
				Message: "sentence mixes store revenue and attributed ad revenue denominators",
			})
			continue
		}
		if bareCabinetDRRRe.MatchString(sentence) && !mentionsAttr {
			issues = append(issues, Issue{
				Code:    "advertising_bare_drr_for_store_share",
				Field:   "ad_efficiency.narrative.summary",
				Message: "cabinet/account «ДРР» must not label store-revenue share",
			})
		}
		m := accountDRRNarrativeRe.FindStringSubmatch(sentence)
		if len(m) < 2 {
			continue
		}
		raw := strings.ReplaceAll(m[1], ",", ".")
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			continue
		}
		if mentionsStore && storeShareToday != nil && math.Abs(v-*storeShareToday) > advertisingNarrativeShareTolerance {
			issues = append(issues, Issue{
				Code:  "advertising_total_revenue_share_mismatch",
				Field: "ad_efficiency.narrative.summary",
				Message: fmt.Sprintf(
					"narrative store-revenue share %.1f%% conflicts with facts (~%.1f%%)",
					v, *storeShareToday,
				),
			})
		}
		if mentionsAttr && attributedShare != nil && math.Abs(v-*attributedShare) > advertisingNarrativeShareTolerance {
			if periodShare == nil || math.Abs(v-*periodShare) > advertisingNarrativeShareTolerance {
				issues = append(issues, Issue{
					Code:  "advertising_attributed_revenue_share_mismatch",
					Field: "ad_efficiency.narrative.summary",
					Message: fmt.Sprintf(
						"narrative attributed share %.1f%% conflicts with facts",
						v,
					),
				})
			}
		}
	}
	return issues
}

func splitSentences(text string) []string {
	parts := regexp.MustCompile(`[.!?…\n]+`).Split(text, -1)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func findFrozenKPI(kpis []dashboardhome.KPICard, key string) *dashboardhome.KPICard {
	for i := range kpis {
		if kpis[i].Key == key {
			return &kpis[i]
		}
	}
	return nil
}

func parsePercentValue(raw string) (float64, bool) {
	s := strings.TrimSpace(raw)
	s = strings.TrimSuffix(s, "%")
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", ".")
	s = strings.ReplaceAll(s, "\u00a0", "")
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
