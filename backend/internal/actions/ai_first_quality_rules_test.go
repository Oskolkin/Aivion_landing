package actions

import (
	"encoding/json"
	"testing"
)

func TestValidateAIFirstActionStructuralQuality_DetailedActionAccepted(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	input := pilotInput(c)
	draft := ValidDetailedPilotDraftFromCandidateForTests(c)
	issues := validateAIFirstActionStructuralQuality(input, draft, 0)
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %+v", issues)
	}
}

func TestValidateAIFirstActionStructuralQuality_MissingObjectActionsRejected(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	input := pilotInput(c)
	draft := ValidMinimalPilotDraftFromCandidateForTests(c)
	issues := validateAIFirstActionStructuralQuality(input, draft, 0)
	if !qualityRulesHasIssueCode(issues, "missing_object_actions") {
		t.Fatalf("issues=%+v", issues)
	}
}

func TestValidateAIFirstActionStructuralQuality_InsufficientActionPlanRejected(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	input := pilotInput(c)
	draft := ValidDetailedPilotDraftFromCandidateForTests(c)
	draft.ActionPlan = []AIFirstActionStep{{Instruction: "Проверить остатки SKU 120 шт"}}
	issues := validateAIFirstActionStructuralQuality(input, draft, 0)
	if !qualityRulesHasIssueCode(issues, "insufficient_action_plan") {
		t.Fatalf("issues=%+v", issues)
	}
}

func TestValidateAIFirstActionStructuralQuality_GenericStepRejected(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	input := pilotInput(c)
	draft := ValidDetailedPilotDraftFromCandidateForTests(c)
	draft.ActionPlan[1].Instruction = "Принять меры"
	issues := validateAIFirstActionStructuralQuality(input, draft, 0)
	if !qualityRulesHasIssueCode(issues, "generic_action_step") {
		t.Fatalf("issues=%+v", issues)
	}
}

func TestParseFlexibleObjectActions_NormalizesMetricsObject(t *testing.T) {
	raw := json.RawMessage(`[
		{
			"entity_type": "sku",
			"entity_id": "sku_1",
			"recommended_action": "Проверить карточку",
			"reason": "Нет заказов 7д",
			"metrics": {"Остаток": "120", "Заказы 7д": "0"}
		}
	]`)
	objects := parseFlexibleObjectActions(raw)
	if len(objects) != 1 {
		t.Fatalf("objects=%d", len(objects))
	}
	if len(objects[0].Metrics) < 2 {
		t.Fatalf("metrics=%+v", objects[0].Metrics)
	}
}

func TestParseFlexibleActionPlan_StringNotSufficientForValidator(t *testing.T) {
	plan := parseFlexibleActionPlan(json.RawMessage(`"Провести анализ остатков"`))
	if len(plan) != 1 {
		t.Fatalf("plan=%+v", plan)
	}
	c := testStockCandidate("stock:candidate:01")
	input := pilotInput(c)
	draft := ValidMinimalPilotDraftFromCandidateForTests(c)
	draft.ActionPlan = plan
	issues := validateAIFirstActionStructuralQuality(input, draft, 0)
	if !qualityRulesHasIssueCode(issues, "insufficient_action_plan") {
		t.Fatalf("issues=%+v", issues)
	}
}

func TestMapAIFirstActionToItem_PreservesObjectActions(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	input, _ := BuildActionsAIFirstInputFromCandidates(
		testV2CatalogPack(), "hash", testGlobalContext(), "gc", []AIFirstActionCandidate{c}, nil, nil, DefaultGenerateActionsOptions(),
	)
	action := ValidDetailedPilotDraftFromCandidateForTests(c)
	item := MapAIFirstActionToItem("2026-06-24", input, action)
	if item.OperationalDetail == nil {
		t.Fatal("operational_detail is nil")
	}
	if len(item.OperationalDetail.ObjectActions) != 1 {
		t.Fatalf("object_actions=%d", len(item.OperationalDetail.ObjectActions))
	}
	if len(item.OperationalDetail.ObjectActions[0].Metrics) < 2 {
		t.Fatalf("metrics=%+v", item.OperationalDetail.ObjectActions[0].Metrics)
	}
	if len(item.OperationalDetail.ActionPlan.Steps) < 3 {
		t.Fatalf("steps=%d", len(item.OperationalDetail.ActionPlan.Steps))
	}
}

func qualityRulesHasIssueCode(issues []ValidationIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
