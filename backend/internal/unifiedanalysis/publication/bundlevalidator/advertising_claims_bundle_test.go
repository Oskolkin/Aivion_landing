package bundlevalidator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

func run205AdWidget(t *testing.T, withClaims bool) dashboardhome.AdEfficiencyWidget {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "acceptance_run205_output.json"))
	if err != nil {
		t.Fatalf("read run205 fixture: %v", err)
	}
	var out contract.UnifiedAnalysisOutputV1
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal run205 fixture: %v", err)
	}
	n := out.Dashboard.AdEfficiency.Narrative
	if n == nil {
		t.Fatal("run205 fixture missing ad_efficiency.narrative")
	}
	attr := 53.33
	storeSpend := 1848129.35
	storeRev := 7045404.86
	attrRev := 3465201.07
	widget := dashboardhome.AdEfficiencyWidget{
		Summary: &dashboardhome.AdEfficiencySummary{
			CurrentDRRPct:            &attr,
			AverageDRR7dPct:          &attr,
			TotalSpendToday:          374012.0, // day-level ≈26.6% store share (legacy trap)
			TotalStoreRevenueToday:   1406053.0,
			TotalSpend7d:             storeSpend,
			TotalStoreRevenue7d:      storeRev,
			TotalAttributedRevenue7d: attrRev,
		},
		Narrative: &dashboardhome.AdEfficiencyNarrative{
			Status:   n.Status,
			Headline: n.Headline,
			Summary:  n.Summary,
		},
	}
	if !withClaims {
		return widget
	}
	widget.Narrative.MetricClaims = projectClaimsFromContract(n.MetricClaims)
	widget.Narrative.ThresholdClaims = projectThresholdsFromContract(n.ThresholdClaims)
	return widget
}

func projectClaimsFromContract(in []contract.UnifiedMetricClaimV1) []dashboardhome.AdEfficiencyMetricClaim {
	out := make([]dashboardhome.AdEfficiencyMetricClaim, 0, len(in))
	for _, c := range in {
		out = append(out, dashboardhome.AdEfficiencyMetricClaim{
			FactID: c.FactID, MetricCode: c.MetricCode, Value: c.Value,
			PeriodType: c.PeriodType, PeriodStart: c.PeriodStart, PeriodEnd: c.PeriodEnd,
		})
	}
	return out
}

func projectThresholdsFromContract(in []contract.UnifiedThresholdClaimV1) []dashboardhome.AdEfficiencyThresholdClaim {
	out := make([]dashboardhome.AdEfficiencyThresholdClaim, 0, len(in))
	for _, c := range in {
		out = append(out, dashboardhome.AdEfficiencyThresholdClaim{
			PolicyID: c.PolicyID, MetricCode: c.MetricCode, Scope: c.Scope,
			Operator: c.Operator, Value: c.Value,
		})
	}
	return out
}

// Offline replay run 205: legacy heuristics without claims reproduce the
// live publication_failed code; claim-aware path must PASS.
func TestOfflineReplayRun205_AdvertisingBundleClaimAware(t *testing.T) {
	legacy := run205AdWidget(t, false)
	legacyIssues := checkAdvertisingNarrativeLegacyHeuristics(legacy, legacy.Narrative.Headline+" "+legacy.Narrative.Summary)
	if !containsIssueCode(legacyIssues, "advertising_total_revenue_share_mismatch") {
		t.Fatalf("before-fix legacy path must reproduce advertising_total_revenue_share_mismatch, got %+v", legacyIssues)
	}

	fixed := run205AdWidget(t, true)
	issues := checkAdvertisingNarrativeArithmetic(fixed)
	if len(issues) != 0 {
		t.Fatalf("claim-aware run205 advertising bundle must PASS, got %+v", issues)
	}

	// Provenance mapping required by acceptance report.
	var storeCode, attrCode, storeFact, attrFact string
	var storeScope, attrScope string
	for _, c := range fixed.Narrative.MetricClaims {
		switch c.MetricCode {
		case adMetricStoreSpendSharePct:
			storeCode, storeFact, storeScope = c.MetricCode, c.FactID, "account"
		case adMetricAttributedCampaignDRR:
			attrCode, attrFact, attrScope = c.MetricCode, c.FactID, "account"
		}
	}
	if storeCode == "" || attrCode == "" {
		t.Fatal("expected both store-share and attributed-DRR claims")
	}
	t.Logf("53.3%% → metric_code=%s scope=%s fact_id=%s", attrCode, attrScope, attrFact)
	t.Logf("26.6%%/26.2%% → metric_code=%s scope=%s fact_id=%s", storeCode, storeScope, storeFact)
}

