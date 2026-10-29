package actions

import (
	"fmt"
	"strings"
)

// NormalizeComposeActions performs Phase A normalization on compose output.
func NormalizeComposeActions(
	input ActionsAIFirstInput,
	compose ActionsAIFirstComposeOutput,
) ([]AIFirstBaseAction, AIFirstNormalizationResult, []AIPipelineNormalizationEvent) {
	result := AIFirstNormalizationResult{
		Actions: make([]AIFirstDraftAction, 0, len(compose.Actions)),
	}
	var events []AIPipelineNormalizationEvent
	if !input.IsV2CandidateInput() {
		return nil, result, events
	}

	candidateByID := buildCandidateIndex(input.ActionCandidates)
	result.ReturnedActionCount = len(compose.Actions)

	baseActions := make([]AIFirstBaseAction, 0, len(compose.Actions))
	for i, action := range compose.Actions {
		base, issues, evs := normalizeOneComposeAction(input, action, i, candidateByID)
		result.Issues = append(result.Issues, issues...)
		events = append(events, evs...)
		if base.Dropped {
			result.DroppedActionCount++
			continue
		}
		baseActions = append(baseActions, base)
	}

	baseActions, dedupIssues, dedupEvents := deduplicateBaseActionsByOperationalGoal(baseActions)
	result.Issues = append(result.Issues, dedupIssues...)
	events = append(events, dedupEvents...)

	filtered := make([]AIFirstBaseAction, 0, len(baseActions))
	for _, base := range baseActions {
		if base.Dropped || len(base.AuthoritativeEntities) == 0 && requiresAuthoritativeEntities(base.ScopeType) {
			result.DroppedActionCount++
			continue
		}
		filtered = append(filtered, base)
	}

	for _, base := range filtered {
		draft := baseActionToDraft(base)
		result.Actions = append(result.Actions, draft)
	}

	return filtered, result, events
}

func normalizeOneComposeAction(
	input ActionsAIFirstInput,
	action AIFirstComposeAction,
	idx int,
	candidateByID map[string]AIFirstActionCandidate,
) (AIFirstBaseAction, []ValidationIssue, []AIPipelineNormalizationEvent) {
	var issues []ValidationIssue
	var events []AIPipelineNormalizationEvent
	addWarning := func(code, message, field string) {
		i := idx
		issues = append(issues, ValidationIssue{
			Severity: IssueSeverityWarning, Code: code, Message: message,
			ActionIndex: &i, Field: field,
		})
	}

	base := AIFirstBaseAction{
		AIFirstComposeAction: action,
		AIIndex:              idx,
	}

	validCandidates, removedCandidates := validateSourceCandidateIDs(action.SourceCandidateIDs, candidateByID, idx, &issues)
	base.ValidSourceCandidates = validCandidates
	base.RemovedCandidateIDs = removedCandidates
	if len(validCandidates) == 0 {
		base.Dropped = true
		base.DropReason = "no_valid_source_candidates"
		appendNormEvent(&events, AIPipelineNormalizationEvent{
			Stage: "compose_candidate_validation", ActionIndex: idx, ActionKey: action.ActionKey,
			EventCode: "action_dropped", Severity: IssueSeverityWarning,
			Details: map[string]any{"drop_reason": base.DropReason},
		})
		return base, issues, events
	}
	action.SourceCandidateIDs = validCandidates

	evidence := deriveAndApplyCanonicalActionEvidence(
		input, action.EvidenceFactIDs, action.SourceCandidateIDs, validCandidates, removedCandidates,
		idx, action.ActionKey, candidateByID,
	)
	issues = append(issues, evidence.Issues...)
	events = append(events, evidence.Events...)
	base.EvidenceDerivation = evidence.Derivation
	if evidence.Dropped {
		base.Dropped = true
		base.DropReason = evidence.DropReason
		appendNormEvent(&events, AIPipelineNormalizationEvent{
			Stage: "compose_evidence_derivation", ActionIndex: idx, ActionKey: action.ActionKey,
			EventCode: "action_dropped", Severity: IssueSeverityWarning,
			Details: map[string]any{"drop_reason": base.DropReason},
		})
		return base, issues, events
	}
	action.EvidenceFactIDs = append([]string(nil), evidence.Canonical...)
	base.EvidenceFactIDs = append([]string(nil), evidence.Canonical...)

	authoritative := unionEntitiesFromSourceCandidates(validCandidates, candidateByID)
	beforeScope := len(authoritative)
	appendNormEvent(&events, AIPipelineNormalizationEvent{
		Stage: "authoritative_scope_expanded", ActionIndex: idx, ActionKey: action.ActionKey,
		EventCode: "entity_added_from_candidate_scope", Severity: IssueSeverityWarning,
		BeforeCount: intPtr(0), AfterCount: intPtr(beforeScope),
	})

	authoritative, appliedEx, rejectedEx, exEvents := applyAIEntityExclusions(
		authoritative, action.ExcludedEntityIDs, validCandidates, candidateByID, idx,
	)
	events = append(events, exEvents...)
	base.AppliedExclusions = appliedEx
	base.RejectedExclusions = rejectedEx

	authoritative, internalDupIssues, internalEvents := deduplicateEntitiesWithinAction(authoritative, idx, action.ActionKey)
	issues = append(issues, internalDupIssues...)
	events = append(events, internalEvents...)

	base.AuthoritativeEntities = authoritative

	if requiresAuthoritativeEntities(action.ScopeType) && len(authoritative) == 0 {
		base.Dropped = true
		base.DropReason = "empty_authoritative_scope"
		addWarning("empty_authoritative_scope", "action dropped: authoritative scope empty after exclusions", "authoritative_entities")
		appendNormEvent(&events, AIPipelineNormalizationEvent{
			Stage: "compose_scope_validation", ActionIndex: idx, ActionKey: action.ActionKey,
			EventCode: "action_dropped", Severity: IssueSeverityWarning,
			Details: map[string]any{"drop_reason": base.DropReason},
		})
		return base, issues, events
	}

	if wantCategory, ok := ActionCategoryForType(action.Type); ok && action.Category != wantCategory {
		addWarning("category_taxonomy_normalized",
			fmt.Sprintf("category normalized from %q to %q", action.Category, wantCategory), "category")
		action.Category = wantCategory
		base.Category = wantCategory
	}

	normalizedConditions, condIssues, _ := normalizeActionCompletionConditionsWithStats(input, action.CompletionConditions, idx)
	issues = append(issues, condIssues...)
	base.CompletionConditions = normalizedConditions

	return base, issues, events
}

