package outputvalidator

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

func TestValidateAdvertisingNarrative_RequiresSeparateFields(t *testing.T) {
	idx := &PackReferenceIndex{HasCampaignsAny: true}
	dash := &contract.UnifiedDashboardV1{
		AdEfficiency: contract.UnifiedAdEfficiencyV1{
			Narrative: &contract.UnifiedAdEfficiencyNarrativeV1{
				Status:  "available",
				Summary: "ДРР кабинета составляет 19,4%.",
			},
		},
	}
	report := &Report{}
	validateAdvertisingNarrativeStructure(report, dash, idx, contract.AnalysisOutcomeMaterialActionsFound)
	if !hasCode(report.Errors, CodeAdvertisingHeadlineMissing) {
		t.Fatalf("expected %s, got %+v", CodeAdvertisingHeadlineMissing, report.Errors)
	}
}

func TestValidateAdvertisingNarrative_RejectsDecimalTruncation(t *testing.T) {
	idx := &PackReferenceIndex{HasCampaignsAny: true}
	dash := &contract.UnifiedDashboardV1{
		AdEfficiency: contract.UnifiedAdEfficiencyV1{
			Narrative: &contract.UnifiedAdEfficiencyNarrativeV1{
				Status:   "available",
				Headline: "Общий ДРР 19.",
				Summary:  "4%), но две кампании требуют внимания.",
			},
		},
	}
	report := &Report{}
	validateAdvertisingNarrativeStructure(report, dash, idx, contract.AnalysisOutcomeMaterialActionsFound)
	if !hasCode(report.Errors, CodeAdvertisingHeadlineTruncated) {
		t.Fatalf("expected truncated headline, got %+v", report.Errors)
	}
	if !hasCode(report.Errors, CodeAdvertisingExplanationFragmented) {
		t.Fatalf("expected fragmented explanation, got %+v", report.Errors)
	}
}

func TestValidateAdvertisingNarrative_AcceptsSeparateHeadlineAndExplanation(t *testing.T) {
	idx := &PackReferenceIndex{HasCampaignsAny: true}
	dash := &contract.UnifiedDashboardV1{
		AdEfficiency: contract.UnifiedAdEfficiencyV1{
			Narrative: &contract.UnifiedAdEfficiencyNarrativeV1{
				Status:   "available",
				Headline: "Реклама умеренно нагружает кабинет, но окупаемость кампаний остаётся низкой",
				Summary:  "За период 2–8 июля на рекламу потрачено 163 800 ₽. Это составляет 16,8% общей выручки кабинета. Выручка, атрибутированная рекламным кампаниям, составила 178 500 ₽, поэтому доля расходов в рекламной выручке достигла 91,8%. Проблемы сосредоточены в отдельных кампаниях.",
			},
		},
	}
	report := &Report{}
	validateAdvertisingNarrativeStructure(report, dash, idx, contract.AnalysisOutcomeMaterialActionsFound)
	if len(report.Errors) != 0 {
		t.Fatalf("expected no errors, got %+v", report.Errors)
	}
}

func TestValidateAdvertisingNarrative_AcceptsBareDRRWordingWhenStructureOK(t *testing.T) {
	// Linguistic «ДРР кабинета» no longer blocks publication by itself —
	// grounding is claim-value based (see advertising_percent_grounding_test.go).
	idx := &PackReferenceIndex{HasCampaignsAny: true}
	dash := &contract.UnifiedDashboardV1{
		AdEfficiency: contract.UnifiedAdEfficiencyV1{
			Narrative: &contract.UnifiedAdEfficiencyNarrativeV1{
				Status:   "available",
				Headline: "Общий ДРР кабинета остаётся высоким",
				Summary:  "Общий ДРР кабинета составляет 109%. Отдельные кампании требуют внимания по ставкам и окупаемости.",
			},
		},
	}
	report := &Report{}
	validateAdvertisingNarrativeStructure(report, dash, idx, contract.AnalysisOutcomeMaterialActionsFound)
	if hasCode(report.Errors, CodeAdvertisingBareDRRForStoreShare) {
		t.Fatalf("bare DRR wording must not block structure validation, got %+v", report.Errors)
	}
}

func TestValidateAdvertisingNarrative_StructureIgnoresConflictingPercents(t *testing.T) {
	// Percent conflicts are enforced only via claim grounding, not sentence semantics.
	idx := &PackReferenceIndex{HasCampaignsAny: true}
	dash := &contract.UnifiedDashboardV1{
		AdEfficiency: contract.UnifiedAdEfficiencyV1{
			Narrative: &contract.UnifiedAdEfficiencyNarrativeV1{
				Status:   "available",
				Headline: "Рекламная нагрузка на кабинет нестабильна",
				Summary:  "Доля рекламных расходов в общей выручке кабинета указана как 19,4% и одновременно 91,8%. Это противоречие проверяется через claims.",
			},
		},
	}
	report := &Report{}
	validateAdvertisingNarrativeStructure(report, dash, idx, contract.AnalysisOutcomeMaterialActionsFound)
	if hasCode(report.Errors, CodeAdvertisingNarrativeFactMismatch) {
		t.Fatalf("structure path must not semantic-block percents, got %+v", report.Errors)
	}
}

func TestValidateAdvertisingNarrative_RejectsRecommendation(t *testing.T) {
	idx := &PackReferenceIndex{HasCampaignsAny: true}
	dash := &contract.UnifiedDashboardV1{
		AdEfficiency: contract.UnifiedAdEfficiencyV1{
			Narrative: &contract.UnifiedAdEfficiencyNarrativeV1{
				Status:         "available",
				Headline:       "Реклама умеренно нагружает кабинет, но окупаемость кампаний остаётся низкой",
				Summary:        "За период 2–8 июля на рекламу потрачено 163 800 ₽. Это составляет 16,8% общей выручки кабинета.",
				Recommendation: "Проверить ставки проблемных кампаний.",
			},
		},
	}
	report := &Report{}
	validateAdvertisingNarrativeStructure(report, dash, idx, contract.AnalysisOutcomeMaterialActionsFound)
	if !hasCode(report.Errors, "advertising_recommendation_not_allowed") {
		t.Fatalf("expected recommendation forbidden, got %+v", report.Errors)
	}
}

func TestValidateAdvertisingNarrative_RejectsDuplicate(t *testing.T) {
	idx := &PackReferenceIndex{HasCampaignsAny: true}
	same := "Реклама умеренно нагружает кабинет, но окупаемость кампаний остаётся низкой"
	dash := &contract.UnifiedDashboardV1{
		AdEfficiency: contract.UnifiedAdEfficiencyV1{
			Narrative: &contract.UnifiedAdEfficiencyNarrativeV1{
				Status:   "available",
				Headline: same,
				Summary:  same,
			},
		},
	}
	report := &Report{}
	validateAdvertisingNarrativeStructure(report, dash, idx, contract.AnalysisOutcomeMaterialActionsFound)
	if !hasCode(report.Errors, CodeAdvertisingHeadlineExplanationDuplicate) {
		t.Fatalf("expected duplicate code, got %+v", report.Errors)
	}
}

func hasCode(issues []Issue, code string) bool {
	for _, i := range issues {
		if i.Code == code {
			return true
		}
	}
	return false
}