func TestAdvertisingClaimAware_PASS_StoreShareOnly(t *testing.T) {
	v := 26.6
	ad := dashboardhome.AdEfficiencyWidget{
		Summary: &dashboardhome.AdEfficiencySummary{
			TotalSpendToday: 266, TotalStoreRevenueToday: 1000,
		},
		Narrative: &dashboardhome.AdEfficiencyNarrative{
			Headline: "Рекламная нагрузка 26,6%",
			Summary:  "Доля расходов 26,6% от выручки кабинета.",
			MetricClaims: []dashboardhome.AdEfficiencyMetricClaim{{
				FactID:     "account:ad_spend_share_of_total_store_revenue_pct:current_day",
				MetricCode: adMetricStoreSpendSharePct, Value: floatPtr(26.6), PeriodType: "current_day",
			}},
		},
	}
	_ = v
	if issues := checkAdvertisingNarrativeArithmetic(ad); len(issues) != 0 {
		t.Fatalf("PASS1 store share only: %+v", issues)
	}
}

func TestAdvertisingClaimAware_PASS_AttributedDRROnly(t *testing.T) {
	attr := 53.3
	ad := dashboardhome.AdEfficiencyWidget{
		Summary: &dashboardhome.AdEfficiencySummary{
			CurrentDRRPct: &attr, AverageDRR7dPct: &attr,
			TotalSpend7d: 533, TotalAttributedRevenue7d: 1000,
		},
		Narrative: &dashboardhome.AdEfficiencyNarrative{
			Headline: "Атрибутированный ДРР 53,3%",
			Summary:  "ДРР по атрибутированной выручке 53,3%.",
			MetricClaims: []dashboardhome.AdEfficiencyMetricClaim{{
				FactID:     "account:attributed_campaign_drr_pct:current_7d",
				MetricCode: adMetricAttributedCampaignDRR, Value: floatPtr(53.3), PeriodType: "current_7d",
			}},
		},
	}
	if issues := checkAdvertisingNarrativeArithmetic(ad); len(issues) != 0 {
		t.Fatalf("PASS2 attributed DRR only: %+v", issues)
	}
}

func TestAdvertisingClaimAware_PASS_StoreAttributedAndThreshold(t *testing.T) {
	attr := 53.33
	ad := dashboardhome.AdEfficiencyWidget{
		Summary: &dashboardhome.AdEfficiencySummary{
			CurrentDRRPct: &attr, AverageDRR7dPct: &attr,
			TotalSpend7d: 1848129.35, TotalStoreRevenue7d: 7045404.86,
			TotalAttributedRevenue7d: 3465201.07,
		},
		Narrative: &dashboardhome.AdEfficiencyNarrative{
			Headline: "Нагрузка 26,2%, атрибутированный ДРР 53,3%, порог 80%",
			Summary:  "Store 26,2%. Attr 53,3%. Campaigns above 80%.",
			MetricClaims: []dashboardhome.AdEfficiencyMetricClaim{
				{FactID: "account:ad_spend_share_of_total_store_revenue_pct:current_7d", MetricCode: adMetricStoreSpendSharePct, Value: floatPtr(26.23), PeriodType: "current_7d"},
				{FactID: "account:attributed_campaign_drr_pct:current_7d", MetricCode: adMetricAttributedCampaignDRR, Value: floatPtr(53.33), PeriodType: "current_7d"},
			},
			ThresholdClaims: []dashboardhome.AdEfficiencyThresholdClaim{{
				PolicyID: "high_drr_campaign_threshold", MetricCode: adMetricCampaignDRRPct,
				Scope: "campaign", Operator: "gt", Value: 80,
			}},
		},
	}
	if issues := checkAdvertisingNarrativeArithmetic(ad); len(issues) != 0 {
		t.Fatalf("PASS3 triple claims: %+v", issues)
	}
}

func TestAdvertisingClaimAware_PASS_MultipleCampaignDRR(t *testing.T) {
	ad := dashboardhome.AdEfficiencyWidget{
		Summary: &dashboardhome.AdEfficiencySummary{TotalSpendToday: 1, TotalStoreRevenueToday: 100},
		Narrative: &dashboardhome.AdEfficiencyNarrative{
			Headline: "Кампании 91,8% и 75,0%",
			Summary:  "Кампания A 91,8%. Кампания B 75,0%.",
			MetricClaims: []dashboardhome.AdEfficiencyMetricClaim{
				{FactID: "campaign:111:drr_pct:current_7d", MetricCode: adMetricCampaignDRRPct, Value: floatPtr(91.8), PeriodType: "current_7d"},
				{FactID: "campaign:222:drr_pct:current_7d", MetricCode: adMetricCampaignDRRPct, Value: floatPtr(75.0), PeriodType: "current_7d"},
			},
		},
	}
	if issues := checkAdvertisingNarrativeArithmetic(ad); len(issues) != 0 {
		t.Fatalf("PASS4 multi campaign DRR: %+v", issues)
	}
}

