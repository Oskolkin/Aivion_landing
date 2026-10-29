package actions

import (
	"strings"
)

const (
	PilotValidatorMode               = "pilot"
	StrictLegacyValidatorMode        = "strict_legacy"
	PilotMaxFinalActions             = 20
	pilotIssueInvalidActionType      = "invalid_action_type"
	pilotIssueInvalidActionCategory  = "invalid_action_category"
	pilotIssueInvalidActionPriority  = "invalid_action_priority"
	pilotIssueMissingTitle           = "missing_title"
	pilotIssueMissingTitleTemplate   = "missing_title_template"
	pilotIssueMissingOperationalGoal = "missing_operational_goal"
	pilotIssueMissingReason          = "missing_reason"
	pilotIssueMissingNextStepOrPlan  = "missing_next_step_or_action_plan"
	pilotIssueMissingCompletion      = "missing_completion_criteria"
	pilotIssueMissingAffected        = "missing_affected_entities"
	pilotIssueUnknownEntity          = "unknown_entity"
	pilotIssueMissingEvidenceFacts   = "missing_evidence_fact_ids" // legacy; prefer no_valid_evidence_after_candidate_derivation
	pilotIssueUnknownFact            = "unknown_fact"
	pilotIssueMissingSourceIDs       = "missing_source_candidate_ids"
	pilotIssueUnknownSourceID        = "unknown_source_candidate_id"
	pilotIssueMissingScopeType       = "missing_scope_type"
	pilotIssueInvalidScopeType       = "invalid_scope_type"
)

// PilotDroppedAction records one AI action removed by the pilot validator.
type PilotDroppedAction struct {
	OriginalIndex int      `json:"original_index"`
	IssueCodes    []string `json:"issue_codes"`
}

// PilotActionsValidationResult is the per-item pilot validation outcome.
type PilotActionsValidationResult struct {
	Accepted         []AIFirstDraftAction `json:"accepted"`
	Dropped          []PilotDroppedAction `json:"dropped"`
	Issues           []ValidationIssue    `json:"issues"`
	ReturnedCount    int                  `json:"returned_count"`
	ValidBeforeLimit int                  `json:"valid_before_limit_count"`
	AcceptedCount    int                  `json:"accepted_count"`
	DroppedCount     int                  `json:"dropped_count"`
	TruncatedCount   int                  `json:"truncated_count"`
	DropIssueCounts  map[string]int       `json:"drop_issue_counts,omitempty"`
}

// ValidateActionsAIFirstOutputPilot validates each AI action independently for
// the production V2 candidate-based flow.
func ValidateActionsAIFirstOutputPilot(input ActionsAIFirstInput, output ActionsAIFirstOutput) PilotActionsValidationResult {
	result := PilotActionsValidationResult{
		Accepted:        make([]AIFirstDraftAction, 0, len(output.Actions)),
		Dropped:         make([]PilotDroppedAction, 0),
		Issues:          make([]ValidationIssue, 0),
		ReturnedCount:   len(output.Actions),
		DropIssueCounts: map[string]int{},
	}

	allowedTypes := input.AllowedActionTypes
	if len(allowedTypes) == 0 {
		allowedTypes = ActionsAIFirstActiveActionTypes
	}
	allowedCategories := input.AllowedCategories
	if len(allowedCategories) == 0 {
		allowedCategories = ActionsAIFirstActiveCategories
	}
	allowedPriorities := input.AllowedPriorities
	if len(allowedPriorities) == 0 {
		allowedPriorities = []ActionPriority{ActionPriorityCritical, ActionPriorityImportant, ActionPriorityWatch}
	}
	candidateByID := buildCandidateIndex(input.ActionCandidates)

	for i, action := range output.Actions {
		issues := validatePilotAction(input, action, i, allowedTypes, allowedCategories, allowedPriorities, candidateByID)
		var blocking []ValidationIssue
		for _, issue := range issues {
			if issue.Severity == IssueSeverityWarning {
				result.Issues = append(result.Issues, issue)
				continue
			}
			blocking = append(blocking, issue)
		}
		if len(blocking) == 0 {
			result.Accepted = append(result.Accepted, action)
			continue
		}
		codes := make([]string, 0, len(blocking))
		for _, issue := range blocking {
			result.Issues = append(result.Issues, issue)
			codes = append(codes, issue.Code)
			result.DropIssueCounts[issue.Code]++
		}
		result.Dropped = append(result.Dropped, PilotDroppedAction{
			OriginalIndex: i,
			IssueCodes:    codes,
		})
	}

	result.ValidBeforeLimit = len(result.Accepted)
	if len(result.Accepted) > PilotMaxFinalActions {
		result.TruncatedCount = len(result.Accepted) - PilotMaxFinalActions
		result.Accepted = result.Accepted[:PilotMaxFinalActions]
	}
	result.AcceptedCount = len(result.Accepted)
	result.DroppedCount = len(result.Dropped)
	return result
}

