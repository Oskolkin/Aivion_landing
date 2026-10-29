package actions

import (
	"fmt"
	"strings"
)

// validateAIFirstActionStructuralQuality checks AI output shape without making
// business decisions. Non-grounding issues are warnings only.
func validateAIFirstActionStructuralQuality(
	input ActionsAIFirstInput,
	action AIFirstDraftAction,
	idx int,
) []ValidationIssue {
	var issues []ValidationIssue
	addWarning := func(code, message, field string) {
		i := idx
		issues = append(issues, ValidationIssue{
			Severity:    IssueSeverityWarning,
			Code:        code,
			Message:     message,
			ActionIndex: &i,
			Field:       field,
		})
	}

	if aiFirstActionRequiresObjectActionsDetail(action) && len(action.ObjectActions) == 0 {
		addWarning("missing_object_actions", "object_actions is required when affected_entities contain concrete objects", "object_actions")
	}

	if action.ObjectActionsCoverage != nil &&
		objectActionsCoverageMetadataMismatch(action.ObjectActionsCoverage, totalEntitiesForCoverage(action), len(action.ObjectActions)) {
		addWarning("object_actions_coverage_metadata_mismatch", "object_actions_coverage.mode does not match actual object_actions counts", "object_actions_coverage")
	}

	if len(action.ActionPlan) < aiFirstMinActionPlanSteps {
		addWarning(
			"insufficient_action_plan",
			fmt.Sprintf("action_plan must contain at least %d concrete steps", aiFirstMinActionPlanSteps),
			"action_plan",
		)
	} else if len(action.ActionPlan) > aiFirstMaxActionPlanSteps {
		addWarning(
			"excessive_action_plan",
			fmt.Sprintf("action_plan must contain at most %d steps", aiFirstMaxActionPlanSteps),
			"action_plan",
		)
	}

	for si, step := range action.ActionPlan {
		field := fmt.Sprintf("action_plan[%d]", si)
		instruction := strings.TrimSpace(step.Instruction)
		if instruction == "" {
			addWarning("missing_step_instruction", "action plan step requires instruction", field)
			continue
		}
		if isGenericActionStep(instruction) {
			addWarning("generic_action_step", "action plan step is too generic", field)
		}
	}

	if aiFirstActionRequiresObjectActionsDetail(action) {
		reason := strings.TrimSpace(action.Reason)
		if len([]rune(reason)) < aiFirstMinObjectBasedReasonLen {
			addWarning(
				"reason_too_short",
				fmt.Sprintf("reason must be at least %d characters for object-based actions", aiFirstMinObjectBasedReasonLen),
				"reason",
			)
		}
	}

	totalEntities := totalEntitiesForCoverage(action)
	scope := normalizeAIFirstScopeType(string(action.ScopeType))
	if scope != AIFirstActionScopeAccount &&
		totalEntities > 0 && totalEntities <= 20 && len(action.ObjectActions) > 0 && len(action.ObjectActions) < totalEntities {
		addWarning(
			"partial_object_actions",
			fmt.Sprintf("expected object_actions for all %d entities when scope <= 20, got %d", totalEntities, len(action.ObjectActions)),
			"object_actions",
		)
	}

	affectedEntityIDs := collectAffectedEntityIDs(action.AffectedEntities)
	for oi, obj := range action.ObjectActions {
		field := fmt.Sprintf("object_actions[%d]", oi)
		entityID := strings.TrimSpace(obj.EntityID)
		entityType := strings.TrimSpace(obj.EntityType)
		if entityType == "" || entityID == "" {
			addWarning("invalid_object_action", "object action requires entity_type and entity_id", field)
			continue
		}
		if !input.entityExists(entityType, entityID) && !affectedEntityIDs[entityType+"|"+entityID] {
			addWarning("unknown_object_entity", fmt.Sprintf("unknown object entity %s:%s", entityType, entityID), field)
		}
		if strings.TrimSpace(obj.RecommendedAction) == "" {
			addWarning("missing_object_recommended_action", "object action requires recommended_action", field)
		}
		if strings.TrimSpace(obj.Reason) == "" {
			addWarning("missing_object_reason", "object action requires reason", field)
		}
		metricCount := countNonEmptyObjectMetrics(obj.Metrics)
		if metricCount < aiFirstMinObjectActionMetrics {
			addWarning(
				"insufficient_object_metrics",
				fmt.Sprintf("object action requires at least %d metrics", aiFirstMinObjectActionMetrics),
				field,
			)
		}
		for _, factID := range obj.EvidenceFactIDs {
			if !input.factExists(strings.TrimSpace(factID)) {
				addWarning("unknown_object_fact_id", fmt.Sprintf("unknown object evidence_fact_id %q", factID), field)
			}
		}
	}

	return issues
}

