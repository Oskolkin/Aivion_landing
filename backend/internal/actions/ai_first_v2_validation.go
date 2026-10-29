package actions

import (
	"fmt"
	"strings"
)

const actionsAIFirstV2MaxActions = 20

func validateAIFirstV2Output(input ActionsAIFirstInput, output ActionsAIFirstOutput) []ValidationIssue {
	if !input.IsV2CandidateInput() {
		return nil
	}
	if len(output.Actions) > actionsAIFirstV2MaxActions {
		return []ValidationIssue{{
			Severity: IssueSeverityFatal,
			Code:     ActionsErrorTooManyActions,
			Message:  fmt.Sprintf("output has %d actions, max is %d", len(output.Actions), actionsAIFirstV2MaxActions),
		}}
	}

	candidateByID := buildCandidateIndex(input.ActionCandidates)
	usedCandidates := map[string]int{}
	var issues []ValidationIssue

	for i, action := range output.Actions {
		idx := i
		sourceIDs := normalizeSourceCandidateIDs(action.SourceCandidateIDs)
		if hasDuplicateSourceCandidateIDs(action.SourceCandidateIDs) {
			issues = append(issues, ValidationIssue{
				Severity: IssueSeverityFatal, Code: ActionsErrorValidationFailed,
				Message: "duplicate source_candidate_id in action", ActionIndex: &idx, Field: "source_candidate_ids",
			})
			continue
		}
		if len(sourceIDs) == 0 {
			issues = append(issues, ValidationIssue{
				Severity: IssueSeverityFatal, Code: ActionsErrorValidationFailed,
				Message: "source_candidate_ids is required", ActionIndex: &idx, Field: "source_candidate_ids",
			})
			continue
		}
		actionType := ActionType("")
		for _, sourceID := range sourceIDs {
			cand, ok := candidateByID[sourceID]
			if !ok {
				issues = append(issues, ValidationIssue{
					Severity: IssueSeverityFatal, Code: ActionsErrorUnknownSourceCandidateID,
					Message: fmt.Sprintf("unknown source_candidate_id %q", sourceID), ActionIndex: &idx, Field: "source_candidate_ids",
				})
				continue
			}
			if actionType == "" {
				actionType = cand.Type
			} else if cand.Type != actionType {
				issues = append(issues, ValidationIssue{
					Severity: IssueSeverityFatal, Code: ActionsErrorIncompatibleSourceCandidateTypes,
					Message:     fmt.Sprintf("source candidates have incompatible types for one action"),
					ActionIndex: &idx, Field: "source_candidate_ids",
				})
			}
			if action.Type != "" && action.Type != cand.Type {
				issues = append(issues, ValidationIssue{
					Severity: IssueSeverityFatal, Code: ActionsErrorIncompatibleSourceCandidateTypes,
					Message:     fmt.Sprintf("action type %q does not match candidate type %q", action.Type, cand.Type),
					ActionIndex: &idx, Field: "type",
				})
			}
			usedCandidates[sourceID]++
			if usedCandidates[sourceID] > 1 {
				issues = append(issues, ValidationIssue{
					Severity: IssueSeverityFatal, Code: ActionsErrorDuplicateSourceCandidateUsage,
					Message:     fmt.Sprintf("candidate %q used in multiple actions", sourceID),
					ActionIndex: &idx, Field: "source_candidate_ids",
				})
			}
		}

		allowedFacts := allowedFactsForSourceCandidates(input, sourceIDs, candidateByID)
		allowedEntities := allowedEntitiesForSourceCandidates(sourceIDs, candidateByID)
		for _, factID := range action.EvidenceFactIDs {
			factID = strings.TrimSpace(factID)
			if factID == "" {
				continue
			}
			if _, ok := allowedFacts[factID]; !ok {
				issues = append(issues, ValidationIssue{
					Severity: IssueSeverityFatal, Code: ActionsErrorFactNotGroundedInCandidates,
					Message:     fmt.Sprintf("fact %q is not grounded in source candidates", factID),
					ActionIndex: &idx, Field: "evidence_fact_ids",
				})
			}
		}
		for _, ent := range action.AffectedEntities {
			entID := strings.TrimSpace(ent.EntityID)
			if entID == "" {
				continue
			}
			if _, ok := allowedEntities[entID]; !ok {
				issues = append(issues, ValidationIssue{
					Severity: IssueSeverityFatal, Code: ActionsErrorEntityNotGroundedInCandidates,
					Message:     fmt.Sprintf("entity %q is not grounded in source candidates", entID),
					ActionIndex: &idx, Field: "affected_entities",
				})
			}
		}
		for _, obj := range action.ObjectActions {
			entID := strings.TrimSpace(obj.EntityID)
			if entID != "" {
				if _, ok := allowedEntities[entID]; !ok {
					issues = append(issues, ValidationIssue{
						Severity: IssueSeverityFatal, Code: ActionsErrorEntityNotGroundedInCandidates,
						Message:     fmt.Sprintf("object entity %q is not grounded in source candidates", entID),
						ActionIndex: &idx, Field: "object_actions",
					})
				}
			}
			for _, factID := range obj.EvidenceFactIDs {
				factID = strings.TrimSpace(factID)
				if factID == "" {
					continue
				}
				if _, ok := allowedFacts[factID]; !ok {
					issues = append(issues, ValidationIssue{
						Severity: IssueSeverityFatal, Code: ActionsErrorFactNotGroundedInCandidates,
						Message:     fmt.Sprintf("object fact %q is not grounded in source candidates", factID),
						ActionIndex: &idx, Field: "object_actions",
					})
				}
			}
		}
	}
	return issues
}

func normalizeSourceCandidateIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(ids))
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func hasDuplicateSourceCandidateIDs(ids []string) bool {
	seen := map[string]struct{}{}
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			return true
		}
		seen[id] = struct{}{}
	}
	return false
}

func applyV2ValidationIssues(result *AIFirstValidationResult, v2Issues []ValidationIssue) {
	if len(v2Issues) == 0 {
		return
	}
	result.Issues = append(result.Issues, v2Issues...)
	for _, issue := range v2Issues {
		if issue.Severity != IssueSeverityFatal && issue.Severity != IssueSeverityRepairable {
			continue
		}
		if issue.ActionIndex == nil {
			result.Status = ValidationStatusInvalid
			continue
		}
		idx := *issue.ActionIndex
		if idx < 0 || idx >= len(result.AcceptedActions) {
			result.Status = ValidationStatusInvalid
			continue
		}
		dropped := result.AcceptedActions[idx]
		result.AcceptedActions = append(result.AcceptedActions[:idx], result.AcceptedActions[idx+1:]...)
		result.DroppedActions = append(result.DroppedActions, AIFirstDroppedAction{
			Action:  dropped,
			Reasons: []string{issue.Code},
		})
		result.Status = ValidationStatusInvalid
	}
}
