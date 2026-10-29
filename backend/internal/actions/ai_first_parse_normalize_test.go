package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func minimalPilotActionJSONFields(c AIFirstActionCandidate, extra string) string {
	return fmt.Sprintf(`{
		"type": %q,
		"category": %q,
		"priority": %q,
		"title": %q,
		"reason": %q,
		"next_step": %q,
		"completion_criteria": "Критерий выполнен",
		"evidence_fact_ids": ["%s"],
		"affected_entities": [{"entity_type":"sku","entity_id":"sku_1"}],
		"source_candidate_ids": ["%s"]
		%s
	}`, c.Type, c.Category, ActionPriorityCritical, "Заголовок "+c.CandidateID, c.Problem, c.RecommendedAction, c.EvidenceFactIDs[0], c.CandidateID, extra)
}

func detailedPilotActionJSONFields(c AIFirstActionCandidate, extra string) string {
	draft := ValidDetailedPilotDraftFromCandidateForTests(c)
	evidenceJSON, _ := json.Marshal(draft.EvidenceFactIDs)
	objectEvidenceJSON, _ := json.Marshal(draft.ObjectActions[0].EvidenceFactIDs)
	return fmt.Sprintf(`{
		"type": %q,
		"category": %q,
		"priority": %q,
		"title": %q,
		"summary": %q,
		"reason": %q,
		"next_step": %q,
		"expected_effect": %q,
		"completion_criteria": %q,
		"evidence_fact_ids": %s,
		"affected_entities": [{"entity_type":"sku","entity_id":"sku_1"}],
		"source_candidate_ids": ["%s"],
		"object_actions": [{
			"entity_type": "sku",
			"entity_id": "sku_1",
			"recommended_action": %q,
			"reason": %q,
			"evidence_fact_ids": %s,
			"metrics": [
				{"label":"Остаток","value":"120","unit":"шт"},
				{"label":"Заказы 7д","value":"0"}
			]
		}],
		"action_plan": [
			{"title":"Шаг 1","instruction":"Открыть список SKU sku_1 и проверить остаток 120 шт.","expected_result":"Определён порядок"},
			{"title":"Шаг 2","instruction":"Проверить карточку sku_1: цену, фото, отзывы и видимость.","expected_result":"Выявлены проблемы"},
			{"title":"Шаг 3","instruction":"Запустить акцию или перераспределить рекламное внимание на кампании sku_1.","expected_result":"Запущены меры"}
		]
		%s
	}`,
		draft.Type, draft.Category, draft.Priority, draft.Title, draft.Summary, draft.Reason, draft.NextStep,
		draft.ExpectedEffect, draft.CompletionCriteria, string(evidenceJSON), c.CandidateID,
		draft.ObjectActions[0].RecommendedAction, draft.ObjectActions[0].Reason, string(objectEvidenceJSON), extra)
}

func TestActionsParser_ActionPlanArrayObjects(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	raw := fmt.Sprintf(`{
		"schema_version": "actions_ai_first_v2",
		"actions": [%s]
	}`, minimalPilotActionJSONFields(c, `,
		"action_plan": [
			{"title":"Проверить остатки","instruction":"Проверьте SKU","expected_result":"Остатки подтверждены"}
		]`))
	out, err := parseActionsAIFirstOutput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Actions) != 1 {
		t.Fatalf("actions=%d", len(out.Actions))
	}
	plan := out.Actions[0].ActionPlan
	if len(plan) != 1 || plan[0].Instruction != "Проверьте SKU" {
		t.Fatalf("plan=%+v", plan)
	}
}

func TestActionsParser_ActionPlanStringNormalized(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	raw := fmt.Sprintf(`{
		"schema_version": "actions_ai_first_v2",
		"actions": [%s]
	}`, minimalPilotActionJSONFields(c, `,
		"action_plan": "Проверить остатки SKU и поставить товар на склад"`))
	out, err := parseActionsAIFirstOutput(raw)
	if err != nil {
		t.Fatal(err)
	}
	plan := out.Actions[0].ActionPlan
	if len(plan) != 1 {
		t.Fatalf("plan=%+v", plan)
	}
	want := "Проверить остатки SKU и поставить товар на склад"
	if plan[0].Title != want || plan[0].Instruction != want {
		t.Fatalf("plan=%+v", plan)
	}
	if out.Actions[0].NextStep != c.RecommendedAction {
		t.Fatalf("next_step=%q want preserved %q", out.Actions[0].NextStep, c.RecommendedAction)
	}
}

func TestActionsParser_ActionPlanArrayStringsNormalized(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	raw := fmt.Sprintf(`{
		"schema_version": "actions_ai_first_v2",
		"actions": [%s]
	}`, minimalPilotActionJSONFields(c, `,
		"action_plan": [
			"Проверить остатки",
			"Сформировать поставку",
			"Проконтролировать поступление"
		]`))
	out, err := parseActionsAIFirstOutput(raw)
	if err != nil {
		t.Fatal(err)
	}
	plan := out.Actions[0].ActionPlan
	if len(plan) != 3 {
		t.Fatalf("plan=%+v", plan)
	}
	if plan[0].Instruction != "Проверить остатки" || plan[2].Instruction != "Проконтролировать поступление" {
		t.Fatalf("plan=%+v", plan)
	}
}

