package outputvalidator

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// Acceptance run 193 (SHA 9aad781) produced a semantically correct dual-metric
// advertising narrative that was blocked by the old "two percents in a
// store-share sentence" heuristic. These fixtures lock the claim-based
// assignment rules.

func run193PackIndex() *PackReferenceIndex {
	return &PackReferenceIndex{
		HasCampaignsAny: true,
		accountMetrics: map[string]AccountMetricFact{
			"account:ad_spend_share_of_total_store_revenue_pct:current_7d": {
				FactID:     "account:ad_spend_share_of_total_store_revenue_pct:current_7d",
				MetricCode: AdMetricAdSpendShareOfTotalStoreRevenuePct, Value: 26.231698344160172, PeriodType: "current_7d",
			},
			"account:attributed_campaign_drr_pct:current_7d": {
				FactID:     "account:attributed_campaign_drr_pct:current_7d",
				MetricCode: AdMetricAttributedCampaignDRRPct, Value: 53.333971468501254, PeriodType: "current_7d",
			},
			"account:ad_spend:current_7d": {
				FactID: "account:ad_spend:current_7d", MetricCode: AdMetricAdSpend, Value: 1848129.35, PeriodType: "current_7d",
			},
			"account:store_total_revenue:current_7d": {
				FactID: "account:store_total_revenue:current_7d", MetricCode: AdMetricStoreTotalRevenue, Value: 7045404.86, PeriodType: "current_7d",
			},
			"account:attributed_ad_revenue:current_7d": {
				FactID: "account:attributed_ad_revenue:current_7d", MetricCode: AdMetricAttributedAdRevenue, Value: 3465201.07, PeriodType: "current_7d",
			},
			"account:ad_orders:current_7d": {
				FactID: "account:ad_orders:current_7d", MetricCode: AdMetricAdOrders, Value: 2289, PeriodType: "current_7d",
			},
		},
	}
}

func run193Claims() []contract.UnifiedMetricClaimV1 {
	return []contract.UnifiedMetricClaimV1{
		adClaim(AdMetricAdSpendShareOfTotalStoreRevenuePct, "account:ad_spend_share_of_total_store_revenue_pct:current_7d", 26.231698344160172),
		adClaim(AdMetricAttributedCampaignDRRPct, "account:attributed_campaign_drr_pct:current_7d", 53.333971468501254),
		adClaim(AdMetricAdSpend, "account:ad_spend:current_7d", 1848129.35),
		adClaim(AdMetricStoreTotalRevenue, "account:store_total_revenue:current_7d", 7045404.86),
		adClaim(AdMetricAttributedAdRevenue, "account:attributed_ad_revenue:current_7d", 3465201.07),
		adClaim(AdMetricAdOrders, "account:ad_orders:current_7d", 2289),
	}
}

func validateRun193Narrative(t *testing.T, narr *contract.UnifiedAdEfficiencyNarrativeV1) *Report {
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

func TestAcceptanceRun193_DualMetricsSameSentencePass(t *testing.T) {
	// Exact shape from run 193: both shares in one headline sentence + claimed.
	narr := &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Рекламная нагрузка на кабинет — 26,2%, а атрибутированный ДРР по кампаниям достиг 53,3%",
		Summary: "За неделю 2–8 июля на рекламу потрачено 1 848 129 ₽, что составляет 26,2% от общей выручки кабинета (7 045 405 ₽). " +
			"При этом по атрибутированным продажам ДРР кампаний — 53,3% (выручка 3 465 201 ₽). " +
			"6 кампаний выделяются высоким ДРР, одна — отсутствием заказов несмотря на затраты. " +
			"В целом рекламная нагрузка выше комфортной, но основные проблемы — на уровне отдельных кампаний.",
		MetricClaims: run193Claims(),
	}
	report := validateRun193Narrative(t, narr)
	for _, code := range []string{
		CodeAdvertisingNarrativeFactMismatch,
		CodeAdvertisingPercentClaimAmbiguous,
		CodeAdvertisingClaimPercentWithoutClaim,
		CodeAdvertisingBareDRRForStoreShare,
		CodeAdvertisingRevenueDenominatorMixed,
	} {
		if hasCode(report.Errors, code) {
			t.Fatalf("run193 dual-metric sentence must PASS, unexpected %s: %+v", code, report.Errors)
		}
	}
	if len(report.Errors) != 0 {
		t.Fatalf("run193 dual-metric sentence must PASS, got %+v", report.Errors)
	}
}

