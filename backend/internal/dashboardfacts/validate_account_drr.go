package dashboardfacts

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const accountDRRMatchTolerance = 0.15 // percent points; presentation rounds to 0.1

// validateAccountDRRConsistency verifies Pulse (store-revenue share) and
// Advertising (attributed-revenue share) are each internally consistent.
// The two metrics MUST be allowed to differ — they use different denominators.
func validateAccountDRRConsistency(facts DashboardFactsV1) []FactsValidationIssue {
	var issues []FactsValidationIssue

	pulseKPI := findKPI(facts.SellerPulse.KPIs, "drr")
	ad := facts.AdEfficiency
	if ad.Status != "available" && ad.Status != "partial" {
		return issues
	}

	if pulseKPI != nil {
		if _, ok := parsePercentValue(pulseKPI.Value); !ok {
			issues = append(issues, FactsValidationIssue{
				Code: "account_drr_source_mismatch", Path: "seller_pulse.kpis[drr].value",
				Message: fmt.Sprintf("cannot parse pulse store-revenue share %q", pulseKPI.Value),
			})
		}
	}

	if ad.Summary != nil && ad.Summary.CurrentDRRPct == nil && ad.Summary.TotalSpendToday > 0 && ad.Summary.TotalRevenueToday > 0 {
		issues = append(issues, FactsValidationIssue{
			Code: "account_drr_source_mismatch", Path: "ad_efficiency.summary.current_drr_pct",
			Message: "ad_efficiency attributed share is nil while spend and attributed revenue are present",
		})
	}

	// Do NOT require pulse value == ad current_drr_pct (different denominators).
	issues = append(issues, validateAccountDRRNotRepeatedAggregate(ad.Trend7D)...)
	issues = append(issues, validateAccountDRRPeriodAggregate(ad)...)
	issues = append(issues, validateAttributedDailyShareMatchesTrend(ad)...)
	issues = append(issues, validateCampaignIDsBelongToSeller(facts)...)

	return issues
}

func validateAttributedDailyShareMatchesTrend(ad AdEfficiencyFacts) []FactsValidationIssue {
	var issues []FactsValidationIssue
	for _, pt := range ad.Trend7D {
		if pt.DataStatus != "available" || pt.DRRPct == nil || pt.Revenue <= 0 {
			continue
		}
		want := math.Round((pt.Spend/pt.Revenue*100)*100) / 100
		if math.Abs(*pt.DRRPct-want) > accountDRRMatchTolerance {
			issues = append(issues, FactsValidationIssue{
				Code: "account_drr_daily_series_mismatch",
				Path: fmt.Sprintf("ad_efficiency.trend_7d[%s].drr_pct", pt.Date),
				Message: fmt.Sprintf(
					"daily attributed share on %s: got %.2f want spend/attributed=%.2f",
					pt.Date, *pt.DRRPct, want,
				),
			})
		}
	}
	return issues
}

func validateAccountDRRNotRepeatedAggregate(trend []AdEfficiencyTrendPoint) []FactsValidationIssue {
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
			return []FactsValidationIssue{{
				Code:    "account_drr_repeated_aggregate_series",
				Path:    "ad_efficiency.trend_7d",
				Message: "daily share series repeats one aggregate that does not match daily spend/attributed revenue",
			}}
		}
	}
	return nil
}

func validateAccountDRRPeriodAggregate(ad AdEfficiencyFacts) []FactsValidationIssue {
	if ad.Summary == nil || ad.Summary.AverageDRR7dPct == nil {
		return nil
	}
	var spendSum, revSum float64
	days := 0
	for _, pt := range ad.Trend7D {
		if pt.DataStatus != "available" || pt.Revenue <= 0 {
			continue
		}
		spendSum += pt.Spend
		revSum += pt.Revenue
		days++
	}
	if days == 0 || revSum <= 0 {
		return nil
	}
	want := math.Round((spendSum/revSum*100)*100) / 100
	if math.Abs(*ad.Summary.AverageDRR7dPct-want) > accountDRRMatchTolerance {
		return []FactsValidationIssue{{
			Code: "account_drr_invalid_aggregation",
			Path: "ad_efficiency.summary.average_drr_7d_pct",
			Message: fmt.Sprintf(
				"7d attributed share %.2f != spend_sum/attributed_sum %.2f",
				*ad.Summary.AverageDRR7dPct, want,
			),
		}}
	}
	return nil
}

func validateCampaignIDsBelongToSeller(facts DashboardFactsV1) []FactsValidationIssue {
	sellerID := facts.SellerAccountID
	if sellerID <= 0 {
		return nil
	}
	check := func(rows []AdCampaignDisplayRow, path string) []FactsValidationIssue {
		var out []FactsValidationIssue
		for i, row := range rows {
			if row.CampaignExternalID <= 0 {
				out = append(out, FactsValidationIssue{
					Code:    "account_drr_campaign_scope_incomplete",
					Path:    fmt.Sprintf("%s[%d]", path, i),
					Message: "campaign external id must be positive",
				})
			}
		}
		return out
	}
	var issues []FactsValidationIssue
	issues = append(issues, check(facts.AdEfficiency.HighDRRCampaigns, "ad_efficiency.high_drr_campaigns")...)
	issues = append(issues, check(facts.AdEfficiency.NoOrderCampaigns, "ad_efficiency.no_order_campaigns")...)
	return issues
}

func findKPI(kpis []KPICardFact, key string) *KPICardFact {
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