func TestAdvertisingClaimAware_PASS_CoincidentValuesDifferentCodes(t *testing.T) {
	// Same numeric value on two codes: identity comes from metric_code, not the number.
	same := 40.0
	ad := dashboardhome.AdEfficiencyWidget{
		Summary: &dashboardhome.AdEfficiencySummary{
			CurrentDRRPct: &same, AverageDRR7dPct: &same,
			TotalSpendToday: 40, TotalStoreRevenueToday: 100,
			TotalSpend7d: 40, TotalAttributedRevenue7d: 100, TotalStoreRevenue7d: 100,
		},
		Narrative: &dashboardhome.AdEfficiencyNarrative{
			Headline: "Оба показателя 40%",
			Summary:  "Store share 40%. Attributed DRR also 40%.",
			MetricClaims: []dashboardhome.AdEfficiencyMetricClaim{
				{FactID: "account:ad_spend_share_of_total_store_revenue_pct:current_day", MetricCode: adMetricStoreSpendSharePct, Value: floatPtr(40), PeriodType: "current_day"},
				{FactID: "account:attributed_campaign_drr_pct:current_7d", MetricCode: adMetricAttributedCampaignDRR, Value: floatPtr(40), PeriodType: "current_7d"},
			},
		},
	}
	if issues := checkAdvertisingNarrativeArithmetic(ad); len(issues) != 0 {
		t.Fatalf("PASS5 coincident values: %+v", issues)
	}
}

func TestAdvertisingClaimAware_PASS_Idempotent(t *testing.T) {
	ad := run205AdWidget(t, true)
	a := checkAdvertisingNarrativeArithmetic(ad)
	b := checkAdvertisingNarrativeArithmetic(ad)
	if len(a) != 0 || len(b) != 0 {
		t.Fatalf("PASS6 idempotent must PASS, a=%+v b=%+v", a, b)
	}
	if len(a) != len(b) {
		t.Fatalf("PASS6 issue count drift")
	}
}

func TestAdvertisingClaimAware_FAIL_CampaignDRRComparedAsStoreShare(t *testing.T) {
	// Mis-labeled claim: campaign DRR value filed under store-share metric_code.
	ad := dashboardhome.AdEfficiencyWidget{
		Summary: &dashboardhome.AdEfficiencySummary{
			TotalSpendToday: 266, TotalStoreRevenueToday: 1000, // store share 26.6
		},
		Narrative: &dashboardhome.AdEfficiencyNarrative{
			Headline: "53,3%",
			Summary:  "Нагрузка 53,3%.",
			MetricClaims: []dashboardhome.AdEfficiencyMetricClaim{{
				FactID:     "account:ad_spend_share_of_total_store_revenue_pct:current_day",
				MetricCode: adMetricStoreSpendSharePct, Value: floatPtr(53.3), PeriodType: "current_day",
			}},
		},
	}
	issues := checkAdvertisingNarrativeArithmetic(ad)
	if !containsIssueCode(issues, "advertising_total_revenue_share_mismatch") {
		t.Fatalf("FAIL1 expected store-share mismatch, got %+v", issues)
	}
}

func TestAdvertisingClaimAware_FAIL_StoreShareUsesCampaignFact(t *testing.T) {
	ad := dashboardhome.AdEfficiencyWidget{
		Summary: &dashboardhome.AdEfficiencySummary{TotalSpendToday: 266, TotalStoreRevenueToday: 1000},
		Narrative: &dashboardhome.AdEfficiencyNarrative{
			Headline: "26,6%",
			Summary:  "Доля 26,6%.",
			MetricClaims: []dashboardhome.AdEfficiencyMetricClaim{{
				FactID:     "campaign:999:drr_pct:current_7d",
				MetricCode: adMetricStoreSpendSharePct, Value: floatPtr(26.6), PeriodType: "current_day",
			}},
		},
	}
	issues := checkAdvertisingNarrativeArithmetic(ad)
	if !containsIssueCode(issues, "advertising_store_share_campaign_fact") {
		t.Fatalf("FAIL2 expected campaign-fact rejection, got %+v", issues)
	}
}

func TestAdvertisingClaimAware_FAIL_PercentWithoutClaim(t *testing.T) {
	ad := dashboardhome.AdEfficiencyWidget{
		Summary: &dashboardhome.AdEfficiencySummary{TotalSpendToday: 1, TotalStoreRevenueToday: 100},
		Narrative: &dashboardhome.AdEfficiencyNarrative{
			Headline: "53,3%",
			Summary:  "Без claim.",
			MetricClaims: []dashboardhome.AdEfficiencyMetricClaim{{
				FactID:     "account:ad_spend_share_of_total_store_revenue_pct:current_day",
				MetricCode: adMetricStoreSpendSharePct, Value: floatPtr(1.0), PeriodType: "current_day",
			}},
		},
	}
	issues := checkAdvertisingNarrativeArithmetic(ad)
	if !containsIssueCode(issues, "advertising_claim_percent_without_claim") {
		t.Fatalf("FAIL3 expected percent-without-claim, got %+v", issues)
	}
}

