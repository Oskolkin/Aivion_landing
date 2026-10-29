package outputvalidator

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/thresholdpolicy"
)

// Acceptance run 194 (SHA f6aac05) blocked on advertising_claim_percent_without_claim
// for the policy threshold «ДРР > 80%». These fixtures lock metric_claims vs
// threshold_claims separation (schema v10 / prompt v1_26).

func run194PackIndex() *PackReferenceIndex {
	return run193PackIndex()
}

func run194MetricClaims() []contract.UnifiedMetricClaimV1 {
	return run193Claims()
}

func highDRRThresholdClaim() contract.UnifiedThresholdClaimV1 {
	return contract.UnifiedThresholdClaimV1{
		PolicyID:   thresholdpolicy.PolicyHighDRRCampaignThreshold,
		MetricCode: thresholdpolicy.MetricCampaignDRRPct,
		Scope:      thresholdpolicy.ScopeCampaign,
		Operator:   thresholdpolicy.OpGT,
		Value:      80,
	}
}

func stockCoverageThresholdClaim() contract.UnifiedThresholdClaimV1 {
	return contract.UnifiedThresholdClaimV1{
		PolicyID:   thresholdpolicy.PolicyLowStockCoverageDays,
		MetricCode: thresholdpolicy.MetricStockCoverageDays7d,
		Scope:      thresholdpolicy.ScopeSKU,
		Operator:   thresholdpolicy.OpLT,
		Value:      7,
	}
}

func validateRun194Narrative(t *testing.T, narr *contract.UnifiedAdEfficiencyNarrativeV1) *Report {
	t.Helper()
	idx := run194PackIndex()
	dash := &contract.UnifiedDashboardV1{
		AdEfficiency: contract.UnifiedAdEfficiencyV1{Narrative: narr},
	}
	report := &Report{}
	validateAdvertisingNarrativeStructure(report, dash, idx, contract.AnalysisOutcomeMaterialActionsFound)
	validateAdvertisingClaimGrounding(report, dash, idx, contract.AnalysisOutcomeMaterialActionsFound)
	return report
}

func TestAcceptanceRun194_Threshold80WithClaimPass(t *testing.T) {
	narr := &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:       "available",
		Headline:     "Часть кампаний дорогая по порогу высокого ДРР",
		Summary:      "Одна из ключевых проблем — наличие дорогих кампаний (ДРР > 80%) без достаточной окупаемости. Локальная оптимизация ставок и отключение убыточных связок снизит рекламную нагрузку.",
		MetricClaims: []contract.UnifiedMetricClaimV1{},
		ThresholdClaims: []contract.UnifiedThresholdClaimV1{
			highDRRThresholdClaim(),
		},
	}
	report := validateRun194Narrative(t, narr)
	if len(report.Errors) != 0 {
		t.Fatalf("PASS: ДРР > 80%% + threshold claim, got %+v", report.Errors)
	}
}

func TestAcceptanceRun194_FactDRRAndThresholdSameSentencePass(t *testing.T) {
	narr := &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Атрибутированный ДРР умеренный, но локальные кампании дорогие",
		Summary:  "Атрибутированный ДРР кампаний — 53,3%, при этом есть дорогие кампании с ДРР > 80%. Проблемы сосредоточены в отдельных кампаниях, а не в общей нагрузке на кабинет.",
		MetricClaims: []contract.UnifiedMetricClaimV1{
			adClaim(AdMetricAttributedCampaignDRRPct, "account:attributed_campaign_drr_pct:current_7d", 53.333971468501254),
		},
		ThresholdClaims: []contract.UnifiedThresholdClaimV1{
			highDRRThresholdClaim(),
		},
	}
	report := validateRun194Narrative(t, narr)
	if len(report.Errors) != 0 {
		t.Fatalf("PASS: fact 53,3%% + threshold 80%%, got %+v", report.Errors)
	}
}

func TestAcceptanceRun194_StoreShareAttrDRRAndThresholdPass(t *testing.T) {
	// Exact run-194 prose shape with structured threshold_claims added.
	narr := &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Рекламная нагрузка на кабинет превышает комфортный уровень, часть кампаний убыточны",
		Summary: "В течение недели затраты на рекламу составили 1,85 млн ₽, что составляет 26,2% общей выручки кабинета. " +
			"При этом по атрибутированным кампаниям ДРР достиг 53,3%. " +
			"Одна из ключевых проблем — наличие как минимум 2 дорогих (ДРР > 80%) и одной кампании с расходами без заказов.",
		MetricClaims: run194MetricClaims(),
		ThresholdClaims: []contract.UnifiedThresholdClaimV1{
			highDRRThresholdClaim(),
		},
	}
	report := validateRun194Narrative(t, narr)
	for _, code := range []string{
		CodeAdvertisingClaimPercentWithoutClaim,
		CodeAdvertisingNarrativeFactMismatch,
		CodeAdvertisingBareDRRForStoreShare,
	} {
		if hasCode(report.Errors, code) {
			t.Fatalf("run194 full narrative must PASS, unexpected %s: %+v", code, report.Errors)
		}
	}
	if len(report.Errors) != 0 {
		t.Fatalf("run194 full narrative must PASS, got %+v", report.Errors)
	}
}