const (
	aiFirstMinActionPlanSteps      = 3
	aiFirstMaxActionPlanSteps      = 7
	aiFirstMinObjectBasedReasonLen = 250
	aiFirstMinObjectActionMetrics  = 2
)

var aiFirstPureGenericStepPhrases = []string{
	"провести анализ",
	"рассмотреть меры",
	"принять меры",
	"оптимизировать ситуацию",
	"оптимизировать показатели",
	"улучшить показатели",
	"изучить ситуацию",
	"проанализировать причины",
	"принять решение",
	"оптимизировать",
}

var aiFirstStepSpecificityMarkers = []string{
	"sku", "товар", "кампани", "остат", "цена", "цен", "фото", "карточк", "отзыв",
	"ставк", "дрр", "бюджет", "склад", "постав", "заказ", "выруч", "конвер",
	"категор", "видимост", "акци", "скидк", "promo", "промо", "реклам", "баннер",
	"coverage", "покрыт", "warehouse", "складск",
}

func aiFirstActionRequiresObjectActionsDetail(action AIFirstDraftAction) bool {
	scope := normalizeAIFirstScopeType(string(action.ScopeType))
	if scope == AIFirstActionScopeAccount {
		return false
	}
	if scope == AIFirstActionScopeEntity || scope == AIFirstActionScopeMixed {
		return draftActionHasAuthoritativeEntities(action)
	}
	if hasConcreteOperationalEntities(action.AffectedEntities) {
		return true
	}
	return aiFirstRequiresObjectActions(action.Type) && action.Type != ActionTypeFunnelWeakTransition
}

func totalEntitiesForCoverage(action AIFirstDraftAction) int {
	total := action.AffectedObjectsTotal
	if total == 0 {
		total = len(action.AffectedEntities)
	}
	return total
}

func hasConcreteOperationalEntities(entities []AIFirstAffectedEntityRef) bool {
	for _, ent := range entities {
		if isConcreteOperationalEntityType(ent.EntityType) {
			return true
		}
	}
	return false
}

func isConcreteOperationalEntityType(entityType string) bool {
	switch strings.ToLower(strings.TrimSpace(entityType)) {
	case "sku", "campaign", "category", "warehouse", "sku_group", "product", "card":
		return true
	default:
		return false
	}
}

func collectAffectedEntityIDs(entities []AIFirstAffectedEntityRef) map[string]bool {
	out := make(map[string]bool, len(entities))
	for _, ent := range entities {
		t := strings.TrimSpace(ent.EntityType)
		id := strings.TrimSpace(ent.EntityID)
		if t == "" || id == "" {
			continue
		}
		out[t+"|"+id] = true
	}
	return out
}

func countNonEmptyObjectMetrics(metrics []AIFirstObjectMetric) int {
	n := 0
	for _, m := range metrics {
		if strings.TrimSpace(m.FactID) != "" {
			n++
			continue
		}
		if strings.TrimSpace(m.Label) != "" && strings.TrimSpace(m.Value) != "" {
			n++
		}
	}
	return n
}

func isGenericActionStep(instruction string) bool {
	lower := strings.ToLower(strings.TrimSpace(instruction))
	if lower == "" {
		return true
	}
	hasPureGeneric := false
	for _, phrase := range aiFirstPureGenericStepPhrases {
		if strings.Contains(lower, phrase) {
			hasPureGeneric = true
			break
		}
	}
	if !hasPureGeneric {
		return isGenericAIFirstText(instruction)
	}
	for _, marker := range aiFirstStepSpecificityMarkers {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	if strings.ContainsAny(lower, "0123456789") {
		return false
	}
	return true
}