func TestAdvertisingClaimAware_FAIL_CampaignDRRMissingFactID(t *testing.T) {
	ad := dashboardhome.AdEfficiencyWidget{
		Summary: &dashboardhome.AdEfficiencySummary{TotalSpendToday: 1, TotalStoreRevenueToday: 100},
		Narrative: &dashboardhome.AdEfficiencyNarrative{
			Headline: "91,8%",
			Summary:  "Кампания 91,8%.",
			MetricClaims: []dashboardhome.AdEfficiencyMetricClaim{{
				MetricCode: adMetricCampaignDRRPct, Value: floatPtr(91.8), PeriodType: "current_7d",
			}},
		},
	}
	issues := checkAdvertisingNarrativeArithmetic(ad)
	if !containsIssueCode(issues, "advertising_campaign_drr_fact_missing") {
		t.Fatalf("FAIL6 expected missing fact_id, got %+v", issues)
	}
}

func TestAdvertisingClaimAware_FAIL_CampaignDRRAccountFact(t *testing.T) {
	ad := dashboardhome.AdEfficiencyWidget{
		Summary: &dashboardhome.AdEfficiencySummary{TotalSpendToday: 1, TotalStoreRevenueToday: 100},
		Narrative: &dashboardhome.AdEfficiencyNarrative{
			Headline: "91,8%",
			Summary:  "Кампания 91,8%.",
			MetricClaims: []dashboardhome.AdEfficiencyMetricClaim{{
				FactID:     "account:attributed_campaign_drr_pct:current_7d",
				MetricCode: adMetricCampaignDRRPct, Value: floatPtr(91.8), PeriodType: "current_7d",
			}},
		},
	}
	issues := checkAdvertisingNarrativeArithmetic(ad)
	if !containsIssueCode(issues, "advertising_campaign_drr_fact_scope") {
		t.Fatalf("FAIL7 expected campaign fact scope, got %+v", issues)
	}
}

func TestAdvertisingClaimAware_FAIL_ThresholdAsFactualMetric(t *testing.T) {
	// 80% appears in text but only as a mis-filed store-share metric claim.
	ad := dashboardhome.AdEfficiencyWidget{
		Summary: &dashboardhome.AdEfficiencySummary{TotalSpendToday: 266, TotalStoreRevenueToday: 1000},
		Narrative: &dashboardhome.AdEfficiencyNarrative{
			Headline: "Порог 80%",
			Summary:  "Кампании выше 80%.",
			MetricClaims: []dashboardhome.AdEfficiencyMetricClaim{{
				FactID:     "account:ad_spend_share_of_total_store_revenue_pct:current_day",
				MetricCode: adMetricStoreSpendSharePct, Value: floatPtr(80), PeriodType: "current_day",
			}},
		},
	}
	issues := checkAdvertisingNarrativeArithmetic(ad)
	if !containsIssueCode(issues, "advertising_total_revenue_share_mismatch") {
		t.Fatalf("FAIL9 threshold-as-metric must mismatch store share, got %+v", issues)
	}
}

func TestAdvertisingClaimAware_FAIL_SharedDenominatorMislabel(t *testing.T) {
	// Attributed DRR claim value that only matches store-share arithmetic.
	ad := dashboardhome.AdEfficiencyWidget{
		Summary: &dashboardhome.AdEfficiencySummary{
			CurrentDRRPct: floatPtr(53.3), AverageDRR7dPct: floatPtr(53.3),
			TotalSpendToday: 266, TotalStoreRevenueToday: 1000,
			TotalSpend7d: 266, TotalStoreRevenue7d: 1000, TotalAttributedRevenue7d: 500,
		},
		Narrative: &dashboardhome.AdEfficiencyNarrative{
			Headline: "26,6%",
			Summary:  "Attr labeled 26,6%.",
			MetricClaims: []dashboardhome.AdEfficiencyMetricClaim{{
				FactID:     "account:attributed_campaign_drr_pct:current_7d",
				MetricCode: adMetricAttributedCampaignDRR, Value: floatPtr(26.6), PeriodType: "current_7d",
			}},
		},
	}
	issues := checkAdvertisingNarrativeArithmetic(ad)
	if !containsIssueCode(issues, "advertising_attributed_revenue_share_mismatch") {
		t.Fatalf("FAIL12 attributed vs store denominator, got %+v", issues)
	}
}