func TestAcceptanceRun194_StockCoverageThresholdProsePass(t *testing.T) {
	narr := &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:       "available",
		Headline:     "Реклама стабильна, но остатки критичны",
		Summary:      "Покрытие меньше 7 дней по ключевым SKU ограничивает эффект рекламы. Без пополнения остатков прирост заказов от ставок будет нестабильным.",
		MetricClaims: []contract.UnifiedMetricClaimV1{},
		ThresholdClaims: []contract.UnifiedThresholdClaimV1{
			stockCoverageThresholdClaim(),
		},
	}
	report := validateRun194Narrative(t, narr)
	if len(report.Errors) != 0 {
		t.Fatalf("PASS: stock coverage < 7 days + threshold claim, got %+v", report.Errors)
	}
}

func TestAcceptanceRun194_Threshold80WithoutStructuredClaimFail(t *testing.T) {
	narr := &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:       "available",
		Headline:     "Часть кампаний дорогая",
		Summary:      "Есть дорогие кампании (ДРР > 80%) без достаточной окупаемости. Нужна локальная оптимизация ставок.",
		MetricClaims: run194MetricClaims(),
	}
	report := validateRun194Narrative(t, narr)
	if !hasCode(report.Errors, CodeAdvertisingClaimPercentWithoutClaim) {
		t.Fatalf("FAIL: threshold without claim expected %s, got %+v", CodeAdvertisingClaimPercentWithoutClaim, report.Errors)
	}
}

func TestAcceptanceRun194_Threshold75AgainstPolicy80Fail(t *testing.T) {
	narr := &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:       "available",
		Headline:     "Часть кампаний дорогая",
		Summary:      "Есть дорогие кампании (ДРР > 75%) без достаточной окупаемости.",
		MetricClaims: []contract.UnifiedMetricClaimV1{},
		ThresholdClaims: []contract.UnifiedThresholdClaimV1{
			{
				PolicyID:   thresholdpolicy.PolicyHighDRRCampaignThreshold,
				MetricCode: thresholdpolicy.MetricCampaignDRRPct,
				Scope:      thresholdpolicy.ScopeCampaign,
				Operator:   thresholdpolicy.OpGT,
				Value:      75, // invented — registry is 80
			},
		},
	}
	report := validateRun194Narrative(t, narr)
	if !hasCode(report.Errors, CodeAdvertisingThresholdPolicyMismatch) {
		t.Fatalf("FAIL: invented 75%% claim expected %s, got %+v", CodeAdvertisingThresholdPolicyMismatch, report.Errors)
	}
	if !hasCode(report.Errors, CodeAdvertisingClaimPercentWithoutClaim) {
		t.Fatalf("FAIL: free-text 75%% cannot match registry, expected %s, got %+v", CodeAdvertisingClaimPercentWithoutClaim, report.Errors)
	}
}

func TestAcceptanceRun194_UnknownPolicyIDFail(t *testing.T) {
	narr := &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Часть кампаний дорогая",
		Summary:  "Есть дорогие кампании (ДРР > 80%).",
		ThresholdClaims: []contract.UnifiedThresholdClaimV1{
			{
				PolicyID:   "invented_drr_threshold",
				MetricCode: thresholdpolicy.MetricCampaignDRRPct,
				Scope:      thresholdpolicy.ScopeCampaign,
				Operator:   thresholdpolicy.OpGT,
				Value:      80,
			},
		},
	}
	report := validateRun194Narrative(t, narr)
	if !hasCode(report.Errors, CodeAdvertisingThresholdUnknownPolicy) {
		t.Fatalf("FAIL: unknown policy expected %s, got %+v", CodeAdvertisingThresholdUnknownPolicy, report.Errors)
	}
}

func TestAcceptanceRun194_WrongScopeFail(t *testing.T) {
	narr := &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Часть кампаний дорогая",
		Summary:  "Есть дорогие кампании (ДРР > 80%).",
		ThresholdClaims: []contract.UnifiedThresholdClaimV1{
			{
				PolicyID:   thresholdpolicy.PolicyHighDRRCampaignThreshold,
				MetricCode: thresholdpolicy.MetricCampaignDRRPct,
				Scope:      thresholdpolicy.ScopeAccount, // wrong — registry is campaign
				Operator:   thresholdpolicy.OpGT,
				Value:      80,
			},
		},
	}
	report := validateRun194Narrative(t, narr)
	if !hasCode(report.Errors, CodeAdvertisingThresholdPolicyMismatch) {
		t.Fatalf("FAIL: wrong scope expected %s, got %+v", CodeAdvertisingThresholdPolicyMismatch, report.Errors)
	}
}