func validatePilotAction(
	input ActionsAIFirstInput,
	action AIFirstDraftAction,
	idx int,
	allowedTypes []ActionType,
	allowedCategories []ActionCategory,
	allowedPriorities []ActionPriority,
	candidateByID map[string]AIFirstActionCandidate,
) []ValidationIssue {
	var issues []ValidationIssue
	add := func(code, message, field string) {
		i := idx
		issues = append(issues, ValidationIssue{
			Severity:    IssueSeverityFatal,
			Code:        code,
			Message:     message,
			ActionIndex: &i,
			Field:       field,
		})
	}

	if action.Type == "" || !containsActionType(allowedTypes, action.Type) {
		add(pilotIssueInvalidActionType, "invalid or missing action type", "type")
	}
	if action.Category == "" || !containsActionCategory(allowedCategories, action.Category) {
		add(pilotIssueInvalidActionCategory, "invalid or missing action category", "category")
	}
	if action.Priority == "" || !containsActionPriority(allowedPriorities, action.Priority) {
		add(pilotIssueInvalidActionPriority, "invalid or missing action priority", "priority")
	}
	if strings.TrimSpace(action.Title) == "" {
		if strings.TrimSpace(action.TitleTemplate) != "" {
			add(pilotIssueMissingTitle, "title is required after template rendering", "title")
		} else {
			add(pilotIssueMissingTitle, "title or title_template is required", "title")
		}
	}
	if strings.TrimSpace(action.TitleTemplate) != "" && strings.TrimSpace(action.OperationalGoal) == "" {
		add(pilotIssueMissingOperationalGoal, "operational_goal is required for structured actions", "operational_goal")
	}
	if strings.TrimSpace(action.Reason) == "" {
		add(pilotIssueMissingReason, "reason is required", "reason")
	}
	if !pilotHasNextStepOrActionPlan(action) {
		add(pilotIssueMissingNextStepOrPlan, "next_step or action_plan is required", "next_step")
	}
	if strings.TrimSpace(action.CompletionCriteria) == "" {
		add(pilotIssueMissingCompletion, "completion_criteria is required", "completion_criteria")
	}

	scope := normalizeAIFirstScopeType(string(action.ScopeType))
	if isStructuredAIFirstAction(action) {
		if scope == "" {
			add(pilotIssueMissingScopeType, "scope_type is required (account|entity|mixed)", "scope_type")
		} else if !isValidAIFirstScopeType(scope) {
			add(pilotIssueInvalidScopeType, "invalid scope_type", "scope_type")
		}
	}

	requireEntities := scope == AIFirstActionScopeEntity || scope == AIFirstActionScopeMixed
	if scope == "" && !isStructuredAIFirstAction(action) {
		requireEntities = true
	}
	if requireEntities {
		if len(action.AffectedEntities) == 0 {
			add(pilotIssueMissingAffected, "affected_entities is required", "affected_entities")
		} else {
			for _, ent := range action.AffectedEntities {
				entType := strings.TrimSpace(ent.EntityType)
				entID := strings.TrimSpace(ent.EntityID)
				if entID == "" || !input.entityExists(entType, entID) {
					add(pilotIssueUnknownEntity, "unknown affected entity", "affected_entities")
					break
				}
			}
		}
	} else if len(action.AffectedEntities) > 0 {
		for _, ent := range action.AffectedEntities {
			entType := strings.TrimSpace(ent.EntityType)
			entID := strings.TrimSpace(ent.EntityID)
			if entID != "" && !input.entityExists(entType, entID) {
				add(pilotIssueUnknownEntity, "unknown affected entity", "affected_entities")
				break
			}
		}
	}
	if len(action.EvidenceFactIDs) == 0 {
		add(pilotIssueNoValidEvidenceAfterDerivation,
			"no valid evidence facts after candidate derivation", "evidence_fact_ids")
	} else {
		for _, factID := range action.EvidenceFactIDs {
			if !input.factExists(strings.TrimSpace(factID)) {
				add(pilotIssueUnknownFact, "unknown evidence_fact_id", "evidence_fact_ids")
				break
			}
		}
	}
	sourceIDs := normalizeSourceCandidateIDs(action.SourceCandidateIDs)
	if len(sourceIDs) == 0 {
		add(pilotIssueMissingSourceIDs, "source_candidate_ids is required", "source_candidate_ids")
	} else {
		for _, sourceID := range sourceIDs {
			if _, ok := candidateByID[sourceID]; !ok {
				add(pilotIssueUnknownSourceID, "unknown source_candidate_id", "source_candidate_ids")
				break
			}
		}
	}

	issues = append(issues, validateAIFirstActionStructuralQuality(input, action, idx)...)
	issues = append(issues, validateCompletionConditionsStructure(input, action, idx)...)

	return issues
}

func pilotHasNextStepOrActionPlan(action AIFirstDraftAction) bool {
	if strings.TrimSpace(action.NextStep) != "" {
		return true
	}
	for _, step := range action.ActionPlan {
		if strings.TrimSpace(step.Instruction) != "" {
			return true
		}
	}
	return false
}

func pilotValidationToLegacyResult(pilot PilotActionsValidationResult) AIFirstValidationResult {
	out := AIFirstValidationResult{
		Status:          ValidationStatusValid,
		AcceptedActions: append([]AIFirstDraftAction(nil), pilot.Accepted...),
		Issues:          append([]ValidationIssue(nil), pilot.Issues...),
	}
	if pilot.DroppedCount > 0 {
		out.DroppedActions = make([]AIFirstDroppedAction, 0, len(pilot.Dropped))
		for _, dropped := range pilot.Dropped {
			out.DroppedActions = append(out.DroppedActions, AIFirstDroppedAction{
				Reasons: dropped.IssueCodes,
			})
		}
	}
	if pilot.AcceptedCount == 0 && pilot.ReturnedCount > 0 {
		out.Status = ValidationStatusInvalid
	} else if pilot.DroppedCount > 0 {
		out.Status = ValidationStatusRepairable
	}
	return out
}
