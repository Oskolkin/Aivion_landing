package actions

import (
	"testing"
)

func TestBuildOperationalDetail_AdHighDRR(t *testing.T) {
	cand := ActionCandidate{
		CandidateID: "ad_high_drr_2026-06-25_over_threshold",
		Type:        ActionTypeAdHighDRR,
		Summary:     "5 кампаний с ДРР выше 100%",
		Aggregates: map[string]any{
			"campaign_count":    5,
			"spend_7d_total":    1074747.0,
			"revenue_7d_total":  418094.0,
			"avg_drr":           364.7,
			"drr_threshold_pct": 100.0,
		},
		Facts: []ActionCandidateFact{
			{FactID: "ad_high_drr_2026-06-25_over_threshold.campaign_count", Label: "Кампаний", Value: "5"},
			{FactID: "ad_high_drr_2026-06-25_over_threshold.spend_7d_total", Label: "Расход 7д", Value: "1074747"},
			{FactID: "ad_high_drr_2026-06-25_over_threshold.drr", Label: "ДРР", Value: "364.7", Unit: "%"},
		},
		TopExamples: []ActionCandidateExample{{
			EntityType: "campaign",
			EntityID:   "882000071674",
			Name:       "Перегрев по ad_waste SKU",
			Metrics:    map[string]any{"spend_7d": 500000.0, "drr_pct": 400.0},
		}},
		EntityGroupRef: &ActionEntityGroupRef{EntityType: "campaign", Count: 5},
		AllowedCTAs:    []string{ActionCTAOpenCampaignList, ActionCTAExportHighDRRCampaigns, ActionCTAAskAI, ActionCTAMarkChecked},
		Limitations:    []string{adHighDRRLimitation},
	}

	detail := BuildOperationalDetailFromCandidate(cand)
	if len(detail.ActionPlan.Steps) == 0 {
		t.Fatal("expected steps")
	}
	if detail.ActionPlan.Steps[0].DecisionMode != DecisionModeDirect {
		t.Fatalf("step mode=%q want direct", detail.ActionPlan.Steps[0].DecisionMode)
	}
	if !containsString(detail.Why.Text, "1 074 747") && !containsString(detail.Why.Text, "1074747") {
		t.Fatalf("why missing spend: %q", detail.Why.Text)
	}
	issues := ValidateOperationalDetail(detail, ActionTypeAdHighDRR, []ActionCandidate{cand})
	for _, issue := range issues {
		if issue.Severity == IssueSeverityFatal || issue.Severity == IssueSeverityRepairable {
			t.Fatalf("unexpected issue: %+v", issue)
		}
	}
}

func TestValidateOperationalDetail_RejectsGenericStep(t *testing.T) {
	cand := sampleStockDomainContext().Candidates[0]
	detail := BuildOperationalDetailFromCandidate(cand)
	detail.ActionPlan.Steps = []OperationalStep{{
		StepID:         "step_1",
		Title:          "Проверка",
		Instruction:    "Откройте список и проверьте настройки",
		WhyThisStep:    "Нужно проверить",
		ExpectedResult: "Поймёте ситуацию",
		DecisionMode:   DecisionModeDirect,
	}}
	issues := ValidateOperationalDetail(detail, ActionTypeStockReplenishment, []ActionCandidate{cand})
	if len(issues) == 0 {
		t.Fatal("expected generic step rejection")
	}
}

func TestValidateOperationalDetail_AcceptsConditionalDiagnostic(t *testing.T) {
	cand := sampleStockDomainContext().Candidates[0]
	detail := BuildOperationalDetailFromCandidate(cand)
	detail.ActionPlan.Steps = []OperationalStep{{
		StepID:         "step_1",
		Title:          "Если нет поставки в пути",
		Instruction:    "Если поставок в пути нет — сформируйте поставку до целевого покрытия 14 дней.",
		WhyThisStep:    "Без поставки покрытие продолжит снижаться.",
		ExpectedResult: "Довести покрытие до целевого уровня.",
		DecisionMode:   DecisionModeConditional,
	}}
	issues := ValidateOperationalDetail(detail, ActionTypeStockReplenishment, []ActionCandidate{cand})
	for _, issue := range issues {
		if issue.Code == "generic_operational_step" || issue.Code == "vague_diagnostic_step" {
			t.Fatalf("unexpected issue: %+v", issue)
		}
	}
}