func validateSourceCandidateIDs(
	sourceIDs []string,
	candidateByID map[string]AIFirstActionCandidate,
	idx int,
	issues *[]ValidationIssue,
) (valid []string, removed []string) {
	for _, id := range normalizeSourceCandidateIDs(sourceIDs) {
		if _, ok := candidateByID[id]; ok {
			valid = append(valid, id)
		} else {
			removed = append(removed, id)
			i := idx
			*issues = append(*issues, ValidationIssue{
				Severity: IssueSeverityWarning, Code: "unknown_candidate_removed",
				Message:     fmt.Sprintf("unknown candidate_id %q removed", id),
				ActionIndex: &i, Field: "source_candidate_ids",
			})
		}
	}
	return valid, removed
}

func applyAIEntityExclusions(
	entities []AIFirstAffectedEntityRef,
	excludedIDs []string,
	validCandidates []string,
	candidateByID map[string]AIFirstActionCandidate,
	idx int,
) ([]AIFirstAffectedEntityRef, []string, []string, []AIPipelineNormalizationEvent) {
	if len(excludedIDs) == 0 {
		return entities, nil, nil, nil
	}
	allowed := unionEntitiesFromSourceCandidates(validCandidates, candidateByID)
	allowedSet := map[string]bool{}
	for _, ref := range allowed {
		allowedSet[entityRefKey(ref)] = true
	}

	excludedSet := map[string]bool{}
	var applied, rejected []string
	var events []AIPipelineNormalizationEvent
	for _, id := range excludedIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		found := false
		for _, ref := range allowed {
			if ref.EntityID == id {
				found = true
				break
			}
		}
		if !found {
			rejected = append(rejected, id)
			continue
		}
		excludedSet[id] = true
		applied = append(applied, id)
	}

	out := make([]AIFirstAffectedEntityRef, 0, len(entities))
	for _, ref := range entities {
		if excludedSet[ref.EntityID] {
			appendNormEvent(&events, AIPipelineNormalizationEvent{
				Stage: "ai_exclusions_applied", ActionIndex: idx,
				EventCode: "entity_removed_by_ai_exclusion", Severity: IssueSeverityWarning,
				EntityIDs: []string{ref.EntityID},
			})
			continue
		}
		out = append(out, ref)
	}
	for _, id := range rejected {
		appendNormEvent(&events, AIPipelineNormalizationEvent{
			Stage: "ai_exclusions_applied", ActionIndex: idx,
			EventCode: "invalid_ai_exclusion_removed", Severity: IssueSeverityWarning,
			EntityIDs: []string{id},
		})
	}
	return out, applied, rejected, events
}

