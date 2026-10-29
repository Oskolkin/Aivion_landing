package outputvalidator

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

func pulsePoint(id, sigType, section, title, expl string, factIDs []string) contract.UnifiedSellerPulsePointV1 {
	return contract.UnifiedSellerPulsePointV1{
		UnifiedDiagnosticSignalV2: contract.UnifiedDiagnosticSignalV2{
			SignalID:       id,
			SignalType:     sigType,
			PrimarySection: section,
			Title:          title,
			Explanation:    expl,
			FactIDs:        factIDs,
		},
		PeriodType:  "current_7d",
		PeriodStart: "2026-07-02",
		PeriodEnd:   "2026-07-08",
	}
}

func TestSellerPulseClaimGroundingMatchesPack(t *testing.T) {
	idx := &PackReferenceIndex{accountMetrics: map[string]AccountMetricFact{
		"account:revenue_7d:current_7d": {FactID: "account:revenue_7d:current_7d", MetricCode: "revenue_7d", Value: 975389.79, PeriodType: "current_7d"},
	}}
	val := 975389.79
	out := &contract.UnifiedAnalysisOutputV1{
		Dashboard: contract.UnifiedDashboardV1{
			SellerPulse: contract.UnifiedSellerPulseV1{
				Narrative: &contract.UnifiedSellerPulseNarrativeV1{
					Status:   "ready",
					Headline: "Недельная выручка снизилась",
					Summary:  "За период 2–8 июля выручка снизилась относительно предыдущих 7 дней. Основное снижение связано с заказами. Поддержкой служит стабильный CTR.",
					MetricClaims: []contract.UnifiedMetricClaimV1{{
						FactID: "account:revenue_7d:current_7d", MetricCode: "revenue_7d", Value: &val,
						PeriodType: "current_7d", PeriodStart: "2026-07-02", PeriodEnd: "2026-07-08",
					}},
					KeyPoints: []contract.UnifiedSellerPulsePointV1{
						pulsePoint("k1", "rev", "seller_pulse_key_point", "Выручка за неделю", "Снижение подтверждено Pack.", []string{"account:revenue_7d:current_7d"}),
						pulsePoint("k2", "ord", "seller_pulse_key_point", "Заказы за неделю", "Динамика заказов отрицательная.", []string{"account:orders_7d:current_7d"}),
						pulsePoint("k3", "sup", "seller_pulse_key_point", "Есть точки опоры", "Часть SKU стабильна.", []string{"account:revenue_7d:current_7d"}),
					},
					ImportantFacts: []contract.UnifiedSellerPulsePointV1{
						pulsePoint("i1", "stock", "seller_pulse_important", "Сейчас важно: остатки", "Дефицит по лидерам требует внимания.", []string{"account:revenue_7d:current_7d"}),
						pulsePoint("i2", "ads", "seller_pulse_important", "Сейчас важно: реклама", "Локальные кампании с высоким ДРР.", []string{"account:revenue_7d:current_7d"}),
						pulsePoint("i3", "stable", "seller_pulse_important", "Сейчас важно: стабильность", "Ряд метрик держится.", []string{"account:revenue_7d:current_7d"}),
					},
				},
			},
		},
	}
	report := Report{}
	validateSellerPulseClaimGrounding(&report, &out.Dashboard, idx, contract.AnalysisOutcomeNoMaterialActions)
	if len(report.Errors) > 0 {
		t.Fatalf("unexpected errors: %+v", report.Errors)
	}

	bad := 437100.0
	out.Dashboard.SellerPulse.Narrative.MetricClaims[0].Value = &bad
	report = Report{}
	validateSellerPulseClaimGrounding(&report, &out.Dashboard, idx, contract.AnalysisOutcomeNoMaterialActions)
	if !hasCodeLocal(report, CodeSellerPulseAIFactMismatch) {
		t.Fatalf("want fact mismatch, got %+v", report.Errors)
	}
}

func TestSellerPulseCrossPeriodClaimBlocked(t *testing.T) {
	report := Report{}
	val := 100.0
	validateMetricClaimStructure(&report, "$.claim", contract.UnifiedMetricClaimV1{
		FactID: "account:revenue_7d:current_7d", MetricCode: "revenue_7d", Value: &val,
		PeriodType: "current_day", PeriodStart: "2026-07-08", PeriodEnd: "2026-07-08",
	})
	if !hasCodeLocal(report, CodeSellerPulseCrossPeriodClaimInvalid) {
		t.Fatalf("want cross-period error, got %+v", report.Errors)
	}
}

func TestSellerPulseImportantFactsRequired(t *testing.T) {
	n := &contract.UnifiedSellerPulseNarrativeV1{
		Status:   "ready",
		Headline: "Недельный спад выручки при локальных рисках",
		Summary:  "За период 2–8 июля выручка снизилась относительно предыдущих 7 дней. Основные причины — остатки и реклама. Поддержкой служит стабильный CTR.",
		KeyPoints: []contract.UnifiedSellerPulsePointV1{
			pulsePoint("k1", "a", "seller_pulse_key_point", "A", "Текст пункта один с числом 123.", []string{"account:revenue_7d:current_7d"}),
			pulsePoint("k2", "b", "seller_pulse_key_point", "B", "Текст пункта два с числом 456.", []string{"account:revenue_7d:current_7d"}),
			pulsePoint("k3", "c", "seller_pulse_key_point", "C", "Текст пункта три с числом 789.", []string{"account:revenue_7d:current_7d"}),
		},
		MetricClaims: []contract.UnifiedMetricClaimV1{},
	}
	report := Report{}
	validateSellerPulseDepth(&report, n, contract.AnalysisOutcomeMaterialActionsFound)
	if !hasCodeLocal(report, CodeSellerPulseImportantFactsMissing) {
		t.Fatalf("want important facts missing, got %+v", report.Errors)
	}
}

func hasCodeLocal(report Report, code string) bool {
	for _, e := range report.Errors {
		if e.Code == code {
			return true
		}
	}
	return false
}