func TestValidateOperationalDetail_RejectsDirectPriceWithoutMargin(t *testing.T) {
	cand := sampleStockDomainContext().Candidates[0]
	detail := BuildOperationalDetailFromCandidate(cand)
	detail.ActionPlan.Steps = []OperationalStep{{
		StepID:         "step_1",
		Title:          "Снизить цену",
		Instruction:    "Снизьте цену на SKU, чтобы увеличить продажи.",
		WhyThisStep:    "Нет продаж",
		ExpectedResult: "Рост заказов",
		DecisionMode:   DecisionModeDirect,
		ActionKind:     "decrease_price",
	}}
	issues := ValidateOperationalDetail(detail, ActionTypeStockReplenishment, []ActionCandidate{cand})
	found := false
	for _, issue := range issues {
		if issue.Code == "unsupported_direct_price" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected unsupported_direct_price, issues=%+v", issues)
	}
}

func TestFallbackProducesOperationalDetail(t *testing.T) {
	domainCtx := sampleStockDomainContext()
	fallback := BuildFallbackActions(domainCtx)
	if len(fallback) != 1 {
		t.Fatalf("fallback count=%d", len(fallback))
	}
	if fallback[0].OperationalDetail == nil {
		t.Fatal("expected operational_detail in fallback")
	}
	if len(fallback[0].OperationalDetail.ActionPlan.Steps) == 0 {
		t.Fatal("expected steps in fallback operational_detail")
	}
}

func TestFallbackAdSpendNoOrders_DirectPauseAction(t *testing.T) {
	cand := ActionCandidate{
		CandidateID: "ad_spend_no_orders_2026-06-25_zero_orders",
		Type:        ActionTypeAdSpendNoOrders,
		Summary:     "1 кампания",
		Aggregates: map[string]any{
			"campaign_count": 1,
			"spend_7d_total": 21568.0,
		},
		Facts: []ActionCandidateFact{
			{FactID: "ad_spend_no_orders_2026-06-25_zero_orders.spend_7d_total", Label: "Расход 7д", Value: "21568"},
			{FactID: "ad_spend_no_orders_2026-06-25_zero_orders.zero_orders", Label: "Заказы 7д", Value: "0"},
		},
		TopExamples: []ActionCandidateExample{{
			EntityType: "campaign",
			EntityID:   "123",
			Name:       "Холодный трафик",
		}},
		AllowedCTAs: []string{ActionCTAAskAI, ActionCTAMarkChecked},
	}
	action, ok := buildFallbackActionForCandidate(cand)
	if !ok {
		t.Fatal("expected fallback action")
	}
	if action.OperationalDetail == nil {
		t.Fatal("missing operational_detail")
	}
	if action.OperationalDetail.ActionPlan.Steps[0].DecisionMode != DecisionModeDirect {
		t.Fatalf("mode=%q", action.OperationalDetail.ActionPlan.Steps[0].DecisionMode)
	}
	if !containsString(action.Title, "паузу") {
		t.Fatalf("title=%q", action.Title)
	}
}

func TestObjectActions_TargetOnlyExamples(t *testing.T) {
	cand := ActionCandidate{
		Type: ActionTypeAdHighDRR,
		TopExamples: []ActionCandidateExample{{
			EntityType: "campaign",
			EntityID:   "allowed",
			Name:       "OK",
		}},
	}
	detail := BuildOperationalDetailFromCandidate(cand)
	detail.ObjectActions = append(detail.ObjectActions, OperationalObjectAction{
		EntityType: "campaign",
		EntityID:   "unknown",
		Name:       "Bad",
	})
	issues := ValidateOperationalDetail(detail, ActionTypeAdHighDRR, []ActionCandidate{cand})
	found := false
	for _, issue := range issues {
		if issue.Code == "operational_unknown_entity" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected unknown entity issue")
	}
}