func TestAcceptanceRun193_DualMetricsSeparateSentencesPass(t *testing.T) {
	narr := &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Рекламная нагрузка умеренная, но атрибутированный ДРР кампаний высокий",
		Summary: "Доля рекламных расходов в общей выручке кабинета составила 26,2%. " +
			"Атрибутированный ДРР кампаний достиг 53,3%. " +
			"Проблемы сосредоточены в отдельных кампаниях, а не в общей нагрузке на кабинет.",
		MetricClaims: run193Claims(),
	}
	report := validateRun193Narrative(t, narr)
	if len(report.Errors) != 0 {
		t.Fatalf("separate-sentence dual metrics must PASS, got %+v", report.Errors)
	}
}

func TestAcceptanceRun193_TwoStoreSharePercentsFail(t *testing.T) {
	narr := &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Рекламная нагрузка на кабинет противоречива",
		Summary: "Доля рекламных расходов в общей выручке кабинета указана как 26,2% и одновременно 40,0%. " +
			"Атрибутированный ДРР кампаний достиг 53,3%. Дополнительный контекст по кампаниям подтверждает локальные проблемы.",
		MetricClaims: []contract.UnifiedMetricClaimV1{
			adClaim(AdMetricAdSpendShareOfTotalStoreRevenuePct, "account:ad_spend_share_of_total_store_revenue_pct:current_7d", 26.231698344160172),
			adClaim(AdMetricAttributedCampaignDRRPct, "account:attributed_campaign_drr_pct:current_7d", 53.333971468501254),
		},
	}
	report := validateRun193Narrative(t, narr)
	if !hasCode(report.Errors, CodeAdvertisingNarrativeFactMismatch) &&
		!hasCode(report.Errors, CodeAdvertisingTotalRevenueShareMismatch) &&
		!hasCode(report.Errors, CodeAdvertisingClaimPercentWithoutClaim) {
		t.Fatalf("two store-share percents must FAIL, got %+v", report.Errors)
	}
}

func TestAcceptanceRun193_TwoAttributedDRRPercentsFail(t *testing.T) {
	narr := &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Атрибутированный ДРР кампаний нестабилен",
		Summary: "Атрибутированный ДРР кампаний указан как 53,3% и одновременно 80,0%. " +
			"Доля рекламных расходов в общей выручке кабинета составила 26,2%. Локальные кампании требуют внимания.",
		MetricClaims: []contract.UnifiedMetricClaimV1{
			adClaim(AdMetricAdSpendShareOfTotalStoreRevenuePct, "account:ad_spend_share_of_total_store_revenue_pct:current_7d", 26.231698344160172),
			adClaim(AdMetricAttributedCampaignDRRPct, "account:attributed_campaign_drr_pct:current_7d", 53.333971468501254),
		},
	}
	report := validateRun193Narrative(t, narr)
	if !hasCode(report.Errors, CodeAdvertisingNarrativeFactMismatch) &&
		!hasCode(report.Errors, CodeAdvertisingAttributedRevenueShareMismatch) &&
		!hasCode(report.Errors, CodeAdvertisingClaimPercentWithoutClaim) {
		t.Fatalf("two attributed-DRR percents must FAIL, got %+v", report.Errors)
	}
}

func TestAcceptanceRun193_PercentWithoutClaimFail(t *testing.T) {
	narr := &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Рекламная нагрузка на кабинет умеренная",
		Summary: "Доля рекламных расходов в общей выручке кабинета составила 26,2%. " +
			"Атрибутированный ДРР кампаний достиг 53,3%. Проблемы локальны.",
		MetricClaims: []contract.UnifiedMetricClaimV1{
			adClaim(AdMetricAdSpendShareOfTotalStoreRevenuePct, "account:ad_spend_share_of_total_store_revenue_pct:current_7d", 26.231698344160172),
			// attributed claim intentionally missing
		},
	}
	report := validateRun193Narrative(t, narr)
	if !hasCode(report.Errors, CodeAdvertisingClaimPercentWithoutClaim) &&
		!hasCode(report.Errors, CodeAdvertisingNarrativeFactMismatch) {
		t.Fatalf("percent without claim must FAIL, got %+v", report.Errors)
	}
}