func TestActionsParser_ActionPlanObjectNormalized(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	raw := fmt.Sprintf(`{
		"schema_version": "actions_ai_first_v2",
		"actions": [%s]
	}`, minimalPilotActionJSONFields(c, `,
		"action_plan": {
			"title": "Проверить рекламу",
			"description": "Отключить кампании с высоким ДРР"
		}`))
	out, err := parseActionsAIFirstOutput(raw)
	if err != nil {
		t.Fatal(err)
	}
	plan := out.Actions[0].ActionPlan
	if len(plan) != 1 || plan[0].Title != "Проверить рекламу" {
		t.Fatalf("plan=%+v", plan)
	}
	if plan[0].Instruction != "Отключить кампании с высоким ДРР" {
		t.Fatalf("instruction=%q", plan[0].Instruction)
	}
}

func TestActionsParser_InvalidActionPlanDropsOnlyOneAction(t *testing.T) {
	c1 := testStockCandidate("stock:candidate:01")
	c2 := testStockCandidate("stock:candidate:02")
	raw := fmt.Sprintf(`{
		"schema_version": "actions_ai_first_v2",
		"actions": [
			%s,
			{"action_plan": 12345}
		]
	}`, minimalPilotActionJSONFields(c1, `,
		"action_plan": "Проверить остатки"`))
	out, err := parseActionsAIFirstOutput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Actions) != 1 {
		t.Fatalf("actions=%d want=1", len(out.Actions))
	}
	if out.Actions[0].SourceCandidateIDs[0] != c1.CandidateID {
		t.Fatalf("kept=%v", out.Actions[0].SourceCandidateIDs)
	}
	_ = c2
}

func TestActionsParser_ActionPlanStringRegression_NoInvalidJSONProviderFailure(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	raw := fmt.Sprintf(`{
		"schema_version": "actions_ai_first_v2",
		"actions": [%s]
	}`, minimalPilotActionJSONFields(c, `,
		"action_plan": "Сделайте то-то по остаткам",
		"evidence_fact_ids": "fact_1",
		"source_candidate_ids": "stock:candidate:01"`))
	out, err := parseActionsAIFirstOutput(raw)
	if err != nil {
		t.Fatalf("parse err=%v", err)
	}
	if strings.Contains(errString(err), ActionsErrorInvalidJSON) {
		t.Fatalf("unexpected invalid_json")
	}
	pack := testV2CatalogPack()
	input, _ := BuildActionsAIFirstInputFromCandidates(pack, "h", testGlobalContext(), "gc", []AIFirstActionCandidate{c}, nil, nil, DefaultGenerateActionsOptions())
	pilot := ValidateActionsAIFirstOutputPilot(input, *out)
	if pilot.AcceptedCount != 1 || !pilotHasIssue(pilot, "insufficient_action_plan") {
		t.Fatalf("accepted=%d dropped=%d issues=%v", pilot.AcceptedCount, pilot.DroppedCount, pilot.Issues)
	}
	if len(out.Actions[0].EvidenceFactIDs) != 1 || out.Actions[0].EvidenceFactIDs[0] != "fact_1" {
		t.Fatalf("evidence=%v", out.Actions[0].EvidenceFactIDs)
	}
}

func TestActionsParser_PartialValidActionsCompletedPartial(t *testing.T) {
	c1 := testStockCandidate("stock:candidate:01")
	c2 := testStockCandidate("stock:candidate:02")
	pack := testV2CatalogPack()
	input, _ := BuildActionsAIFirstInputFromCandidates(pack, "h", testGlobalContext(), "gc", []AIFirstActionCandidate{c1, c2}, nil, nil, DefaultGenerateActionsOptions())
	raw := fmt.Sprintf(`{
		"schema_version": "actions_ai_first_v2",
		"actions": [
			%s,
			{
				"type": %q,
				"category": %q,
				"priority": %q,
				"title": "",
				"reason": %q,
				"next_step": %q,
				"completion_criteria": "Критерий выполнен",
				"evidence_fact_ids": ["%s"],
				"affected_entities": [{"entity_type":"sku","entity_id":"sku_1"}],
				"source_candidate_ids": ["%s"]
			}
		]
	}`,
		detailedPilotActionJSONFields(c1, ""),
		c2.Type, c2.Category, ActionPriorityCritical,
		c2.Problem, c2.RecommendedAction, c2.EvidenceFactIDs[0], c2.CandidateID,
	)
	client := &FakeActionAIClient{
		ComposeAIFirstFn: func(ctx context.Context, in ActionsAIFirstInput) (*ActionsAIFirstOutput, *AIRawResponse, error) {
			out, err := parseActionsAIFirstOutput(raw)
			if err != nil {
				return nil, &AIRawResponse{}, err
			}
			return out, &AIRawResponse{}, nil
		},
	}
	svc := &GenerationService{ai: client, repo: newFakeGenerationRepo()}
	res, err := svc.GenerateActionsAIFirstFromInput(context.Background(), input, GenerateActionsOptions{AIEnabled: true, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.FinalStatus != GenerationFinalStatusCompletedPartial {
		t.Fatalf("status=%s", res.FinalStatus)
	}
	if res.AcceptedCount != 1 || res.DroppedCount < 1 {
		t.Fatalf("accepted=%d dropped=%d", res.AcceptedCount, res.DroppedCount)
	}
	if strings.Contains(res.FailureSubreason, ActionsErrorInvalidJSON) {
		t.Fatalf("failure=%s", res.FailureSubreason)
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
