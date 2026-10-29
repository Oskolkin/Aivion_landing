package actions

import (
	"strings"
	"testing"
)

func TestRun70StabilizationPolicy(t *testing.T) {
	input := buildRun69LikeInput(t)
	specs := []struct {
		candidateID    string
		entityID       string
		goal           string
		unknownFact    bool
		missingObjects bool
	}{
		{"repl:crit", "sku_1", "replenish_stock", false, false},
		{"repl:prev", "sku_3", "replenish_prev_scope", false, false},
		{"no_sales", "sku_4", "reduce_excess_stock", true, false},
		{"surplus", "sku_6", "review_surplus_scope", false, true},
	}

	actions := make([]AIFirstDraftAction, 0, len(specs))
	for _, spec := range specs {
		action := buildRun70ValidAction(input, spec.candidateID, spec.entityID, spec.goal)
		if spec.unknownFact {
			target := 15.0
			action.CompletionConditions = []AIFirstCompletionCondition{{
				MetricCode: "drr_pct", Operator: "lte", TargetValue: &target, Scope: "per_campaign",
				FactID: "invented.completion.fact",
			}}
		}
		if spec.missingObjects {
			action.ObjectActions = nil
		}
		actions = append(actions, action)
	}

	raw := append([]AIFirstDraftAction(nil), actions...)
	normOutput, normResult := NormalizeActionsAIFirstV2Output(input, ActionsAIFirstOutput{Actions: actions})
	pilot := ValidateActionsAIFirstOutputPilot(input, normOutput)
	if len(normResult.Issues) > 0 {
		pilot.Issues = append(pilot.Issues, normResult.Issues...)
	}
	persist := mapAIFirstDraftsWithDiagnostics("2026-07-08", input, pilot.Accepted, warningsByActionIndex(pilot.Issues))

	if pilot.DroppedCount != 0 || persist.DroppedCount != 0 {
		t.Fatalf("unexpected drops validation=%d persist=%v issues=%v", pilot.DroppedCount, persist.DropIssueCounts, pilot.Issues)
	}
	if len(persist.Items) != 4 {
		t.Fatalf("final=%d want 4", len(persist.Items))
	}
	warnings := buildWarningIssueCounts(pilot.Issues)
	if warnings["missing_object_actions"] < 1 {
		t.Fatalf("missing_object_actions warnings=%v", warnings)
	}
	if warnings["unknown_completion_fact_id_removed"] < 1 {
		t.Fatalf("unknown_completion_fact_id_removed warnings=%v", warnings)
	}
	if normResult.CompletionConditionsFactIDRemovedCount < 1 {
		t.Fatalf("fact_id_removed=%d", normResult.CompletionConditionsFactIDRemovedCount)
	}

	persistedKeys := map[string]bool{}
	for _, item := range persist.Items {
		for _, id := range item.SourceCandidateIDs {
			persistedKeys[id] = true
		}
	}
	coverage := BuildActionsCandidateCoverageDiagnostics(input, raw, pilot.Accepted, pilot.Dropped, persistedKeys)
	total := coverage["actions_candidate_total_count"].(int)
	persisted := coverage["actions_candidate_referenced_persisted_count"].(int)
	droppedOnly := coverage["actions_candidate_referenced_dropped_only_count"].(int)
	notReferenced := coverage["actions_candidate_not_referenced_by_ai_count"].(int)
	if total != persisted+droppedOnly+notReferenced {
		t.Fatalf("coverage invariant failed total=%d persisted=%d droppedOnly=%d notReferenced=%d", total, persisted, droppedOnly, notReferenced)
	}

	diag := buildAIFirstPipelineDiagnostics(len(actions), normResult.DroppedActionCount, len(normOutput.Actions), pilot, persist, pilot.Issues, coverage)
	if err := diag.invariantError(); err != nil {
		t.Fatal(err)
	}
}

func buildRun70ValidAction(input ActionsAIFirstInput, candidateID, entityID, goal string) AIFirstDraftAction {
	action := detailedNormalizedAction(input, candidateID, entityID)
	for _, c := range input.ActionCandidates {
		if strings.TrimSpace(c.CandidateID) == candidateID {
			action.Type = c.Type
			action.Category = c.Category
			break
		}
	}
	action.OperationalGoal = goal
	action.CompletionCriteria = "Остаток восстановлен"
	target := 12.0
	action.CompletionConditions = []AIFirstCompletionCondition{{
		MetricCode: "stock_available", Operator: "gte", TargetValue: &target, Scope: "per_sku", PeriodDays: 7,
	}}
	entType := "sku"
	if strings.HasPrefix(entityID, "campaign") {
		entType = "campaign"
	}
	action.ObjectActions = []AIFirstObjectAction{{
		EntityType: entType, EntityID: entityID, Name: entityID,
		RecommendedAction: "Проверить и принять меры",
		Reason:            "Объект входит в scope действия",
		Metrics:           []AIFirstObjectMetric{{FactID: "fact_1", Label: "orders", Value: "1"}},
		EvidenceFactIDs:   []string{"fact_1"},
	}}
	return action
}

func TestValidateAIFirstPersistableActionItem_AllowsMissingObjectActionsWithScope(t *testing.T) {
	item := ActionItem{
		Type:               ActionTypeSlowTurnoverReview,
		Category:           ActionCategorySlowTurnover,
		Priority:           ActionPriorityImportant,
		EvidenceFactIDs:    []string{"f1"},
		Evidence:           []ActionEvidence{{Kind: "metric", Label: "orders", Value: "5"}},
		CompletionCriteria: "Решение принято по каждому SKU.",
		AffectedEntities: ActionAffectedEntities{
			SKUs: []ActionSKURef{{SKU: "1", Name: "SKU 1"}, {SKU: "2", Name: "SKU 2"}},
		},
		AffectedObjectsTotal:     2,
		AffectedObjectsDisplayed: 0,
		AffectedObjectsTruncated: true,
		OperationalDetail: &OperationalDetail{
			Status:        "available",
			FormatVersion: OperationalDetailFormatVersion,
			Why:           OperationalWhy{Headline: "h", Text: "t"},
			ActionPlan:    OperationalActionPlan{Headline: "Что сделать", Steps: []OperationalStep{{StepID: "s1", Title: "Шаг 1", Instruction: "Сделать"}}},
		},
		CTAs: []ActionCTA{{Type: "mark_checked", Label: "ok"}},
	}
	if err := ValidateAIFirstPersistableActionItem(item); err != nil {
		t.Fatalf("expected persist with scope only, got %v", err)
	}
}

func TestAIFirstPipelineDiagnosticsInvariant(t *testing.T) {
	pilot := PilotActionsValidationResult{
		ReturnedCount: 8, ValidBeforeLimit: 6, AcceptedCount: 6, DroppedCount: 2,
		DropIssueCounts: map[string]int{"unknown_source_candidate_id": 2},
	}
	persist := aiFirstPersistGuardResult{Items: make([]ActionItem, 6), DroppedCount: 0}
	diag := buildAIFirstPipelineDiagnostics(8, 0, 8, pilot, persist, nil, nil)
	if err := diag.invariantError(); err != nil {
		t.Fatal(err)
	}
}