func deduplicateEntitiesWithinAction(
	entities []AIFirstAffectedEntityRef,
	idx int,
	actionKey string,
) ([]AIFirstAffectedEntityRef, []ValidationIssue, []AIPipelineNormalizationEvent) {
	seen := map[string]bool{}
	out := make([]AIFirstAffectedEntityRef, 0, len(entities))
	var issues []ValidationIssue
	var events []AIPipelineNormalizationEvent
	before := len(entities)
	for _, ref := range entities {
		key := entityRefKey(ref)
		if seen[key] {
			i := idx
			issues = append(issues, ValidationIssue{
				Severity: IssueSeverityWarning, Code: "duplicate_entity_removed_within_action",
				Message:     fmt.Sprintf("duplicate entity %s removed within action", ref.EntityID),
				ActionIndex: &i, Field: "authoritative_entities",
			})
			appendNormEvent(&events, AIPipelineNormalizationEvent{
				Stage: "internal_entity_dedup", ActionIndex: idx, ActionKey: actionKey,
				EventCode: "duplicate_entity_removed_within_action", Severity: IssueSeverityWarning,
				EntityIDs: []string{ref.EntityID},
			})
			continue
		}
		seen[key] = true
		out = append(out, ref)
	}
	if len(out) != before {
		appendNormEvent(&events, AIPipelineNormalizationEvent{
			Stage: "internal_entity_dedup", ActionIndex: idx, ActionKey: actionKey,
			EventCode: "duplicate_entity_removed_within_action", Severity: IssueSeverityWarning,
			BeforeCount: intPtr(before), AfterCount: intPtr(len(out)),
		})
	}
	return out, issues, events
}

func deduplicateBaseActionsByOperationalGoal(
	actions []AIFirstBaseAction,
) ([]AIFirstBaseAction, []ValidationIssue, []AIPipelineNormalizationEvent) {
	type entityGoalKey struct {
		entityID string
		goal     string
	}
	seen := map[entityGoalKey]int{}
	var issues []ValidationIssue
	var events []AIPipelineNormalizationEvent

	out := make([]AIFirstBaseAction, len(actions))
	copy(out, actions)

	for actionIdx := range out {
		goal := strings.TrimSpace(out[actionIdx].OperationalGoal)
		if goal == "" {
			continue
		}
		if isAccountScope(out[actionIdx].ScopeType) {
			continue
		}
		kept := make([]AIFirstAffectedEntityRef, 0, len(out[actionIdx].AuthoritativeEntities))
		for _, ent := range out[actionIdx].AuthoritativeEntities {
			entityID := strings.TrimSpace(ent.EntityID)
			if entityID == "" {
				continue
			}
			key := entityGoalKey{entityID: entityID, goal: goal}
			if firstIdx, ok := seen[key]; ok && firstIdx != actionIdx {
				i := actionIdx
				issues = append(issues, ValidationIssue{
					Severity: IssueSeverityWarning, Code: "duplicate_entity_operational_goal_removed",
					Message:     fmt.Sprintf("entity %q removed from action %d; kept in action %d", entityID, actionIdx, firstIdx),
					ActionIndex: &i, Field: "authoritative_entities",
				})
				appendNormEvent(&events, AIPipelineNormalizationEvent{
					Stage: "cross_action_dedup", ActionIndex: actionIdx, ActionKey: out[actionIdx].ActionKey,
					EventCode: "entity_removed_by_cross_action_dedup", Severity: IssueSeverityWarning,
					EntityIDs: []string{entityID},
					Details:   map[string]any{"kept_in_action_index": firstIdx},
				})
				continue
			}
			if _, ok := seen[key]; !ok {
				seen[key] = actionIdx
			}
			kept = append(kept, ent)
		}
		if len(kept) != len(out[actionIdx].AuthoritativeEntities) {
			out[actionIdx].AuthoritativeEntities = kept
		}
		if requiresAuthoritativeEntities(out[actionIdx].ScopeType) && len(kept) == 0 {
			out[actionIdx].Dropped = true
			out[actionIdx].DropReason = "duplicate_scope_after_ai_order_normalization"
			i := actionIdx
			earlierIdx := actionIdx
			sameComparison := true
			for key, firstIdx := range seen {
				if key.goal == goal {
					_ = key
					if firstIdx != actionIdx {
						earlierIdx = firstIdx
						sameComparison = false
						break
					}
				}
			}
			issues = append(issues, ValidationIssue{
				Severity: IssueSeverityWarning, Code: "duplicate_scope_after_ai_order_normalization",
				Message:     "action removed because all entities were deduplicated into an earlier action with the same operational_goal",
				ActionIndex: &i,
			})
			evDetails := map[string]any{
				"current_action_index":    actionIdx,
				"earlier_action_index":    earlierIdx,
				"same_action_comparison":  sameComparison,
				"current_action_key":      out[actionIdx].ActionKey,
				"earlier_action_key":      out[earlierIdx].ActionKey,
				"operational_goal":        goal,
				"current_entities_before": entityIDsFromActionRefs(out[actionIdx].AuthoritativeEntities),
				"current_entities_after":  []string{},
				"result":                  "action_dropped",
			}
			if sameComparison {
				appendNormEvent(&events, AIPipelineNormalizationEvent{
					Stage: "cross_action_dedup", ActionIndex: actionIdx, ActionKey: out[actionIdx].ActionKey,
					EventCode: "dedup_self_comparison_detected", Severity: IssueSeverityFatal,
					Details: evDetails,
				})
			}
			appendNormEvent(&events, AIPipelineNormalizationEvent{
				Stage: "cross_action_dedup", ActionIndex: actionIdx, ActionKey: out[actionIdx].ActionKey,
				EventCode: "cross_action_scope_dedup_comparison", Severity: IssueSeverityWarning,
				Details: evDetails,
			})
		}
	}
	return out, issues, events
}

