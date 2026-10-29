package outputvalidator

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

func adClaim(metricCode, factID string, value float64) contract.UnifiedMetricClaimV1 {
	v := value
	return contract.UnifiedMetricClaimV1{
		FactID:      factID,
		MetricCode:  metricCode,
		Value:       &v,
		PeriodType:  "current_7d",
		PeriodStart: "2026-07-02",
		PeriodEnd:   "2026-07-08",
	}
}

// TestValidateAdvertisingClaimGrounding_TwoPercentagesRequireClaims covers
// requirement 5: a paragraph with TWO free-text percentages (store share +
// attributed campaign DRR) and no metric_claims at all must be blocked —
// both percentages need their own structured claim.
func TestValidateAdvertisingClaimGrounding_TwoPercentagesRequireClaims(t *testing.T) {
	idx := &PackReferenceIndex{HasCampaignsAny: true}
	dash := &contract.UnifiedDashboardV1{
		AdEfficiency: contract.UnifiedAdEfficiencyV1{
			Narrative: &contract.UnifiedAdEfficiencyNarrativeV1{
				Status:   "available",
				Headline: "Реклама умеренно нагружает кабинет, но окупаемость кампаний остаётся низкой",
				Summary:  "Доля рекламных расходов в общей выручке кабинета составила 16,8%. Доля расходов в выручке, атрибутированной рекламным кампаниям, достигла 91,8%.",
			},
		},
	}
	report := &Report{}
	validateAdvertisingClaimGrounding(report, dash, idx, contract.AnalysisOutcomeMaterialActionsFound)
	if !hasCode(report.Errors, CodeAdvertisingClaimPercentWithoutClaim) {
		t.Fatalf("expected %s for both percentages, got %+v", CodeAdvertisingClaimPercentWithoutClaim, report.Errors)
	}
	count := 0
	for _, e := range report.Errors {
		if e.Code == CodeAdvertisingClaimPercentWithoutClaim {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("expected 2 percent-without-claim errors (one per percentage), got %d: %+v", count, report.Errors)
	}
}

// TestValidateAdvertisingClaimGrounding_TwoPercentagesWithClaimsPass is the
// happy path counterpart: both percentages have a matching metric_claims
// entry, so no percent-without-claim error fires.
func TestValidateAdvertisingClaimGrounding_TwoPercentagesWithClaimsPass(t *testing.T) {
	idx := &PackReferenceIndex{HasCampaignsAny: true}
	dash := &contract.UnifiedDashboardV1{
		AdEfficiency: contract.UnifiedAdEfficiencyV1{
			Narrative: &contract.UnifiedAdEfficiencyNarrativeV1{
				Status:   "available",
				Headline: "Реклама умеренно нагружает кабинет, но окупаемость кампаний остаётся низкой",
				Summary:  "Доля рекламных расходов в общей выручке кабинета составила 16,8%. Доля расходов в выручке, атрибутированной рекламным кампаниям, достигла 91,8%.",
				MetricClaims: []contract.UnifiedMetricClaimV1{
					adClaim(AdMetricAdSpendShareOfTotalStoreRevenuePct, "account:ad_spend_share_of_total_store_revenue_pct:current_7d", 16.8),
					adClaim(AdMetricAttributedCampaignDRRPct, "account:attributed_campaign_drr_pct:current_7d", 91.8),
				},
			},
		},
	}
	report := &Report{}
	validateAdvertisingClaimGrounding(report, dash, idx, contract.AnalysisOutcomeMaterialActionsFound)
	if len(report.Errors) != 0 {
		t.Fatalf("expected no errors, got %+v", report.Errors)
	}
}

// TestValidateAdvertisingClaimGrounding_ValueMismatchBlocksWhenPackIndexed
// covers requirement 5's "mismatch blocks": the claim's own free-text value
// matches its claim, but the claim value diverges from the indexed Pack
// fact, so publication is still blocked.
func TestValidateAdvertisingClaimGrounding_ValueMismatchBlocksWhenPackIndexed(t *testing.T) {
	idx := &PackReferenceIndex{
		HasCampaignsAny: true,
		accountMetrics: map[string]AccountMetricFact{
			"account:ad_spend_share_of_total_store_revenue_pct:current_7d": {
				FactID: "account:ad_spend_share_of_total_store_revenue_pct:current_7d", MetricCode: AdMetricAdSpendShareOfTotalStoreRevenuePct, Value: 16.8, PeriodType: "current_7d",
			},
			"account:attributed_campaign_drr_pct:current_7d": {
				FactID: "account:attributed_campaign_drr_pct:current_7d", MetricCode: AdMetricAttributedCampaignDRRPct, Value: 91.8, PeriodType: "current_7d",
			},
		},
	}
	dash := &contract.UnifiedDashboardV1{
		AdEfficiency: contract.UnifiedAdEfficiencyV1{
			Narrative: &contract.UnifiedAdEfficiencyNarrativeV1{
				Status:   "available",
				Headline: "Реклама умеренно нагружает кабинет, но окупаемость кампаний остаётся низкой",
				Summary:  "Доля рекламных расходов в общей выручке кабинета составила 40,0%. Доля расходов в выручке, атрибутированной рекламным кампаниям, достигла 91,8%.",
				MetricClaims: []contract.UnifiedMetricClaimV1{
					adClaim(AdMetricAdSpendShareOfTotalStoreRevenuePct, "account:ad_spend_share_of_total_store_revenue_pct:current_7d", 40.0),
					adClaim(AdMetricAttributedCampaignDRRPct, "account:attributed_campaign_drr_pct:current_7d", 91.8),
				},
			},
		},
	}
	report := &Report{}
	validateAdvertisingClaimGrounding(report, dash, idx, contract.AnalysisOutcomeMaterialActionsFound)
	if !hasCode(report.Errors, CodeAdvertisingClaimValueMismatch) {
		t.Fatalf("expected %s, got %+v", CodeAdvertisingClaimValueMismatch, report.Errors)
	}
	// Invalid claim values do not ground free-text percents.
	if !hasCode(report.Errors, CodeAdvertisingClaimPercentWithoutClaim) {
		t.Fatalf("expected %s for text percent backed only by invalid claim, got %+v", CodeAdvertisingClaimPercentWithoutClaim, report.Errors)
	}
}

func TestValidateAdvertisingClaimGrounding_UnknownMetricCodeRejected(t *testing.T) {
	idx := &PackReferenceIndex{HasCampaignsAny: true}
	dash := &contract.UnifiedDashboardV1{
		AdEfficiency: contract.UnifiedAdEfficiencyV1{
			Narrative: &contract.UnifiedAdEfficiencyNarrativeV1{
				Status:   "available",
				Headline: "Реклама нагружает кабинет",
				Summary:  "Кампании расходуют бюджет без явных проблем.",
				MetricClaims: []contract.UnifiedMetricClaimV1{
					adClaim("account_drr_pct", "account:account_drr_pct:current_7d", 91.8),
				},
			},
		},
	}
	report := &Report{}
	validateAdvertisingClaimGrounding(report, dash, idx, contract.AnalysisOutcomeMaterialActionsFound)
	if !hasCode(report.Errors, CodeAdvertisingClaimUnknownMetricCode) {
		t.Fatalf("expected %s, got %+v", CodeAdvertisingClaimUnknownMetricCode, report.Errors)
	}
}

func TestValidateAdvertisingClaimGrounding_ClaimWithoutPeriodRejected(t *testing.T) {
	idx := &PackReferenceIndex{HasCampaignsAny: true}
	v := 16.8
	dash := &contract.UnifiedDashboardV1{
		AdEfficiency: contract.UnifiedAdEfficiencyV1{
			Narrative: &contract.UnifiedAdEfficiencyNarrativeV1{
				Status:   "available",
				Headline: "Реклама нагружает кабинет",
				Summary:  "Кампании расходуют бюджет без явных проблем.",
				MetricClaims: []contract.UnifiedMetricClaimV1{
					{
						FactID:     "account:ad_spend_share_of_total_store_revenue_pct:current_7d",
						MetricCode: AdMetricAdSpendShareOfTotalStoreRevenuePct,
						Value:      &v,
						// period_type/period_start/period_end intentionally omitted.
					},
				},
			},
		},
	}
	report := &Report{}
	validateAdvertisingClaimGrounding(report, dash, idx, contract.AnalysisOutcomeMaterialActionsFound)
	if !hasCode(report.Errors, CodeAdvertisingClaimPeriodMissing) {
		t.Fatalf("expected %s for claim without period, got %+v", CodeAdvertisingClaimPeriodMissing, report.Errors)
	}
}

// TestValidateAdvertisingClaimGrounding_DenominatorConfusionDetected covers
// "using total store revenue as attributed revenue or vice versa": the
// claim's metric_code is attributed_ad_revenue but its value actually
// matches the store_total_revenue Pack fact.
func TestValidateAdvertisingClaimGrounding_DenominatorConfusionDetected(t *testing.T) {
	idx := &PackReferenceIndex{
		HasCampaignsAny: true,
		accountMetrics: map[string]AccountMetricFact{
			"account:store_total_revenue:current_7d": {
				FactID: "account:store_total_revenue:current_7d", MetricCode: AdMetricStoreTotalRevenue, Value: 975389.79, PeriodType: "current_7d",
			},
			"account:attributed_ad_revenue:current_7d": {
				FactID: "account:attributed_ad_revenue:current_7d", MetricCode: AdMetricAttributedAdRevenue, Value: 178500, PeriodType: "current_7d",
			},
		},
	}
	dash := &contract.UnifiedDashboardV1{
		AdEfficiency: contract.UnifiedAdEfficiencyV1{
			Narrative: &contract.UnifiedAdEfficiencyNarrativeV1{
				Status:   "available",
				Headline: "Реклама нагружает кабинет",
				Summary:  "Выручка, атрибутированная рекламным кампаниям, составила 975 389,79 ₽.",
				MetricClaims: []contract.UnifiedMetricClaimV1{
					// Mislabels the total store revenue value as attributed_ad_revenue.
					adClaim(AdMetricAttributedAdRevenue, "account:attributed_ad_revenue:current_7d", 975389.79),
				},
			},
		},
	}
	report := &Report{}
	validateAdvertisingClaimGrounding(report, dash, idx, contract.AnalysisOutcomeMaterialActionsFound)
	if !hasCode(report.Errors, CodeAdvertisingRevenueDenominatorConfused) {
		t.Fatalf("expected %s, got %+v", CodeAdvertisingRevenueDenominatorConfused, report.Errors)
	}
}

func TestValidateAdvertisingClaimGrounding_FactUnknownWhenPackIndexed(t *testing.T) {
	idx := &PackReferenceIndex{
		HasCampaignsAny: true,
		accountMetrics: map[string]AccountMetricFact{
			"account:ad_spend:current_7d": {FactID: "account:ad_spend:current_7d", MetricCode: AdMetricAdSpend, Value: 163800, PeriodType: "current_7d"},
		},
	}
	dash := &contract.UnifiedDashboardV1{
		AdEfficiency: contract.UnifiedAdEfficiencyV1{
			Narrative: &contract.UnifiedAdEfficiencyNarrativeV1{
				Status:   "available",
				Headline: "Реклама нагружает кабинет",
				Summary:  "Кампании расходуют бюджет без явных проблем.",
				MetricClaims: []contract.UnifiedMetricClaimV1{
					adClaim(AdMetricAdOrders, "account:ad_orders:current_7d", 105),
				},
			},
		},
	}
	report := &Report{}
	validateAdvertisingClaimGrounding(report, dash, idx, contract.AnalysisOutcomeMaterialActionsFound)
	if !hasCode(report.Errors, CodeAdvertisingClaimFactUnknown) {
		t.Fatalf("expected %s when fact_id/metric is not indexed, got %+v", CodeAdvertisingClaimFactUnknown, report.Errors)
	}
}

func TestValidateAdvertisingClaimGrounding_SkippedWithoutAdvertisingData(t *testing.T) {
	idx := &PackReferenceIndex{HasCampaignsAny: false}
	dash := &contract.UnifiedDashboardV1{
		AdEfficiency: contract.UnifiedAdEfficiencyV1{
			Narrative: &contract.UnifiedAdEfficiencyNarrativeV1{
				Status:  "available",
				Summary: "Расходы на рекламу составили 16,8% выручки.",
			},
		},
	}
	report := &Report{}
	validateAdvertisingClaimGrounding(report, dash, idx, contract.AnalysisOutcomeMaterialActionsFound)
	if len(report.Errors) != 0 {
		t.Fatalf("expected no errors when pack has no advertising data, got %+v", report.Errors)
	}
}
