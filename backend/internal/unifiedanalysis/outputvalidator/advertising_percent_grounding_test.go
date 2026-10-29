package outputvalidator

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/thresholdpolicy"
)

// Claim-based advertising percent grounding (post ce0c31f simplification).
// Free-text Russian wording is ignored; only validated claim values matter.

func groundingPackIndex() *PackReferenceIndex {
	return run193PackIndex()
}

func groundingValidate(t *testing.T, narr *contract.UnifiedAdEfficiencyNarrativeV1) *Report {
	t.Helper()
	dash := &contract.UnifiedDashboardV1{
		AdEfficiency: contract.UnifiedAdEfficiencyV1{Narrative: narr},
	}
	report := &Report{}
	validateAdvertisingNarrativeStructure(report, dash, groundingPackIndex(), contract.AnalysisOutcomeMaterialActionsFound)
	validateAdvertisingClaimGrounding(report, dash, groundingPackIndex(), contract.AnalysisOutcomeMaterialActionsFound)
	return report
}

func mustPass(t *testing.T, report *Report) {
	t.Helper()
	if len(report.Errors) != 0 {
		t.Fatalf("expected PASS, got %+v", report.Errors)
	}
}

func mustFailCode(t *testing.T, report *Report, code string) {
	t.Helper()
	if !hasCode(report.Errors, code) {
		t.Fatalf("expected %s, got %+v", code, report.Errors)
	}
}

func TestPercentMatchesClaim_Rounding(t *testing.T) {
	cases := []struct {
		text, claim float64
		want        bool
	}{
		{26.2, 26.234567, true},
		{26.2, 26.23, true},
		{26.23, 26.234567, true},
		{80, 80, true},
		{80.0, 80, true},
		{0, 0, true},
		{120, 120.0, true},
		{27, 26.23, false},
		{75, 80, false},
		{40, 16.8, false},
	}
	for _, tc := range cases {
		if got := PercentMatchesClaim(tc.text, tc.claim); got != tc.want {
			t.Fatalf("PercentMatchesClaim(%v,%v)=%v want %v", tc.text, tc.claim, got, tc.want)
		}
	}
}

// --- PASS: runs 193–196 shapes ---

func TestAdvertisingPercentGrounding_Pass_DualMetricsOneSentence(t *testing.T) {
	// Run 193
	mustPass(t, groundingValidate(t, &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Рекламная нагрузка на кабинет — 26,2%, а атрибутированный ДРР по кампаниям достиг 53,3%",
		Summary: "За неделю 2–8 июля на рекламу потрачено 1 848 129 ₽, что составляет 26,2% от общей выручки кабинета. " +
			"При этом по атрибутированным продажам ДРР кампаний — 53,3%. Проблемы локальны.",
		MetricClaims: run193Claims(),
	}))
}

func TestAdvertisingPercentGrounding_Pass_ThresholdGT80(t *testing.T) {
	mustPass(t, groundingValidate(t, &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:          "available",
		Headline:        "Часть кампаний дорогая по ДРР",
		Summary:         "Есть дорогие кампании (ДРР > 80%) без достаточной окупаемости. Локальная оптимизация снизит нагрузку.",
		ThresholdClaims: []contract.UnifiedThresholdClaimV1{highDRRThresholdClaim()},
	}))
}

func TestAdvertisingPercentGrounding_Pass_ThresholdAbove80(t *testing.T) {
	mustPass(t, groundingValidate(t, &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:          "available",
		Headline:        "Часть кампаний дорогая по ДРР",
		Summary:         "В 2 кампаниях ДРР выше 80%, а по одной фиксируются расходы без заказов. Нужна локальная оптимизация ставок.",
		ThresholdClaims: []contract.UnifiedThresholdClaimV1{highDRRThresholdClaim()},
	}))
}

func TestAdvertisingPercentGrounding_Pass_ThresholdExceedPorog(t *testing.T) {
	// Run 195
	mustPass(t, groundingValidate(t, &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Доля рекламы стабильна, но часть кампаний дорогая",
		Summary: "Доля рекламы в общей выручке 26,2%. Атрибутированный ДРР 53,3%. " +
			"При этом 3 кампании превысили порог 80%. Требуется перераспределение бюджета.",
		MetricClaims: []contract.UnifiedMetricClaimV1{
			adClaim(AdMetricAdSpendShareOfTotalStoreRevenuePct, "account:ad_spend_share_of_total_store_revenue_pct:current_7d", 26.231698344160172),
			adClaim(AdMetricAttributedCampaignDRRPct, "account:attributed_campaign_drr_pct:current_7d", 53.333971468501254),
		},
		ThresholdClaims: []contract.UnifiedThresholdClaimV1{highDRRThresholdClaim()},
	}))
}