func TestAcceptanceRun193_SwappedClaimValuesFail(t *testing.T) {
	narr := &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Рекламная нагрузка на кабинет — 26,2%, а атрибутированный ДРР по кампаниям достиг 53,3%",
		Summary: "Доля рекламных расходов в общей выручке кабинета составила 26,2%. " +
			"Атрибутированный ДРР кампаний достиг 53,3%. Проблемы локальны по кампаниям.",
		MetricClaims: []contract.UnifiedMetricClaimV1{
			// Values swapped relative to the prose.
			adClaim(AdMetricAdSpendShareOfTotalStoreRevenuePct, "account:ad_spend_share_of_total_store_revenue_pct:current_7d", 53.333971468501254),
			adClaim(AdMetricAttributedCampaignDRRPct, "account:attributed_campaign_drr_pct:current_7d", 26.231698344160172),
		},
	}
	report := validateRun193Narrative(t, narr)
	if !hasCode(report.Errors, CodeAdvertisingNarrativeFactMismatch) &&
		!hasCode(report.Errors, CodeAdvertisingClaimValueMismatch) &&
		!hasCode(report.Errors, CodeAdvertisingTotalRevenueShareMismatch) &&
		!hasCode(report.Errors, CodeAdvertisingClaimPercentWithoutClaim) &&
		!hasCode(report.Errors, CodeAdvertisingRevenueDenominatorConfused) {
		t.Fatalf("swapped claims must FAIL, got %+v", report.Errors)
	}
}

func TestAcceptanceRun193_StoreShareLabeledDRRPassesWithClaims(t *testing.T) {
	// Wording «ДРР кабинета» is not a blocker when claim values ground the percents.
	narr := &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Общий ДРР кабинета остаётся высоким",
		Summary: "Общий ДРР кабинета составляет 26,2%. Атрибутированный ДРР кампаний достиг 53,3%. " +
			"Проблемы сосредоточены в отдельных кампаниях.",
		MetricClaims: run193Claims(),
	}
	report := validateRun193Narrative(t, narr)
	if len(report.Errors) != 0 {
		t.Fatalf("store-share wording with valid claims must PASS, got %+v", report.Errors)
	}
}

func TestAcceptanceRun193_CrossLabeledValuesPassWhenClaimsMatch(t *testing.T) {
	// Wording no longer assigns metrics; values 53.3 and 26.2 both have claims.
	narr := &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Рекламная нагрузка на кабинет требует внимания",
		Summary: "Доля рекламных расходов в общей выручке кабинета составила 53,3%. " +
			"Атрибутированный ДРР кампаний достиг 26,2%. Проблемы локальны.",
		MetricClaims: run193Claims(),
	}
	report := validateRun193Narrative(t, narr)
	if len(report.Errors) != 0 {
		t.Fatalf("value-matched claims must PASS regardless of wording, got %+v", report.Errors)
	}
}

func TestAcceptanceRun193_UnlabeledPercentsPassWhenClaimsMatch(t *testing.T) {
	narr := &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Рекламные показатели кабинета требуют внимания",
		Summary: "Ключевые показатели составили 26,2% и 53,3% за неделю. " +
			"Отдельные кампании выделяются аномалиями. Дополнительный обзор подтверждает локальный характер проблем.",
		MetricClaims: run193Claims(),
	}
	report := validateRun193Narrative(t, narr)
	if len(report.Errors) != 0 {
		t.Fatalf("unlabeled percents with matching claim values must PASS, got %+v", report.Errors)
	}
}

func TestAcceptanceRun193_ValueDivergesFromPackFail(t *testing.T) {
	narr := &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Рекламная нагрузка на кабинет умеренная",
		Summary: "Доля рекламных расходов в общей выручке кабинета составила 40,0%. " +
			"Атрибутированный ДРР кампаний достиг 53,3%. Проблемы локальны.",
		MetricClaims: []contract.UnifiedMetricClaimV1{
			adClaim(AdMetricAdSpendShareOfTotalStoreRevenuePct, "account:ad_spend_share_of_total_store_revenue_pct:current_7d", 40.0),
			adClaim(AdMetricAttributedCampaignDRRPct, "account:attributed_campaign_drr_pct:current_7d", 53.333971468501254),
		},
	}
	report := validateRun193Narrative(t, narr)
	if !hasCode(report.Errors, CodeAdvertisingClaimValueMismatch) &&
		!hasCode(report.Errors, CodeAdvertisingTotalRevenueShareMismatch) {
		t.Fatalf("value diverging from pack must FAIL, got %+v", report.Errors)
	}
}