func TestAcceptanceRun194_WrongOperatorFail(t *testing.T) {
	narr := &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Часть кампаний дорогая",
		Summary:  "Есть дорогие кампании (ДРР > 80%).",
		ThresholdClaims: []contract.UnifiedThresholdClaimV1{
			{
				PolicyID:   thresholdpolicy.PolicyHighDRRCampaignThreshold,
				MetricCode: thresholdpolicy.MetricCampaignDRRPct,
				Scope:      thresholdpolicy.ScopeCampaign,
				Operator:   thresholdpolicy.OpGTE, // wrong — registry is gt
				Value:      80,
			},
		},
	}
	report := validateRun194Narrative(t, narr)
	if !hasCode(report.Errors, CodeAdvertisingThresholdPolicyMismatch) {
		t.Fatalf("FAIL: wrong operator expected %s, got %+v", CodeAdvertisingThresholdPolicyMismatch, report.Errors)
	}
}

func TestAcceptanceRun194_BarePercentWithoutClaimFail(t *testing.T) {
	narr := &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:          "available",
		Headline:        "Реклама даёт 40% эффекта",
		Summary:         "Эффективность рекламы оценена в 40% без привязки к Pack-метрике.",
		MetricClaims:    []contract.UnifiedMetricClaimV1{},
		ThresholdClaims: []contract.UnifiedThresholdClaimV1{},
	}
	report := validateRun194Narrative(t, narr)
	if !hasCode(report.Errors, CodeAdvertisingClaimPercentWithoutClaim) {
		t.Fatalf("FAIL: bare percent expected %s, got %+v", CodeAdvertisingClaimPercentWithoutClaim, report.Errors)
	}
}

func TestAcceptanceRun194_BareDRRForStoreSharePassesWithClaim(t *testing.T) {
	// Linguistic «ДРР кабинета» is no longer a separate blocker when the percent
	// is grounded by a validated store-share claim.
	narr := &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Общий ДРР кабинета завышен",
		Summary:  "Общий ДРР кабинета составляет 26,2% от общей выручки. Проблемы локальны по кампаниям.",
		MetricClaims: []contract.UnifiedMetricClaimV1{
			adClaim(AdMetricAdSpendShareOfTotalStoreRevenuePct, "account:ad_spend_share_of_total_store_revenue_pct:current_7d", 26.231698344160172),
		},
	}
	report := validateRun194Narrative(t, narr)
	if len(report.Errors) != 0 {
		t.Fatalf("wording + valid claim must PASS, got %+v", report.Errors)
	}
}

func TestAcceptanceRun194_FactValueGroundsComparativeProse(t *testing.T) {
	// «выше 53,3%» with a matching attributed metric claim is grounded by value;
	// Russian comparative wording is not interpreted as a separate threshold.
	narr := &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "ДРР выглядит как порог",
		Summary:  "Атрибутированный ДРР кампаний выше 53,3%. Проблемы сосредоточены в отдельных кампаниях.",
		MetricClaims: []contract.UnifiedMetricClaimV1{
			adClaim(AdMetricAttributedCampaignDRRPct, "account:attributed_campaign_drr_pct:current_7d", 53.333971468501254),
		},
		ThresholdClaims: []contract.UnifiedThresholdClaimV1{
			highDRRThresholdClaim(),
		},
	}
	report := validateRun194Narrative(t, narr)
	if len(report.Errors) != 0 {
		t.Fatalf("fact value must ground prose regardless of wording, got %+v", report.Errors)
	}
}

func TestAcceptanceRun194_MissingPolicyIDFail(t *testing.T) {
	narr := &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Часть кампаний дорогая",
		Summary:  "Есть дорогие кампании (ДРР > 80%).",
		ThresholdClaims: []contract.UnifiedThresholdClaimV1{
			{
				PolicyID:   "",
				MetricCode: thresholdpolicy.MetricCampaignDRRPct,
				Scope:      thresholdpolicy.ScopeCampaign,
				Operator:   thresholdpolicy.OpGT,
				Value:      80,
			},
		},
	}
	report := validateRun194Narrative(t, narr)
	if !hasCode(report.Errors, CodeAdvertisingThresholdMissingPolicyID) {
		t.Fatalf("FAIL: missing policy_id expected %s, got %+v", CodeAdvertisingThresholdMissingPolicyID, report.Errors)
	}
}

func TestAcceptanceRun194_WrongMetricCodeFail(t *testing.T) {
	narr := &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Часть кампаний дорогая",
		Summary:  "Есть дорогие кампании (ДРР > 80%).",
		ThresholdClaims: []contract.UnifiedThresholdClaimV1{
			{
				PolicyID:   thresholdpolicy.PolicyHighDRRCampaignThreshold,
				MetricCode: AdMetricAdSpendShareOfTotalStoreRevenuePct, // wrong
				Scope:      thresholdpolicy.ScopeCampaign,
				Operator:   thresholdpolicy.OpGT,
				Value:      80,
			},
		},
	}
	report := validateRun194Narrative(t, narr)
	if !hasCode(report.Errors, CodeAdvertisingThresholdPolicyMismatch) {
		t.Fatalf("FAIL: wrong metric_code expected %s, got %+v", CodeAdvertisingThresholdPolicyMismatch, report.Errors)
	}
}