func TestAdvertisingPercentGrounding_Pass_ThresholdPrevyshaet(t *testing.T) {
	// Run 196 headline form
	mustPass(t, groundingValidate(t, &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Доля рекламы в общей выручке кабинета на стабильном уровне, но ДРР по 2 кампаниям превышает 80%",
		Summary: "Доля расходов на рекламу в общей выручке составила 26,2%. " +
			"При этом атрибутированный ДРР всех кампаний достиг 53,3%. В 2 кампаниях ДРР выше 80%, а по одной фиксируются расходы без заказов.",
		MetricClaims: []contract.UnifiedMetricClaimV1{
			adClaim(AdMetricAdSpendShareOfTotalStoreRevenuePct, "account:ad_spend_share_of_total_store_revenue_pct:current_7d", 26.231698344160172),
			adClaim(AdMetricAttributedCampaignDRRPct, "account:attributed_campaign_drr_pct:current_7d", 53.333971468501254),
		},
		ThresholdClaims: []contract.UnifiedThresholdClaimV1{highDRRThresholdClaim()},
	}))
}

func TestAdvertisingPercentGrounding_Pass_ThreePercentsTogether(t *testing.T) {
	mustPass(t, groundingValidate(t, &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Рекламная нагрузка умеренная, но локальный ДРР высокий",
		Summary: "Рекламная нагрузка составляет 26,2% общей выручки, атрибутированный ДРР — 53,3%, " +
			"а три кампании превышают установленный порог 80%. Оптимизация точечная.",
		MetricClaims: []contract.UnifiedMetricClaimV1{
			adClaim(AdMetricAdSpendShareOfTotalStoreRevenuePct, "account:ad_spend_share_of_total_store_revenue_pct:current_7d", 26.231698344160172),
			adClaim(AdMetricAttributedCampaignDRRPct, "account:attributed_campaign_drr_pct:current_7d", 53.333971468501254),
		},
		ThresholdClaims: []contract.UnifiedThresholdClaimV1{highDRRThresholdClaim()},
	}))
}

func TestAdvertisingPercentGrounding_Pass_RepeatedThreshold80(t *testing.T) {
	mustPass(t, groundingValidate(t, &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:          "available",
		Headline:        "Порог высокого ДРР — 80%",
		Summary:         "Порог составляет 80%. Три кампании имеют ДРР выше 80%. Остальные кампании в норме.",
		ThresholdClaims: []contract.UnifiedThresholdClaimV1{highDRRThresholdClaim()},
	}))
}

func TestAdvertisingPercentGrounding_Pass_RawClaimRounding(t *testing.T) {
	mustPass(t, groundingValidate(t, &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Рекламная нагрузка умеренная",
		Summary:  "Доля рекламы в общей выручке составила 26,2%. Атрибутированный ДРР достиг 53,3%.",
		MetricClaims: []contract.UnifiedMetricClaimV1{
			adClaim(AdMetricAdSpendShareOfTotalStoreRevenuePct, "account:ad_spend_share_of_total_store_revenue_pct:current_7d", 26.234567),
			adClaim(AdMetricAttributedCampaignDRRPct, "account:attributed_campaign_drr_pct:current_7d", 53.333971468501254),
		},
	}))
}

func TestAdvertisingPercentGrounding_Pass_HeadlineSummaryRepeat(t *testing.T) {
	mustPass(t, groundingValidate(t, &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Доля рекламы 26,2%, атрибутированный ДРР 53,3%",
		Summary:  "Доля рекламы в общей выручке 26,2%. Атрибутированный ДРР кампаний 53,3%. Проблемы локальны по кампаниям.",
		MetricClaims: []contract.UnifiedMetricClaimV1{
			adClaim(AdMetricAdSpendShareOfTotalStoreRevenuePct, "account:ad_spend_share_of_total_store_revenue_pct:current_7d", 26.231698344160172),
			adClaim(AdMetricAttributedCampaignDRRPct, "account:attributed_campaign_drr_pct:current_7d", 53.333971468501254),
		},
	}))
}

// --- FAIL ---

func TestAdvertisingPercentGrounding_Fail_ThresholdMissing(t *testing.T) {
	mustFailCode(t, groundingValidate(t, &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:       "available",
		Headline:     "Часть кампаний дорогая",
		Summary:      "Есть дорогие кампании (ДРР > 80%) без достаточной окупаемости. Нужна локальная оптимизация.",
		MetricClaims: []contract.UnifiedMetricClaimV1{},
	}), CodeAdvertisingClaimPercentWithoutClaim)
}

