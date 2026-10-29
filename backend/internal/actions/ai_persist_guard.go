package actions

import (
	"fmt"
	"strings"
)

// PersistValidationMode selects which persist guard applies to a mapped item.
type PersistValidationMode string

const (
	PersistValidationModeCandidateFirst PersistValidationMode = "candidate_first"
	PersistValidationModeAIFirst        PersistValidationMode = "ai_first"
)

// ValidatePersist dispatches to the mode-specific persist guard.
func ValidatePersist(item ActionItem, mode PersistValidationMode) error {
	if mode == PersistValidationModeAIFirst {
		return ValidateAIFirstPersistableActionItem(item)
	}
	return ValidatePersistableActionItem(item)
}

// ValidateAIFirstPersistableActionItem is the final guard for AI-first mapped
// actions. Unlike the legacy guard it does not require source_candidate_ids,
// entity_group_ref or list/export CTAs, but it does require self-sufficiency:
// evidence facts, affected entities, object actions and completion criteria.
func ValidateAIFirstPersistableActionItem(item ActionItem) error {
	if !containsActionType(ActionsAIFirstActiveActionTypes, item.Type) {
		return fmt.Errorf("action type %q is not allowed for AI-first persist", item.Type)
	}
	if !containsActionCategory(ActionsAIFirstActiveCategories, item.Category) {
		return fmt.Errorf("category %q is not allowed for AI-first persist", item.Category)
	}
	if !isValidPriority(item.Priority) {
		return fmt.Errorf("invalid priority %q", item.Priority)
	}
	if len(item.EvidenceFactIDs) == 0 {
		return fmt.Errorf("missing evidence_fact_ids")
	}
	if len(item.Evidence) == 0 {
		return fmt.Errorf("missing evidence")
	}
	if strings.TrimSpace(item.CompletionCriteria) == "" {
		return fmt.Errorf("missing completion_criteria")
	}
	if item.Type == ActionTypeFunnelWeakTransition && item.AffectedEntities.Funnel == nil {
		return fmt.Errorf("missing funnel transition for funnel action")
	}
	scope := resolvePersistScopeType(item)
	switch scope {
	case AIFirstActionScopeAccount:
		// Account-level actions may have empty affected entities and object_actions.
	case AIFirstActionScopeEntity, AIFirstActionScopeMixed:
		if !hasAffectedEntitiesPreview(item.AffectedEntities) &&
			(item.OperationalDetail == nil || len(item.OperationalDetail.ObjectActions) == 0) {
			return fmt.Errorf("missing affected entities")
		}
	default:
		if !hasAffectedEntitiesPreview(item.AffectedEntities) &&
			(item.OperationalDetail == nil || len(item.OperationalDetail.ObjectActions) == 0) {
			return fmt.Errorf("missing affected entities")
		}
	}
	for _, cta := range item.CTAs {
		key := strings.ToLower(strings.TrimSpace(cta.Type))
		if key == "export" || key == "navigate" || strings.HasPrefix(key, "open_") || strings.HasPrefix(key, "export_") {
			return fmt.Errorf("cta %q is not allowed for AI-first actions", cta.Type)
		}
	}
	return nil
}

// ValidatePersistableActionItem is the final guard before persisting mapped actions.
func ValidatePersistableActionItem(item ActionItem) error {
	if len(item.SourceCandidateIDs) == 0 {
		return fmt.Errorf("missing source_candidate_ids")
	}
	if len(item.EvidenceFactIDs) == 0 {
		return fmt.Errorf("missing evidence_fact_ids")
	}
	if len(item.CTAs) == 0 {
		return fmt.Errorf("missing ctas")
	}
	if len(item.Evidence) == 0 {
		return fmt.Errorf("missing evidence")
	}
	if isObjectBasedActionType(item.Type) {
		if strings.TrimSpace(item.EntityGroupRef) == "" && !hasAffectedEntitiesPreview(item.AffectedEntities) {
			return fmt.Errorf("missing entity group or affected entities for object-based action")
		}
	}
	if item.Type == ActionTypeFunnelWeakTransition {
		if item.AffectedEntities.Funnel == nil && strings.TrimSpace(item.EntityGroupRef) == "" {
			return fmt.Errorf("missing funnel transition evidence")
		}
	}
	return nil
}

func isObjectBasedActionType(t ActionType) bool {
	switch t {
	case ActionTypeStockReplenishment,
		ActionTypeNoSalesWithStock,
		ActionTypeSKUOrdersDrop,
		ActionTypeKeySKURisk,
		ActionTypeAdHighDRR,
		ActionTypeAdSpendNoOrders:
		return true
	default:
		return false
	}
}

func isSKUEntityGroupType(t ActionType) bool {
	switch t {
	case ActionTypeStockReplenishment,
		ActionTypeNoSalesWithStock,
		ActionTypeSKUOrdersDrop,
		ActionTypeKeySKURisk:
		return true
	default:
		return false
	}
}

func isCampaignEntityGroupType(t ActionType) bool {
	switch t {
	case ActionTypeAdHighDRR,
		ActionTypeAdSpendNoOrders:
		return true
	default:
		return false
	}
}

func hasAffectedEntitiesPreview(entities ActionAffectedEntities) bool {
	if len(entities.SKUs) > 0 || len(entities.Campaigns) > 0 {
		return true
	}
	if entities.Funnel != nil {
		return strings.TrimSpace(entities.Funnel.TransitionKey) != "" ||
			strings.TrimSpace(entities.Funnel.FromStage) != "" ||
			strings.TrimSpace(entities.Funnel.ToStage) != ""
	}
	return false
}