func requiresAuthoritativeEntities(scope AIFirstActionScopeType) bool {
	return scope == AIFirstActionScopeEntity || scope == AIFirstActionScopeMixed
}

func isAccountScope(scope AIFirstActionScopeType) bool {
	return scope == AIFirstActionScopeAccount
}

func baseActionToDraft(base AIFirstBaseAction) AIFirstDraftAction {
	steps := base.Steps
	if len(steps) == 0 && len(base.ActionPlan) > 0 {
		for _, s := range base.ActionPlan {
			if instr := strings.TrimSpace(s.Instruction); instr != "" {
				steps = append(steps, instr)
			}
		}
	}
	nextStep := base.NextStep
	if nextStep == "" && len(steps) > 0 {
		nextStep = steps[0]
	}
	return AIFirstDraftAction{
		Type:                     base.Type,
		Category:                 base.Category,
		Priority:                 base.Priority,
		OperationalGoal:          base.OperationalGoal,
		ScopeType:                base.ScopeType,
		Title:                    base.Title,
		TitleTemplate:            base.TitleTemplate,
		Summary:                  base.Summary,
		SummaryTemplate:          base.SummaryTemplate,
		Reason:                   base.Reason,
		NextStep:                 nextStep,
		ExpectedEffect:           base.ExpectedEffect,
		ComputedValues:           append([]AIFirstComputedValue(nil), base.ComputedValues...),
		EvidenceFactIDs:          append([]string(nil), base.EvidenceFactIDs...),
		CTAKeys:                  append([]string(nil), base.CTAKeys...),
		AffectedEntities:         append([]AIFirstAffectedEntityRef(nil), base.AuthoritativeEntities...),
		ActionPlan:               append([]AIFirstActionStep(nil), base.ActionPlan...),
		CompletionCriteria:       base.CompletionCriteria,
		CompletionConditions:     append([]AIFirstCompletionCondition(nil), base.CompletionConditions...),
		VisibleLimitations:       append([]string(nil), base.VisibleLimitations...),
		Limitations:              append([]string(nil), base.Limitations...),
		UrgencyMarker:            base.UrgencyMarker,
		SourceCandidateIDs:       firstNonEmptyStringSlice(base.ValidSourceCandidates, base.SourceCandidateIDs),
		AffectedObjectsTotal:     len(base.AuthoritativeEntities),
		AffectedObjectsDisplayed: 0,
		AffectedObjectsTruncated: len(base.AuthoritativeEntities) > 0,
	}
}

func finalizeDraftAfterObjectDetail(draft AIFirstDraftAction, merge ObjectDetailMergeResult) AIFirstDraftAction {
	draft.ObjectActions = merge.ObjectActions
	draft.AffectedObjectsTotal = merge.AuthoritativeTotal
	draft.AffectedObjectsDisplayed = merge.DisplayedCount
	draft.AffectedObjectsTruncated = merge.Truncated
	if merge.DetailCoveragePartial {
		if draft.VisibleLimitations == nil {
			draft.VisibleLimitations = []string{}
		}
	}
	return draft
}

func firstNonEmptyStringSlice(primary, fallback []string) []string {
	if len(primary) > 0 {
		return append([]string(nil), primary...)
	}
	return append([]string(nil), fallback...)
}