func TestAdvertisingPercentGrounding_Fail_Text75Claim80(t *testing.T) {
	mustFailCode(t, groundingValidate(t, &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:          "available",
		Headline:        "Часть кампаний дорогая",
		Summary:         "Есть дорогие кампании (ДРР > 75%) без достаточной окупаемости. Нужна локальная оптимизация.",
		ThresholdClaims: []contract.UnifiedThresholdClaimV1{highDRRThresholdClaim()},
	}), CodeAdvertisingClaimPercentWithoutClaim)
}

func TestAdvertisingPercentGrounding_Fail_UnknownPolicyID(t *testing.T) {
	mustFailCode(t, groundingValidate(t, &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Часть кампаний дорогая",
		Summary:  "Есть дорогие кампании (ДРР > 80%). Нужна локальная оптимизация ставок.",
		ThresholdClaims: []contract.UnifiedThresholdClaimV1{{
			PolicyID: "invented_drr_threshold", MetricCode: thresholdpolicy.MetricCampaignDRRPct,
			Scope: thresholdpolicy.ScopeCampaign, Operator: thresholdpolicy.OpGT, Value: 80,
		}},
	}), CodeAdvertisingThresholdUnknownPolicy)
}

func TestAdvertisingPercentGrounding_Fail_WrongOperator(t *testing.T) {
	mustFailCode(t, groundingValidate(t, &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Часть кампаний дорогая",
		Summary:  "Есть дорогие кампании (ДРР > 80%). Нужна локальная оптимизация ставок.",
		ThresholdClaims: []contract.UnifiedThresholdClaimV1{{
			PolicyID: thresholdpolicy.PolicyHighDRRCampaignThreshold, MetricCode: thresholdpolicy.MetricCampaignDRRPct,
			Scope: thresholdpolicy.ScopeCampaign, Operator: thresholdpolicy.OpGTE, Value: 80,
		}},
	}), CodeAdvertisingThresholdPolicyMismatch)
}

func TestAdvertisingPercentGrounding_Fail_WrongScope(t *testing.T) {
	mustFailCode(t, groundingValidate(t, &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Часть кампаний дорогая",
		Summary:  "Есть дорогие кампании (ДРР > 80%). Нужна локальная оптимизация ставок.",
		ThresholdClaims: []contract.UnifiedThresholdClaimV1{{
			PolicyID: thresholdpolicy.PolicyHighDRRCampaignThreshold, MetricCode: thresholdpolicy.MetricCampaignDRRPct,
			Scope: thresholdpolicy.ScopeAccount, Operator: thresholdpolicy.OpGT, Value: 80,
		}},
	}), CodeAdvertisingThresholdPolicyMismatch)
}

func TestAdvertisingPercentGrounding_Fail_WrongMetricCode(t *testing.T) {
	mustFailCode(t, groundingValidate(t, &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Часть кампаний дорогая",
		Summary:  "Есть дорогие кампании (ДРР > 80%). Нужна локальная оптимизация ставок.",
		ThresholdClaims: []contract.UnifiedThresholdClaimV1{{
			PolicyID: thresholdpolicy.PolicyHighDRRCampaignThreshold, MetricCode: AdMetricAdSpendShareOfTotalStoreRevenuePct,
			Scope: thresholdpolicy.ScopeCampaign, Operator: thresholdpolicy.OpGT, Value: 80,
		}},
	}), CodeAdvertisingThresholdPolicyMismatch)
}

func TestAdvertisingPercentGrounding_Fail_TextStoreShareClaimAttrOnly(t *testing.T) {
	mustFailCode(t, groundingValidate(t, &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Реклама даёт 26,2% эффекта",
		Summary:  "Доля 26,2% указана в тексте без store-share claim. Атрибутированный ДРР достиг 53,3%.",
		MetricClaims: []contract.UnifiedMetricClaimV1{
			adClaim(AdMetricAttributedCampaignDRRPct, "account:attributed_campaign_drr_pct:current_7d", 53.333971468501254),
		},
	}), CodeAdvertisingClaimPercentWithoutClaim)
}

func TestAdvertisingPercentGrounding_Fail_InvalidClaimDoesNotGround(t *testing.T) {
	// Threshold claim value matches text but fails policy validation → cannot ground.
	report := groundingValidate(t, &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Часть кампаний дорогая",
		Summary:  "Есть дорогие кампании (ДРР > 80%). Нужна локальная оптимизация ставок.",
		ThresholdClaims: []contract.UnifiedThresholdClaimV1{{
			PolicyID: thresholdpolicy.PolicyHighDRRCampaignThreshold, MetricCode: thresholdpolicy.MetricCampaignDRRPct,
			Scope: thresholdpolicy.ScopeCampaign, Operator: thresholdpolicy.OpGT, Value: 75, // invalid vs registry 80
		}},
	})
	mustFailCode(t, report, CodeAdvertisingThresholdPolicyMismatch)
	mustFailCode(t, report, CodeAdvertisingClaimPercentWithoutClaim)
}

func TestAdvertisingPercentGrounding_Fail_Invented95(t *testing.T) {
	mustFailCode(t, groundingValidate(t, &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Реклама даёт 95% эффекта",
		Summary:  "Эффективность оценена в 95% без привязки к Pack. Нужен пересмотр подхода.",
		MetricClaims: []contract.UnifiedMetricClaimV1{
			adClaim(AdMetricAdSpendShareOfTotalStoreRevenuePct, "account:ad_spend_share_of_total_store_revenue_pct:current_7d", 26.231698344160172),
		},
	}), CodeAdvertisingClaimPercentWithoutClaim)
}

func TestAdvertisingPercentGrounding_Fail_UnknownFactID(t *testing.T) {
	mustFailCode(t, groundingValidate(t, &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Доля рекламы 26,2%",
		Summary:  "Доля рекламы в общей выручке составила 26,2%. Атрибутированный ДРР достиг 53,3%.",
		MetricClaims: []contract.UnifiedMetricClaimV1{
			adClaim(AdMetricAdSpendShareOfTotalStoreRevenuePct, "account:invented:current_7d", 26.231698344160172),
			adClaim(AdMetricAttributedCampaignDRRPct, "account:attributed_campaign_drr_pct:current_7d", 53.333971468501254),
		},
	}), CodeAdvertisingClaimFactUnknown)
}

func TestAdvertisingPercentGrounding_Fail_WrongPeriod(t *testing.T) {
	v := 26.231698344160172
	mustFailCode(t, groundingValidate(t, &contract.UnifiedAdEfficiencyNarrativeV1{
		Status:   "available",
		Headline: "Доля рекламы 26,2%",
		Summary:  "Доля рекламы в общей выручке составила 26,2%. Атрибутированный ДРР достиг 53,3%.",
		MetricClaims: []contract.UnifiedMetricClaimV1{{
			FactID:     "account:ad_spend_share_of_total_store_revenue_pct:current_7d",
			MetricCode: AdMetricAdSpendShareOfTotalStoreRevenuePct, Value: &v,
			PeriodType: "not_a_period", PeriodStart: "2026-07-02", PeriodEnd: "2026-07-08",
		}, adClaim(AdMetricAttributedCampaignDRRPct, "account:attributed_campaign_drr_pct:current_7d", 53.333971468501254)},
	}), CodeAdvertisingClaimPeriodMissing)
}

func TestAdvertisingPercentGrounding_Fail_StoreDenomConfused(t *testing.T) {
	// store_total_revenue claim carrying attributed revenue value
	idx := groundingPackIndex()
	attr := idx.accountMetrics["account:attributed_ad_revenue:current_7d"]
	dash := &contract.UnifiedDashboardV1{
		AdEfficiency: contract.UnifiedAdEfficiencyV1{
			Narrative: &contract.UnifiedAdEfficiencyNarrativeV1{
				Status:   "available",
				Headline: "Выручка кабинета завышена в claim",
				Summary:  "Структурный claim путает знаменатели выручки. Дополнительный контекст подтверждает ошибку.",
				MetricClaims: []contract.UnifiedMetricClaimV1{
					adClaim(AdMetricStoreTotalRevenue, "account:store_total_revenue:current_7d", attr.Value),
				},
			},
		},
	}
	report := &Report{}
	validateAdvertisingClaimGrounding(report, dash, idx, contract.AnalysisOutcomeMaterialActionsFound)
	mustFailCode(t, report, CodeAdvertisingRevenueDenominatorConfused)
}

func TestAdvertisingPercentGrounding_Fail_AttrDenomConfused(t *testing.T) {
	idx := groundingPackIndex()
	store := idx.accountMetrics["account:store_total_revenue:current_7d"]
	dash := &contract.UnifiedDashboardV1{
		AdEfficiency: contract.UnifiedAdEfficiencyV1{
			Narrative: &contract.UnifiedAdEfficiencyNarrativeV1{
				Status:   "available",
				Headline: "Атрибутированная выручка подменена",
				Summary:  "Структурный claim путает знаменатели выручки. Дополнительный контекст подтверждает ошибку.",
				MetricClaims: []contract.UnifiedMetricClaimV1{
					adClaim(AdMetricAttributedAdRevenue, "account:attributed_ad_revenue:current_7d", store.Value),
				},
			},
		},
	}
	report := &Report{}
	validateAdvertisingClaimGrounding(report, dash, idx, contract.AnalysisOutcomeMaterialActionsFound)
	mustFailCode(t, report, CodeAdvertisingRevenueDenominatorConfused)
}
